package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"googleplex/internal/auth"
	"googleplex/internal/calendar"
	"googleplex/internal/db"
	"googleplex/internal/docs"
	"googleplex/internal/drive"
	"googleplex/internal/gmail"
	"googleplex/internal/google"
	"googleplex/internal/player"
	"googleplex/internal/sheets"
	"googleplex/internal/tasks"
	"googleplex/internal/youtube"
	"googleplex/internal/ytdlp"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds the few pieces of shared state. Its exported methods are what the
// React frontend can call; each one stays thin and hands off to a package.
type App struct {
	ctx         context.Context
	db          *sql.DB
	devClient   *auth.Client // from .env, dev builds only
	playerBase  string
	streamToken string // guards the /stream pages on the loopback server
	startErr    error

	mu sync.Mutex
	ts oauth2.TokenSource

	ytMu    sync.Mutex
	ytCache map[string]ytCached // YouTube reads, kept briefly to save quota

	mailMu    sync.Mutex
	mailCache map[string]mailPage // first pages of recent Gmail searches

	pipMu     sync.Mutex
	pipCmd    *exec.Cmd // the running pop-out window, if any
	pipToken  string    // per pop-out; its calls must carry it
	pipTime   float64   // last playback time it reported
	pipDocked bool      // it handed the video back, so its exit is not a close

	xferMu    sync.Mutex
	transfers map[string]context.CancelFunc // Drive uploads and downloads, while running

	dlMu      sync.Mutex
	downloads map[string]context.CancelFunc // video id -> cancel, while running
}

func NewApp() *App {
	return &App{downloads: map[string]context.CancelFunc{}, transfers: map[string]context.CancelFunc{}, mailCache: map[string]mailPage{}, ytCache: map[string]ytCached{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	path, err := db.Path()
	if err == nil {
		a.db, err = db.Open(path)
	}
	if err != nil {
		a.startErr = err
		return
	}
	if runtime.Environment(ctx).BuildType == "dev" {
		if c, ok := auth.ClientFromDotEnv(".env"); ok {
			a.devClient = &c
		}
	}
	tok := make([]byte, 24)
	rand.Read(tok)
	a.streamToken = hex.EncodeToString(tok)
	a.playerBase, _ = player.Start(map[string]http.HandlerFunc{
		"/pip/state":        a.pipHandler(false),
		"/pip/dock":         a.pipHandler(true),
		"/pip/ended":        a.pipEnded,
		"/pip/theme":        a.pipTheme,
		"/stream":           a.streamPage,
		"/stream/proxy":     a.streamProxy,
		"/stream/index":     a.streamIndex,
		"/drive/preview":    a.drivePreview,
		"/gmail/attachment": a.gmailAttachment,
	})
}

func (a *App) shutdown(ctx context.Context) {
	a.ClosePopOut()
	if a.db != nil {
		a.db.Close()
	}
}

// ---- setup state ----

type Account struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type SetupState struct {
	StartError   string   `json:"startError"`
	DevMode      bool     `json:"devMode"`
	HasClient    bool     `json:"hasClient"`
	ClientSource string   `json:"clientSource"` // "env" | "json" | ""
	ProjectID    string   `json:"projectId"`
	SignedIn     bool     `json:"signedIn"`
	Account      *Account `json:"account"`
}

func (a *App) GetSetupState() SetupState {
	s := SetupState{DevMode: a.devClient != nil}
	if a.startErr != nil {
		s.StartError = a.startErr.Error()
		return s
	}
	c, source, _ := a.client()
	s.HasClient = c.ID != ""
	s.ClientSource = source
	s.ProjectID = c.ProjectID
	if s.ProjectID == "" {
		s.ProjectID = db.GetSetting(a.db, "project_id")
	}
	if acc := a.account(); acc != nil {
		if rt, _ := auth.LoadSecret("refresh_token"); rt != "" {
			s.SignedIn = true
			s.Account = acc
		}
	}
	return s
}

// SetProjectID lets the setup links point at the user's project before the
// client JSON exists.
func (a *App) SetProjectID(id string) error {
	return db.SetSetting(a.db, "project_id", strings.TrimSpace(id))
}

// SaveClientJSON stores the Desktop OAuth client the user downloaded.
func (a *App) SaveClientJSON(text string) (SetupState, error) {
	c, err := auth.ParseClientJSON(text)
	if err != nil {
		return a.GetSetupState(), err
	}
	if err := auth.SaveSecret("client_secret", c.Secret); err != nil {
		return a.GetSetupState(), errors.New("could not save to the system keychain: " + err.Error())
	}
	db.SetSetting(a.db, "client_id", c.ID)
	if c.ProjectID != "" {
		db.SetSetting(a.db, "project_id", c.ProjectID)
	}
	return a.GetSetupState(), nil
}

// client prefers a saved client JSON; dev builds fall back to .env.
func (a *App) client() (auth.Client, string, error) {
	if id := db.GetSetting(a.db, "client_id"); id != "" {
		secret, err := auth.LoadSecret("client_secret")
		return auth.Client{ID: id, Secret: secret, ProjectID: db.GetSetting(a.db, "project_id")}, "json", err
	}
	if a.devClient != nil {
		return *a.devClient, "env", nil
	}
	return auth.Client{}, "", nil
}

// ---- sign-in ----

func (a *App) SignIn() (SetupState, error) {
	return a.authorize(auth.BaseScopes)
}

// ConnectService asks for one feature's scopes on top of what is granted.
func (a *App) ConnectService(service string) (SetupState, error) {
	extra, ok := auth.ServiceScopes[service]
	if !ok {
		return a.GetSetupState(), errors.New("unknown service " + service)
	}
	return a.authorize(append(append([]string{}, auth.BaseScopes...), extra...))
}

func (a *App) authorize(scopes []string) (SetupState, error) {
	c, _, err := a.client()
	if err != nil || c.ID == "" {
		return a.GetSetupState(), errors.New("add your OAuth client first")
	}
	cfg := auth.Config(c, scopes)
	tok, err := auth.Login(a.ctx, cfg, func(u string) { runtime.BrowserOpenURL(a.ctx, u) })
	if err != nil {
		return a.GetSetupState(), err
	}
	if tok.RefreshToken != "" {
		if err := auth.SaveSecret("refresh_token", tok.RefreshToken); err != nil {
			return a.GetSetupState(), errors.New("could not save to the system keychain: " + err.Error())
		}
	}
	a.mu.Lock()
	a.ts = cfg.TokenSource(context.Background(), tok)
	a.mu.Unlock()

	p, err := google.GetProfile(oauth2.NewClient(a.ctx, a.ts))
	if err != nil {
		return a.GetSetupState(), err
	}
	granted, _ := tok.Extra("scope").(string)
	a.db.Exec(`INSERT INTO accounts (email, name, picture, scopes, token_expiry, status)
		VALUES (?, ?, ?, ?, ?, 'ok')
		ON CONFLICT(email) DO UPDATE SET name=excluded.name, picture=excluded.picture,
		scopes=excluded.scopes, token_expiry=excluded.token_expiry, status='ok'`,
		p.Email, p.Name, p.Picture, granted, tok.Expiry.Format("2006-01-02T15:04:05Z07:00"))
	return a.GetSetupState(), nil
}

func (a *App) SignOut() (SetupState, error) {
	if rt, _ := auth.LoadSecret("refresh_token"); rt != "" {
		auth.Revoke(rt)
	}
	auth.DeleteSecret("refresh_token")
	a.db.Exec("DELETE FROM accounts")
	a.mu.Lock()
	a.ts = nil
	a.mu.Unlock()
	return a.GetSetupState(), nil
}

func (a *App) account() *Account {
	var acc Account
	err := a.db.QueryRow("SELECT email, name, picture FROM accounts ORDER BY id LIMIT 1").
		Scan(&acc.Email, &acc.Name, &acc.Picture)
	if err != nil {
		return nil
	}
	return &acc
}

// httpClient returns an authorized client, rebuilding the token source from
// the keychain after a restart.
func (a *App) httpClient() (*http.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ts == nil {
		c, _, err := a.client()
		if err != nil || c.ID == "" {
			return nil, errors.New("no OAuth client")
		}
		rt, _ := auth.LoadSecret("refresh_token")
		if rt == "" {
			return nil, errors.New("not signed in")
		}
		a.ts = auth.Config(c, nil).TokenSource(context.Background(), &oauth2.Token{RefreshToken: rt})
	}
	return oauth2.NewClient(a.ctx, a.ts), nil
}

// ---- per-feature setup ----

type ServiceStatus struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
	API     string `json:"api"` // the API that needs attention, as a ConsoleLinks key
}

// CheckService reports whether a feature is ready, or which setup step is next.
func (a *App) CheckService(service string) ServiceStatus {
	client, err := a.httpClient()
	if err != nil {
		return ServiceStatus{service, google.StatusNeedsSignIn, err.Error(), ""}
	}
	status, detail, api := google.CheckAPI(client, service)
	if status == google.StatusUnknownError && strings.Contains(detail, "invalid_grant") {
		a.db.Exec("UPDATE accounts SET status = 'needs_reconnect'")
		status, detail = google.StatusNeedsSignIn, "Google no longer accepts the saved sign-in."
	}
	return ServiceStatus{service, status, detail, api}
}

// ConsoleLinks are deep links into the user's own Cloud project.
func (a *App) ConsoleLinks() map[string]string {
	p := a.GetSetupState().ProjectID
	q := ""
	if p != "" {
		q = "?project=" + p
	}
	base := "https://console.cloud.google.com"
	return map[string]string{
		"createProject": base + "/projectcreate",
		"authOverview":  base + "/auth/overview" + q,
		"audience":      base + "/auth/audience" + q,
		"createClient":  base + "/auth/clients/create" + q,
		"gmail":         base + "/apis/library/gmail.googleapis.com" + q,
		"calendar":      base + "/apis/library/calendar-json.googleapis.com" + q,
		"youtube":       base + "/apis/library/youtube.googleapis.com" + q,
		"drive":         base + "/apis/library/drive.googleapis.com" + q,
		"docs":          base + "/apis/library/docs.googleapis.com" + q,
		"sheets":        base + "/apis/library/sheets.googleapis.com" + q,
		"tasks":         base + "/apis/library/tasks.googleapis.com" + q,
		"portability":   base + "/apis/library/dataportability.googleapis.com" + q,
		"geminiKey":     "https://aistudio.google.com/apikey",
	}
}

func (a *App) OpenURL(url string) { runtime.BrowserOpenURL(a.ctx, url) }

// PlayerURL is the loopback player page used by the YouTube embed test.
func (a *App) PlayerURL(videoID string) string { return a.playerBase + "/player?v=" + videoID }

// ---- YouTube ----

type YouTubeOverview struct {
	Channel       google.Channel        `json:"channel"`
	Subscriptions []google.Subscription `json:"subscriptions"`
}

// GetYouTubeOverview returns the user's channel and up to 500 subscriptions
// (at most 11 quota units).
func (a *App) GetYouTubeOverview() (YouTubeOverview, error) {
	client, err := a.httpClient()
	if err != nil {
		return YouTubeOverview{}, err
	}
	ch, err := google.MyChannel(client)
	if err != nil {
		return YouTubeOverview{}, err
	}
	subs, err := google.MySubscriptions(client, 500)
	return YouTubeOverview{Channel: ch, Subscriptions: subs}, err
}

// ---- Gmail ----

// GetInbox returns the cached inbox instantly; GmailList refreshes it.
func (a *App) GetInbox() ([]gmail.Thread, error) { return gmail.CachedInbox(a.db) }

type mailPage struct {
	page gmail.Page
	at   time.Time
}

// GmailList lists threads for a Gmail search and/or labels, a page at a time.
// The first page of each search is kept for two minutes; the plain inbox is
// also saved to SQLite so it shows instantly next time.
func (a *App) GmailList(q string, labelIDs []string, pageToken string) (gmail.Page, error) {
	key := q + "|" + strings.Join(labelIDs, ",")
	if pageToken == "" {
		a.mailMu.Lock()
		c, ok := a.mailCache[key]
		a.mailMu.Unlock()
		if ok && time.Since(c.at) < 2*time.Minute {
			return c.page, nil
		}
	}
	client, err := a.httpClient()
	if err != nil {
		return gmail.Page{}, err
	}
	p, err := gmail.ListThreads(client, q, labelIDs, pageToken, 50)
	if err != nil {
		return p, err
	}
	if p.Threads == nil {
		p.Threads = []gmail.Thread{}
	}
	if pageToken == "" {
		a.mailMu.Lock()
		a.mailCache[key] = mailPage{p, time.Now()}
		a.mailMu.Unlock()
		if q == "" && len(labelIDs) == 1 && labelIDs[0] == "INBOX" {
			gmail.SaveInbox(a.db, p.Threads)
		}
	}
	return p, nil
}

// forgetMail drops cached lists after a change, so the next list is fresh.
func (a *App) forgetMail() {
	a.mailMu.Lock()
	a.mailCache = map[string]mailPage{}
	a.mailMu.Unlock()
}

type GmailLabels struct {
	Labels []gmail.Label     `json:"labels"`
	Tree   []gmail.LabelNode `json:"tree"`
}

func (a *App) GmailLabels() (GmailLabels, error) {
	client, err := a.httpClient()
	if err != nil {
		return GmailLabels{}, err
	}
	labels, err := gmail.ListLabels(client)
	return GmailLabels{Labels: labels, Tree: gmail.LabelTree(labels)}, err
}

func (a *App) GmailCreateLabel(name string) (gmail.Label, error) {
	client, err := a.httpClient()
	if err != nil {
		return gmail.Label{}, err
	}
	return gmail.CreateLabel(client, name)
}

func (a *App) GmailRenameLabel(id, name string) (gmail.Label, error) {
	client, err := a.httpClient()
	if err != nil {
		return gmail.Label{}, err
	}
	return gmail.RenameLabel(client, id, name)
}

// GmailDeleteLabel removes a label; the mail that had it stays.
func (a *App) GmailDeleteLabel(id string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.forgetMail()
	return gmail.DeleteLabel(client, id)
}

// OpenThread returns every message in a thread and marks it read. Inline
// cid: images point at the loopback attachment route.
func (a *App) OpenThread(id string) ([]gmail.Message, error) {
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	msgs, err := gmail.GetThread(client, id)
	if err != nil {
		return nil, err
	}
	unread := false
	for i := range msgs {
		gmail.ResolveCIDs(&msgs[i], func(at gmail.Attachment) string { return a.GmailAttachmentURL(at.MessageID, at.PartID) })
		for _, l := range msgs[i].LabelIDs {
			unread = unread || l == "UNREAD"
		}
	}
	if unread {
		go gmail.MarkRead(client, a.db, id) // best effort; the list refresh shows the result
		a.forgetMail()
	}
	return msgs, nil
}

// GmailModify adds and removes labels: one thread with threads.modify, several
// with a single messages.batchModify over all their messages.
func (a *App) GmailModify(threadIDs, messageIDs, add, remove []string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.forgetMail()
	if len(threadIDs) == 1 || len(messageIDs) == 0 {
		for _, id := range threadIDs {
			if err := gmail.ModifyThread(client, id, add, remove); err != nil {
				return err
			}
		}
		return nil
	}
	return gmail.BatchModify(client, messageIDs, add, remove)
}

// GmailTrash moves threads to the trash, or back out of it (untrash).
func (a *App) GmailTrash(threadIDs []string, untrash bool) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.forgetMail()
	for _, id := range threadIDs {
		if untrash {
			err = gmail.UntrashThread(client, id)
		} else {
			err = gmail.TrashThread(client, id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// SendEmail sends a new email, reply or forward, and remembers its recipients
// for address autocomplete.
func (a *App) SendEmail(d gmail.Draft) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	if err := gmail.Send(client, d); err != nil {
		return err
	}
	a.rememberRecipients(d.To + ", " + d.Cc + ", " + d.Bcc)
	a.forgetMail()
	return nil
}

func (a *App) rememberRecipients(list string) {
	var recent []gmail.Contact
	json.Unmarshal([]byte(db.GetSetting(a.db, "gmail_recent")), &recent)
	addrs, _ := mail.ParseAddressList(strings.Trim(strings.ReplaceAll(list, ", ,", ","), ", "))
	for _, ad := range addrs {
		c := gmail.Contact{Name: ad.Name, Email: ad.Address}
		kept := []gmail.Contact{c}
		for _, r := range recent {
			if !strings.EqualFold(r.Email, c.Email) {
				kept = append(kept, r)
			}
		}
		recent = kept
	}
	if len(recent) > 200 {
		recent = recent[:200]
	}
	b, _ := json.Marshal(recent)
	db.SetSetting(a.db, "gmail_recent", string(b))
}

// GmailContacts lists people to suggest in To, Cc and Bcc: recent recipients
// first, then senders in the cached inbox.
func (a *App) GmailContacts() []gmail.Contact {
	var out []gmail.Contact
	json.Unmarshal([]byte(db.GetSetting(a.db, "gmail_recent")), &out)
	seen := map[string]bool{}
	for _, c := range out {
		seen[strings.ToLower(c.Email)] = true
	}
	for _, c := range gmail.CachedContacts(a.db) {
		if !seen[strings.ToLower(c.Email)] {
			out = append(out, c)
		}
	}
	return out
}

// GmailReplyDraft prefills a reply ("reply"), reply all ("all") or forward ("forward").
func (a *App) GmailReplyDraft(m gmail.Message, mode string) gmail.Draft {
	self := ""
	if acc := a.account(); acc != nil {
		self = acc.Email
	}
	return gmail.ReplyDraft(m, mode, self)
}

// GmailSaveDraft saves the compose window to Gmail's drafts (create, then update).
func (a *App) GmailSaveDraft(d gmail.Draft) (gmail.SavedDraft, error) {
	client, err := a.httpClient()
	if err != nil {
		return gmail.SavedDraft{}, err
	}
	return gmail.SaveDraft(client, d)
}

// GmailDeleteDraft discards a draft (drafts only; sent mail is never deleted).
func (a *App) GmailDeleteDraft(id string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return gmail.DeleteDraft(client, id)
}

type DraftsPage struct {
	Drafts        []gmail.DraftSummary `json:"drafts"`
	NextPageToken string               `json:"nextPageToken"`
}

func (a *App) GmailDrafts(pageToken string) (DraftsPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return DraftsPage{}, err
	}
	d, next, err := gmail.ListDrafts(client, pageToken)
	if d == nil {
		d = []gmail.DraftSummary{}
	}
	return DraftsPage{Drafts: d, NextPageToken: next}, err
}

func (a *App) GmailGetDraft(id string) (gmail.Draft, error) {
	client, err := a.httpClient()
	if err != nil {
		return gmail.Draft{}, err
	}
	return gmail.GetDraft(client, id)
}

type Signature struct {
	HTML   string `json:"html"`
	Source string `json:"source"` // gmail, local or ""
}

// GmailSignature is the signature set in Gmail, or the app's own one when
// Gmail's can't be read.
func (a *App) GmailSignature() Signature {
	if client, err := a.httpClient(); err == nil {
		if s, err := gmail.Signature(client); err == nil && s != "" {
			return Signature{HTML: s, Source: "gmail"}
		}
	}
	if s := db.GetSetting(a.db, "gmail_signature"); s != "" {
		return Signature{HTML: strings.ReplaceAll(html.EscapeString(s), "\n", "<br>"), Source: "local"}
	}
	return Signature{}
}

func (a *App) GmailLocalSignature() string { return db.GetSetting(a.db, "gmail_signature") }

func (a *App) GmailSetLocalSignature(text string) error {
	return db.SetSetting(a.db, "gmail_signature", text)
}

type LocalFile struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// GmailFileInfo describes local files to attach (from a drop or the picker).
func (a *App) GmailFileInfo(paths []string) []LocalFile {
	out := []LocalFile{}
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			out = append(out, LocalFile{Path: p, Name: filepath.Base(p), Size: info.Size()})
		}
	}
	return out
}

func (a *App) GmailPickFiles() ([]LocalFile, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "Attach files"})
	if err != nil {
		return nil, err
	}
	return a.GmailFileInfo(paths), nil
}

// GmailAttachmentURL is a loopback link to one attachment, for previews and inline images.
func (a *App) GmailAttachmentURL(messageID, partID string) string {
	return a.playerBase + "/gmail/attachment?m=" + url.QueryEscape(messageID) + "&p=" + url.QueryEscape(partID) + "&t=" + a.streamToken
}

func (a *App) gmailAttachment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !player.TokenOK(q.Get("t"), a.streamToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	client, err := a.httpClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	data, info, err := gmail.AttachmentData(client, q.Get("m"), q.Get("p"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", info.MimeType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(data)
}

// GmailDownloadAttachment saves an attachment into the download folder,
// reporting on "drive:progress" like other transfers.
func (a *App) GmailDownloadAttachment(messageID, partID, name string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	dir, err := a.downloadDir()
	if err != nil {
		return "", err
	}
	return a.startTransfer("download", name, func(ctx context.Context, progress func(float64)) (string, error) {
		data, _, err := gmail.AttachmentData(client, messageID, partID)
		if err != nil {
			return "", err
		}
		path := uniquePath(dir, name)
		progress(100)
		return path, os.WriteFile(path, data, 0o644)
	}), nil
}

// GmailSaveToDrive copies an attachment into My Drive.
func (a *App) GmailSaveToDrive(messageID, partID, name string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	return a.startTransfer("upload", name, func(ctx context.Context, progress func(float64)) (string, error) {
		data, _, err := gmail.AttachmentData(client, messageID, partID)
		if err != nil {
			return "", err
		}
		dir, err := os.MkdirTemp("", "googleplex-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		tmp := filepath.Join(dir, filepath.Base(name))
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			return "", err
		}
		f, err := drive.Upload(ctx, client, tmp, "root", progress)
		return f.ID, err
	}), nil
}

// ---- YouTube feed ----

type FeedResult struct {
	Videos []youtube.Video `json:"videos"`
	Failed int             `json:"failed"` // channels whose RSS feed could not be read
}

// GetYouTubeFeed returns the cached feed instantly.
func (a *App) GetYouTubeFeed() (FeedResult, error) {
	v, err := youtube.CachedFeed(a.db)
	return FeedResult{Videos: v}, err
}

// RefreshYouTubeFeed lists subscriptions (1 quota unit per 50 channels), then
// reads each channel's RSS feed, which costs no quota.
func (a *App) RefreshYouTubeFeed() (FeedResult, error) {
	client, err := a.httpClient()
	if err != nil {
		return FeedResult{}, err
	}
	subs, err := google.MySubscriptions(client, 500)
	if err != nil {
		return FeedResult{}, err
	}
	ids := make([]string, len(subs))
	for i, s := range subs {
		ids[i] = s.ChannelID
	}
	videos, failed := youtube.FetchFeed(ids, 200)
	// Lengths are a nice-to-have: if videos.list fails, the feed still shows.
	if err := youtube.AddDetails(client, a.db, videos); err != nil {
		log.Println("video details:", err)
	}
	if err := youtube.SaveFeed(a.db, videos); err != nil {
		return FeedResult{}, err
	}
	return FeedResult{Videos: videos, Failed: failed}, nil
}

// ---- downloads ----

type DownloadSettings struct {
	Dir          string `json:"dir"`
	YtdlpVersion string `json:"ytdlpVersion"` // "" until the first download installs it
	HasFFmpeg    bool   `json:"hasFFmpeg"`
}

// downloadDir is the chosen folder, or ~/Downloads/Googleplex.
func (a *App) downloadDir() (string, error) {
	dir := db.GetSetting(a.db, "download_dir")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "Downloads", "Googleplex")
	}
	return dir, os.MkdirAll(dir, 0o755)
}

func (a *App) GetDownloadSettings() (DownloadSettings, error) {
	dir, err := a.downloadDir()
	return DownloadSettings{Dir: dir, YtdlpVersion: ytdlp.Version(), HasFFmpeg: ytdlp.FindFFmpeg() != ""}, err
}

// ChooseDownloadDir asks for a folder and saves it. Cancelling keeps the old one.
func (a *App) ChooseDownloadDir() (DownloadSettings, error) {
	current, _ := a.downloadDir()
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Save downloads to", DefaultDirectory: current})
	if err != nil {
		return DownloadSettings{}, err
	}
	if dir != "" {
		if err := db.SetSetting(a.db, "download_dir", dir); err != nil {
			return DownloadSettings{}, err
		}
	}
	return a.GetDownloadSettings()
}

// UpdateYtdlp installs yt-dlp if needed and updates it; returns the new version.
func (a *App) UpdateYtdlp() (string, error) { return ytdlp.Update() }

// DownloadVideo starts a download and returns at once. Progress arrives as
// "download:progress" events carrying a ytdlp.Progress.
func (a *App) DownloadVideo(id, title string) error {
	dir, err := a.downloadDir()
	if err != nil {
		return err
	}
	a.dlMu.Lock()
	if _, running := a.downloads[id]; running {
		a.dlMu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.downloads[id] = cancel
	a.dlMu.Unlock()

	emit := func(p ytdlp.Progress) {
		p.VideoID, p.Title = id, title
		runtime.EventsEmit(a.ctx, "download:progress", p)
	}
	go func() {
		defer func() {
			a.dlMu.Lock()
			delete(a.downloads, id)
			a.dlMu.Unlock()
			cancel()
		}()
		emit(ytdlp.Progress{Status: "starting"})
		if err := ytdlp.Download(ctx, id, dir, emit); err != nil {
			emit(ytdlp.Progress{Status: "error", Error: err.Error()})
		}
	}()
	return nil
}

// CancelDownload stops a running download.
func (a *App) CancelDownload(id string) {
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	if cancel, ok := a.downloads[id]; ok {
		cancel()
	}
}

// RevealFile shows a downloaded file in the system file manager.
func (a *App) RevealFile(path string) error { return ytdlp.Reveal(path) }

// ---- pop-out player ----

// PopOut opens the video in its own always-on-top window (a second process of
// this executable, see pip.go), replacing any pop-out already open.
func (a *App) PopOut(id, title string, start int, stream string) error {
	if !player.ValidID(id) {
		return errors.New("bad video id")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	u, err := url.Parse(a.playerBase)
	if err != nil {
		return err
	}
	b := make([]byte, 24)
	rand.Read(b)
	token := hex.EncodeToString(b)

	args := []string{"--pip", "--video", id, "--title", title, "--start", strconv.Itoa(max(start, 0)),
		"--port", u.Port(), "--token", token}
	if stream != "" { // the video plays through yt-dlp: the pop-out uses this app's stream page
		args = append(args, "--stream", stream)
	}
	if x, y, ok := strings.Cut(db.GetSetting(a.db, "pip_position"), ","); ok {
		args = append(args, "--x", x, "--y", y)
	}

	a.ClosePopOut()
	cmd := exec.Command(exe, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open the pop-out window: %w", err)
	}
	a.pipMu.Lock()
	a.pipCmd, a.pipToken, a.pipTime, a.pipDocked = cmd, token, float64(start), false
	a.pipMu.Unlock()

	go func() {
		cmd.Wait()
		a.pipMu.Lock()
		current := a.pipCmd == cmd
		docked := a.pipDocked
		if current {
			a.pipCmd, a.pipToken = nil, ""
		}
		a.pipMu.Unlock()
		if current && !docked {
			runtime.EventsEmit(a.ctx, "pip:closed")
		}
	}()
	return nil
}

// BringBack closes the pop-out and returns where it had got to.
func (a *App) BringBack() float64 {
	a.pipMu.Lock()
	t := a.pipTime
	a.pipMu.Unlock()
	a.ClosePopOut()
	return t
}

// ClosePopOut closes the pop-out window if one is open.
func (a *App) ClosePopOut() {
	a.pipMu.Lock()
	cmd := a.pipCmd
	a.pipCmd, a.pipToken = nil, ""
	a.pipMu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// pipHandler takes the pop-out's reports. dock=true means "show it in the app
// again at this time".
func (a *App) pipHandler(dock bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		a.pipMu.Lock()
		ok := player.TokenOK(r.Header.Get("X-Pip-Token"), a.pipToken)
		a.pipMu.Unlock()
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var st struct {
			VideoID string   `json:"videoId"`
			Time    *float64 `json:"time"`
			X       int      `json:"x"`
			Y       int      `json:"y"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&st); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		db.SetSetting(a.db, "pip_position", fmt.Sprintf("%d,%d", st.X, st.Y))
		a.pipMu.Lock()
		if st.Time != nil {
			a.pipTime = *st.Time
		}
		if dock {
			a.pipDocked = true
		}
		t := a.pipTime
		a.pipMu.Unlock()
		if dock {
			runtime.EventsEmit(a.ctx, "pip:docked", map[string]any{"videoId": st.VideoID, "time": t})
			runtime.WindowUnminimise(a.ctx)
			runtime.WindowShow(a.ctx)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ---- YouTube search ----

// SearchYouTube searches with the YouTube API, falling back to yt-dlp (no
// quota) when Google's daily allowance is used up or the API call fails.
func (a *App) SearchYouTube(query, pageToken string) (youtube.SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return youtube.SearchResult{}, errors.New("type something to search for")
	}
	if r := []rune(query); len(r) > 200 {
		query = string(r[:200])
	}
	client, err := a.httpClient()
	if err == nil {
		var r youtube.SearchResult
		if r, err = youtube.Search(client, query, pageToken); err == nil {
			r.SearchesToday = youtube.CountSearch(a.db)
			return r, nil
		}
	}
	if !errors.Is(err, youtube.ErrQuota) && !ytdlp.Installed() {
		return youtube.SearchResult{}, err
	}
	out, ytErr := ytdlp.Search(a.ctx, query, 24)
	if ytErr != nil {
		return youtube.SearchResult{}, fmt.Errorf("%v; the yt-dlp fallback failed too: %v", err, ytErr)
	}
	videos, ytErr := youtube.ParseYtdlpSearch(out)
	if ytErr != nil {
		return youtube.SearchResult{}, ytErr
	}
	return youtube.SearchResult{Videos: videos, Source: "ytdlp", SearchesToday: youtube.SearchesToday(a.db)}, nil
}

// ---- Calendar ----

// GetCalendars returns the calendar list from Google (or the cached one when
// offline), each marked with whether Googleplex shows it.
func (a *App) GetCalendars() ([]calendar.Calendar, error) {
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	cals, err := calendar.ListCalendars(client)
	if err != nil {
		if cached := calendar.CachedCalendars(a.db); len(cached) > 0 {
			return cached, nil
		}
		return nil, err
	}
	return calendar.ApplyVisibility(a.db, cals), nil
}

func (a *App) SetCalendarVisible(id string, visible bool) error {
	return calendar.SetVisible(a.db, id, visible)
}

func parseRange(startISO, endISO string) (time.Time, time.Time, error) {
	start, err1 := time.Parse(time.RFC3339, startISO)
	end, err2 := time.Parse(time.RFC3339, endISO)
	if err1 != nil || err2 != nil || !end.After(start) {
		return time.Time{}, time.Time{}, errors.New("bad date range")
	}
	return start, end, nil
}

// GetEvents returns cached events for the shown calendars, instantly.
func (a *App) GetEvents(startISO, endISO string) ([]calendar.Event, error) {
	start, end, err := parseRange(startISO, endISO)
	if err != nil {
		return nil, err
	}
	return calendar.CachedRange(a.db, calendar.VisibleIDs(calendar.CachedCalendars(a.db)), start, end)
}

type EventsResult struct {
	Events []calendar.Event `json:"events"`
	Failed int              `json:"failed"` // calendars that could not be read
}

// RefreshEvents fetches the range from Google, caches it and returns it.
func (a *App) RefreshEvents(startISO, endISO string) (EventsResult, error) {
	start, end, err := parseRange(startISO, endISO)
	if err != nil {
		return EventsResult{}, err
	}
	client, err := a.httpClient()
	if err != nil {
		return EventsResult{}, err
	}
	ids := calendar.VisibleIDs(calendar.CachedCalendars(a.db))
	events, failed := calendar.ListEvents(client, ids, start, end)
	if failed == len(ids) && failed > 0 {
		return EventsResult{}, errors.New("could not read any calendar from Google")
	}
	if err := calendar.SaveRange(a.db, ids, start, end, events); err != nil {
		return EventsResult{}, err
	}
	cached, err := calendar.CachedRange(a.db, ids, start, end)
	return EventsResult{Events: cached, Failed: failed}, err
}

func (a *App) CreateEvent(in calendar.Input) (calendar.Event, error) {
	client, err := a.httpClient()
	if err != nil {
		return calendar.Event{}, err
	}
	e, err := calendar.CreateEvent(client, in)
	if err == nil {
		calendar.SaveEvent(a.db, e)
	}
	return e, err
}

// UpdateEvent saves an edit. scope (this, following, all) matters only for
// repeating events; in.MoveTo moves the event to another calendar afterwards.
func (a *App) UpdateEvent(scope string, in calendar.Input) (calendar.Event, error) {
	client, err := a.httpClient()
	if err != nil {
		return calendar.Event{}, err
	}
	e, err := calendar.UpdateEventScoped(client, scope, in)
	if err == nil {
		if e.CalendarID != in.CalendarID {
			calendar.ForgetEvent(a.db, in.CalendarID, in.EventID)
		}
		calendar.SaveEvent(a.db, e)
	}
	return e, err
}

func (a *App) DeleteEvent(calendarID, eventID, scope string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	if err := calendar.DeleteEventScoped(client, calendarID, eventID, scope); err != nil {
		return err
	}
	return calendar.ForgetEvent(a.db, calendarID, eventID)
}

// GetEvent reads one event; for a series master it includes the repeat rule.
func (a *App) GetEvent(calendarID, eventID string) (calendar.Event, error) {
	client, err := a.httpClient()
	if err != nil {
		return calendar.Event{}, err
	}
	return calendar.GetEvent(client, calendarID, eventID)
}

// GetWeekStart is 0 for Sunday (the default) or 1 for Monday.
func (a *App) GetWeekStart() int {
	if db.GetSetting(a.db, "cal_week_start") == "1" {
		return 1
	}
	return 0
}

func (a *App) SetWeekStart(day int) error {
	if day != 0 && day != 1 {
		return errors.New("week can start on Sunday (0) or Monday (1)")
	}
	return db.SetSetting(a.db, "cal_week_start", strconv.Itoa(day))
}

// QuickAddEvent creates an event from a sentence, Google-style.
func (a *App) QuickAddEvent(calendarID, text string) (calendar.Event, error) {
	client, err := a.httpClient()
	if err != nil {
		return calendar.Event{}, err
	}
	e, err := calendar.QuickAdd(client, calendarID, text)
	if err == nil {
		calendar.SaveEvent(a.db, e)
	}
	return e, err
}

// ---- yt-dlp playback (videos that block embedding) ----

// StreamURL resolves a playable file for a video whose owner blocks embedding
// and returns the loopback page that plays it. The first call installs yt-dlp.
func (a *App) StreamURL(id string) (string, error) {
	if !player.ValidID(id) {
		return "", errors.New("bad video id")
	}
	if _, err := ytdlp.StreamURL(a.ctx, id); err != nil {
		return "", err
	}
	return a.playerBase + "/stream?v=" + id + "&t=" + a.streamToken, nil
}

func (a *App) streamFor(w http.ResponseWriter, r *http.Request) (string, ytdlp.Stream, bool) {
	q := r.URL.Query()
	id := q.Get("v")
	if !player.TokenOK(q.Get("t"), a.streamToken) || !player.ValidID(id) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return "", ytdlp.Stream{}, false
	}
	s, ok := ytdlp.Cached(id)
	if !ok {
		var err error
		if s, err = ytdlp.StreamURL(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return "", ytdlp.Stream{}, false
		}
	}
	return id, s, true
}

// streamPage plays through the app's proxy, which fetches from YouTube in
// chunks; the direct links are only a fallback. Measured in Chrome over 60 s,
// direct links made the video stall 29 times, the proxy once.
func (a *App) streamPage(w http.ResponseWriter, r *http.Request) {
	id, s, ok := a.streamFor(w, r)
	if !ok {
		return
	}
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	proxy := "/stream/proxy?v=" + id + "&t=" + a.streamToken
	d := player.StreamPageData{
		Src:      proxy + "&part=video",
		Fallback: s.URL,
		Start:    max(start, 0),
		Autoplay: r.URL.Query().Get("autoplay") == "1",
	}
	if s.Audio != "" {
		d.AudioSrc, d.AudioFallback = proxy+"&part=audio", s.Audio
		d.VideoCodec, d.AudioCodec = s.VideoCodec, s.AudioCodec
		index := "/stream/index?v=" + id + "&t=" + a.streamToken
		d.VideoIndex, d.AudioIndex = index+"&part=video", index+"&part=audio"
	}
	player.ServeStreamPage(w, d)
}

// streamIndex returns a stream file's segment index (from its sidx box), so
// the Media Source player can fetch and seek segment by segment.
func (a *App) streamIndex(w http.ResponseWriter, r *http.Request) {
	_, s, ok := a.streamFor(w, r)
	if !ok {
		return
	}
	link := s.URL
	if r.URL.Query().Get("part") == "audio" {
		link = s.Audio
	}
	idx, err := ytdlp.IndexOf(r, s, link)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idx)
}

func (a *App) streamProxy(w http.ResponseWriter, r *http.Request) {
	if _, s, ok := a.streamFor(w, r); ok {
		link := s.URL
		if r.URL.Query().Get("part") == "audio" && s.Audio != "" {
			link = s.Audio
		}
		ytdlp.Proxy(w, r, s, link)
	}
}

// ---- Drive ----

// DriveList returns one page of a view: folder (with folderID, "" for My
// Drive), shared, recent, starred or trash. The first page is cached.
func (a *App) DriveList(view, folderID, orderBy, pageToken string) (drive.Page, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.Page{}, err
	}
	p, err := drive.List(client, view, folderID, orderBy, pageToken)
	if err == nil && pageToken == "" {
		drive.SaveListing(a.db, drive.CacheKey(view, folderID), p.Files)
	}
	return p, err
}

// DriveCached returns a view's first page as last seen, instantly.
func (a *App) DriveCached(view, folderID string) ([]drive.File, error) {
	return drive.CachedListing(a.db, drive.CacheKey(view, folderID))
}

func (a *App) DriveSearch(text, pageToken string) (drive.Page, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.Page{}, err
	}
	return drive.Search(client, text, pageToken)
}

func (a *App) DriveGet(id string) (drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.File{}, err
	}
	return drive.Get(client, id)
}

// DriveRecent lists recently viewed Docs ("docs") or Sheets ("sheets").
func (a *App) DriveRecent(kind string) ([]drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	mt := "application/vnd.google-apps.document"
	if kind == "sheets" {
		mt = "application/vnd.google-apps.spreadsheet"
	}
	return drive.RecentOfType(client, mt)
}

func (a *App) DriveExportFormats(mimeType string) []string { return drive.ExportFormats(mimeType) }

func (a *App) DriveCreateFolder(parentID, name string) (drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.File{}, err
	}
	return drive.CreateFolder(client, parentID, name)
}

func (a *App) DriveRename(id, name string) (drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.File{}, err
	}
	return drive.Rename(client, id, name)
}

func (a *App) DriveStar(id string, starred bool) (drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.File{}, err
	}
	return drive.SetStarred(client, id, starred)
}

// DriveTrash moves a file to Drive's trash (restorable there for 30 days).
func (a *App) DriveTrash(id string) (drive.File, error) {
	client, err := a.httpClient()
	if err != nil {
		return drive.File{}, err
	}
	return drive.Trash(client, id)
}

// DrivePreviewURL is a loopback link that streams a file, for image and PDF previews.
func (a *App) DrivePreviewURL(id string) string {
	return a.playerBase + "/drive/preview?id=" + url.QueryEscape(id) + "&t=" + a.streamToken
}

func (a *App) drivePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !player.TokenOK(q.Get("t"), a.streamToken) || q.Get("id") == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	client, err := a.httpClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resp, err := drive.Open(r.Context(), client, q.Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for _, h := range []string{"Content-Type", "Content-Length"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	io.Copy(w, resp.Body)
}

// ---- Drive transfers (progress arrives as "drive:progress" events) ----

func (a *App) startTransfer(kind, name string, run func(ctx context.Context, progress func(float64)) (string, error)) string {
	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)
	ctx, cancel := context.WithCancel(a.ctx)
	a.xferMu.Lock()
	a.transfers[id] = cancel
	a.xferMu.Unlock()
	emit := func(p drive.Progress) {
		p.ID, p.Kind, p.Name = id, kind, name
		runtime.EventsEmit(a.ctx, "drive:progress", p)
	}
	go func() {
		defer func() {
			a.xferMu.Lock()
			delete(a.transfers, id)
			a.xferMu.Unlock()
			cancel()
		}()
		emit(drive.Progress{Status: "running"})
		last := -1.0
		path, err := run(ctx, func(pct float64) {
			if pct-last >= 1 || pct == 100 { // about one event per percent
				last = pct
				emit(drive.Progress{Status: "running", Percent: pct})
			}
		})
		if err != nil {
			if ctx.Err() != nil {
				err = errors.New("cancelled")
			}
			emit(drive.Progress{Status: "error", Error: err.Error()})
			return
		}
		emit(drive.Progress{Status: "done", Percent: 100, Path: path})
	}()
	return id
}

func (a *App) DriveCancel(transferID string) {
	a.xferMu.Lock()
	defer a.xferMu.Unlock()
	if cancel, ok := a.transfers[transferID]; ok {
		cancel()
	}
}

// uniquePath adds " (2)", " (3)"… before the extension if the name is taken.
func uniquePath(dir, name string) string {
	name = strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(name)
	p := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}

// DriveDownload saves a regular file into the download folder.
func (a *App) DriveDownload(id string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	f, err := drive.Get(client, id)
	if err != nil {
		return "", err
	}
	if drive.IsGoogleFile(f.MimeType) {
		return "", errors.New("Google Docs, Sheets and Slides are downloaded with Export")
	}
	dir, err := a.downloadDir()
	if err != nil {
		return "", err
	}
	return a.startTransfer("download", f.Name, func(ctx context.Context, progress func(float64)) (string, error) {
		path := uniquePath(dir, f.Name)
		return path, drive.Download(ctx, client, f, path, progress)
	}), nil
}

// DriveExport saves a Google file converted to format (docx, xlsx, pptx, pdf…).
func (a *App) DriveExport(id, format string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	f, err := drive.Get(client, id)
	if err != nil {
		return "", err
	}
	dir, err := a.downloadDir()
	if err != nil {
		return "", err
	}
	name := drive.ExportName(f, format)
	return a.startTransfer("export", name, func(ctx context.Context, progress func(float64)) (string, error) {
		path := uniquePath(dir, name)
		return path, drive.Export(ctx, client, f, format, path, progress)
	}), nil
}

// DriveUploadPaths uploads local files (from a drop) into a folder.
func (a *App) DriveUploadPaths(folderID string, paths []string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	for _, p := range paths {
		path := p
		a.startTransfer("upload", filepath.Base(path), func(ctx context.Context, progress func(float64)) (string, error) {
			f, err := drive.Upload(ctx, client, path, folderID, progress)
			return f.ID, err
		})
	}
	return nil
}

// DriveUpload asks for files, then uploads them into a folder. It returns how
// many were picked.
func (a *App) DriveUpload(folderID string) (int, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "Upload to Drive"})
	if err != nil || len(paths) == 0 {
		return 0, err
	}
	return len(paths), a.DriveUploadPaths(folderID, paths)
}

// ---- Docs and Sheets ----

func (a *App) DocsGet(id string) (docs.Document, error) {
	client, err := a.httpClient()
	if err != nil {
		return docs.Document{}, err
	}
	return docs.Get(client, id)
}

func (a *App) DocsAppend(id, text string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return docs.Append(client, id, text)
}

func (a *App) DocsReplace(id, find, replace string, matchCase bool) (int, error) {
	client, err := a.httpClient()
	if err != nil {
		return 0, err
	}
	return docs.ReplaceAll(client, id, find, replace, matchCase)
}

func (a *App) SheetsGet(id string) (sheets.Spreadsheet, error) {
	client, err := a.httpClient()
	if err != nil {
		return sheets.Spreadsheet{}, err
	}
	return sheets.Get(client, id)
}

// SheetsValues reads rows [row, row+count) of the first cols columns.
func (a *App) SheetsValues(id, sheet string, row, count, cols int) ([][]string, error) {
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	return sheets.Values(client, id, sheet, row, count, cols)
}

func (a *App) SheetsFormula(id, sheet string, row, col int) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	return sheets.Formula(client, id, sheet, row, col)
}

// SheetsUpdate sets one cell as typed and returns what it now shows.
func (a *App) SheetsUpdate(id, sheet string, row, col int, value string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	return sheets.Update(client, id, sheet, row, col, value)
}

func (a *App) SheetsAppend(id, sheet string, cells []string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return sheets.Append(client, id, sheet, cells)
}

// ---- Tasks ----

// TasksCached returns the lists, the last selected list and its tasks as last
// seen, for an instant first draw.
type TasksSnapshot struct {
	Lists    []tasks.List `json:"lists"`
	Selected string       `json:"selected"`
	Tasks    []tasks.Task `json:"tasks"`
}

func (a *App) TasksCached() TasksSnapshot {
	sel := db.GetSetting(a.db, "tasks_list")
	return TasksSnapshot{Lists: tasks.CachedLists(a.db), Selected: sel, Tasks: tasks.CachedTasks(a.db, sel)}
}

func (a *App) TasksLists() ([]tasks.List, error) {
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	lists, err := tasks.Lists(client)
	if err == nil {
		tasks.SaveLists(a.db, lists)
	}
	return lists, err
}

// TasksOf fetches every task in a list, caches it and remembers the list as selected.
func (a *App) TasksOf(listID string) ([]tasks.Task, error) {
	db.SetSetting(a.db, "tasks_list", listID)
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	ts, err := tasks.Tasks(client, listID)
	if err == nil {
		tasks.SaveTasks(a.db, listID, ts)
	}
	return ts, err
}

// TasksCachedOf returns a list's tasks as last seen.
func (a *App) TasksCachedOf(listID string) []tasks.Task { return tasks.CachedTasks(a.db, listID) }

func (a *App) TasksCreateList(title string) (tasks.List, error) {
	client, err := a.httpClient()
	if err != nil {
		return tasks.List{}, err
	}
	return tasks.CreateList(client, title)
}

func (a *App) TasksRenameList(id, title string) (tasks.List, error) {
	client, err := a.httpClient()
	if err != nil {
		return tasks.List{}, err
	}
	return tasks.RenameList(client, id, title)
}

func (a *App) TasksDeleteList(id string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return tasks.DeleteList(client, id)
}

func (a *App) TasksCreate(listID string, n tasks.New) (tasks.Task, error) {
	client, err := a.httpClient()
	if err != nil {
		return tasks.Task{}, err
	}
	return tasks.Create(client, listID, n)
}

func (a *App) TasksUpdate(listID, taskID string, p tasks.Patch) (tasks.Task, error) {
	client, err := a.httpClient()
	if err != nil {
		return tasks.Task{}, err
	}
	return tasks.Update(client, listID, taskID, p)
}

func (a *App) TasksDelete(listID, taskID string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return tasks.Delete(client, listID, taskID)
}

func (a *App) TasksMove(listID, taskID, parent, previous, destList string) (tasks.Task, error) {
	client, err := a.httpClient()
	if err != nil {
		return tasks.Task{}, err
	}
	return tasks.Move(client, listID, taskID, parent, previous, destList)
}

func (a *App) TasksClearCompleted(listID string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return tasks.ClearCompleted(client, listID)
}

// ---- YouTube: playlists, likes, channels, comments ----
//
// Reads cost 1 quota unit and writes about 50, out of 10,000 a day, so the
// lists people revisit are kept in memory for a few minutes.

type ytCached struct {
	v  any
	at time.Time
}

func (a *App) ytGet(key string, ttl time.Duration, fetch func() (any, error)) (any, error) {
	a.ytMu.Lock()
	c, ok := a.ytCache[key]
	a.ytMu.Unlock()
	if ok && time.Since(c.at) < ttl {
		return c.v, nil
	}
	v, err := fetch()
	if err == nil {
		a.ytMu.Lock()
		a.ytCache[key] = ytCached{v, time.Now()}
		a.ytMu.Unlock()
	}
	return v, err
}

func (a *App) ytForget(prefix string) {
	a.ytMu.Lock()
	defer a.ytMu.Unlock()
	for k := range a.ytCache {
		if strings.HasPrefix(k, prefix) {
			delete(a.ytCache, k)
		}
	}
}

type PlaylistsPage struct {
	Playlists     []youtube.Playlist `json:"playlists"`
	NextPageToken string             `json:"nextPageToken"`
}

// YTPlaylists lists the user's playlists (channelID "") or a channel's public ones.
func (a *App) YTPlaylists(channelID, pageToken string) (PlaylistsPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return PlaylistsPage{}, err
	}
	v, err := a.ytGet("playlists:"+channelID+":"+pageToken, 5*time.Minute, func() (any, error) {
		p, next, err := youtube.Playlists(client, channelID, pageToken)
		return PlaylistsPage{p, next}, err
	})
	if err != nil {
		return PlaylistsPage{}, err
	}
	return v.(PlaylistsPage), nil
}

type PlaylistItemsPage struct {
	Items         []youtube.PlaylistItem `json:"items"`
	NextPageToken string                 `json:"nextPageToken"`
}

func (a *App) YTPlaylistItems(playlistID, pageToken string) (PlaylistItemsPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return PlaylistItemsPage{}, err
	}
	v, err := a.ytGet("items:"+playlistID+":"+pageToken, 5*time.Minute, func() (any, error) {
		items, next, err := youtube.PlaylistItems(client, playlistID, pageToken)
		return PlaylistItemsPage{items, next}, err
	})
	if err != nil {
		return PlaylistItemsPage{}, err
	}
	return v.(PlaylistItemsPage), nil
}

func (a *App) YTCreatePlaylist(title, privacy string) (youtube.Playlist, error) {
	client, err := a.httpClient()
	if err != nil {
		return youtube.Playlist{}, err
	}
	a.ytForget("playlists:")
	return youtube.CreatePlaylist(client, title, privacy)
}

func (a *App) YTRenamePlaylist(id, title, description string) (youtube.Playlist, error) {
	client, err := a.httpClient()
	if err != nil {
		return youtube.Playlist{}, err
	}
	a.ytForget("playlists:")
	return youtube.RenamePlaylist(client, id, title, description)
}

func (a *App) YTDeletePlaylist(id string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.ytForget("playlists:")
	return youtube.DeletePlaylist(client, id)
}

func (a *App) YTAddToPlaylist(playlistID, videoID string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	a.ytForget("items:" + playlistID)
	a.ytForget("playlists:")
	return youtube.AddToPlaylist(client, playlistID, videoID)
}

func (a *App) YTRemoveFromPlaylist(playlistID, itemID string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.ytForget("items:" + playlistID)
	a.ytForget("playlists:")
	return youtube.RemoveFromPlaylist(client, itemID)
}

func (a *App) YTMovePlaylistItem(itemID, playlistID, videoID string, position int) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.ytForget("items:" + playlistID)
	return youtube.MovePlaylistItem(client, itemID, playlistID, videoID, position)
}

// YTMembership says which of the user's playlists hold a video (playlist id -> item id).
func (a *App) YTMembership(videoID string) (map[string]string, error) {
	pl, err := a.YTPlaylists("", "")
	if err != nil {
		return nil, err
	}
	client, err := a.httpClient()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(pl.Playlists))
	for i, p := range pl.Playlists {
		ids[i] = p.ID
	}
	return youtube.Membership(client, ids, videoID)
}

type VideosPage struct {
	Videos        []youtube.Video `json:"videos"`
	NextPageToken string          `json:"nextPageToken"`
}

func (a *App) YTLiked(pageToken string) (VideosPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return VideosPage{}, err
	}
	v, err := a.ytGet("liked:"+pageToken, 5*time.Minute, func() (any, error) {
		vs, next, err := youtube.LikedVideos(client, pageToken)
		return VideosPage{vs, next}, err
	})
	if err != nil {
		return VideosPage{}, err
	}
	return v.(VideosPage), nil
}

// YTVideo reads a video's description, counts and the user's rating.
func (a *App) YTVideo(id string) (youtube.VideoInfo, error) {
	client, err := a.httpClient()
	if err != nil {
		return youtube.VideoInfo{}, err
	}
	v, err := a.ytGet("video:"+id, 10*time.Minute, func() (any, error) { return youtube.VideoDetails(client, id) })
	if err != nil {
		return youtube.VideoInfo{}, err
	}
	return v.(youtube.VideoInfo), nil
}

func (a *App) YTRate(videoID, rating string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.ytForget("video:" + videoID)
	a.ytForget("liked:")
	return youtube.Rate(client, videoID, rating)
}

type ChannelPage struct {
	Channel        youtube.ChannelInfo `json:"channel"`
	SubscriptionID string              `json:"subscriptionId"` // "" when not subscribed
}

func (a *App) YTChannel(id string) (ChannelPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return ChannelPage{}, err
	}
	v, err := a.ytGet("channel:"+id, time.Hour, func() (any, error) { return youtube.Channel(client, id) })
	if err != nil {
		return ChannelPage{}, err
	}
	sub, _ := a.YTSubscription(id)
	return ChannelPage{Channel: v.(youtube.ChannelInfo), SubscriptionID: sub}, nil
}

// YTSubscription is the user's subscription to a channel, or "".
func (a *App) YTSubscription(channelID string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	v, err := a.ytGet("sub:"+channelID, 10*time.Minute, func() (any, error) { return youtube.SubscriptionID(client, channelID) })
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (a *App) YTSubscribe(channelID string) (string, error) {
	client, err := a.httpClient()
	if err != nil {
		return "", err
	}
	a.ytForget("sub:" + channelID)
	return youtube.Subscribe(client, channelID)
}

func (a *App) YTUnsubscribe(channelID, subscriptionID string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	a.ytForget("sub:" + channelID)
	return youtube.Unsubscribe(client, subscriptionID)
}

type CommentsPage struct {
	Threads       []youtube.CommentThread `json:"threads"`
	NextPageToken string                  `json:"nextPageToken"`
}

func (a *App) YTComments(videoID, order, pageToken string) (CommentsPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return CommentsPage{}, err
	}
	t, next, err := youtube.CommentThreads(client, videoID, order, pageToken)
	return CommentsPage{t, next}, err
}

type RepliesPage struct {
	Replies       []youtube.Comment `json:"replies"`
	NextPageToken string            `json:"nextPageToken"`
}

func (a *App) YTReplies(parentID, pageToken string) (RepliesPage, error) {
	client, err := a.httpClient()
	if err != nil {
		return RepliesPage{}, err
	}
	r, next, err := youtube.Replies(client, parentID, pageToken)
	return RepliesPage{r, next}, err
}

func (a *App) YTPostComment(videoID, text string) (youtube.CommentThread, error) {
	client, err := a.httpClient()
	if err != nil {
		return youtube.CommentThread{}, err
	}
	return youtube.PostComment(client, videoID, text)
}

func (a *App) YTReply(parentID, text string) (youtube.Comment, error) {
	client, err := a.httpClient()
	if err != nil {
		return youtube.Comment{}, err
	}
	return youtube.Reply(client, parentID, text)
}

func (a *App) YTDeleteComment(id string) error {
	client, err := a.httpClient()
	if err != nil {
		return err
	}
	return youtube.DeleteComment(client, id)
}

// YTMyChannelID is the user's own channel id, to know which comments they can delete.
func (a *App) YTMyChannelID() string {
	client, err := a.httpClient()
	if err != nil {
		return ""
	}
	v, err := a.ytGet("mychannel", 24*time.Hour, func() (any, error) {
		ch, err := google.MyChannel(client)
		return ch.ID, err
	})
	if err != nil {
		return ""
	}
	return v.(string)
}

// ---- YouTube: local lists (the API has no Watch later or history) ----

func (a *App) YTWatchLater() ([]youtube.Video, error) { return youtube.WatchLater(a.db) }
func (a *App) YTAddWatchLater(v youtube.Video) error  { return youtube.AddWatchLater(a.db, v) }
func (a *App) YTRemoveWatchLater(videoID string) error {
	return youtube.RemoveWatchLater(a.db, videoID)
}
func (a *App) YTHistory() ([]youtube.Video, error)    { return youtube.History(a.db) }
func (a *App) YTAddHistory(v youtube.Video) error     { return youtube.AddHistory(a.db, v) }
func (a *App) YTClearHistory() error                  { return youtube.ClearHistory(a.db) }
func (a *App) YTRecentSearches() []string             { return youtube.RecentSearches(a.db) }
func (a *App) YTAddRecentSearch(q string) []string    { return youtube.AddRecentSearch(a.db, q) }
func (a *App) YTRemoveRecentSearch(q string) []string { return youtube.RemoveRecentSearch(a.db, q) }
func (a *App) YTClearRecentSearches() []string        { return youtube.ClearRecentSearches(a.db) }

// pipEnded hears that the pop-out's video finished, so the app's queue can
// play the next one there.
func (a *App) pipEnded(w http.ResponseWriter, r *http.Request) {
	a.pipMu.Lock()
	ok := player.TokenOK(r.Header.Get("X-Pip-Token"), a.pipToken)
	a.pipMu.Unlock()
	if r.Method != http.MethodPost || !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var st struct {
		VideoID string `json:"videoId"`
	}
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&st)
	runtime.EventsEmit(a.ctx, "pip:ended", map[string]any{"videoId": st.VideoID})
	w.WriteHeader(http.StatusNoContent)
}

// pipTheme tells the pop-out the current theme (it asks every few seconds).
func (a *App) pipTheme(w http.ResponseWriter, r *http.Request) {
	a.pipMu.Lock()
	ok := player.TokenOK(r.Header.Get("X-Pip-Token"), a.pipToken)
	a.pipMu.Unlock()
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Write([]byte(a.GetTheme()))
}
