package gmail

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MaxAttachments is Gmail's limit for the files in one message.
const MaxAttachments = 25 << 20

// Draft is an email to send or save. ThreadID, InReplyTo and References make
// it a reply. Files are local paths to attach; Forwarded are attachments of
// an existing message (a forward, or a saved draft's own files).
type Draft struct {
	DraftID    string       `json:"draftId"`
	To         string       `json:"to"`
	Cc         string       `json:"cc"`
	Bcc        string       `json:"bcc"`
	Subject    string       `json:"subject"`
	Body       string       `json:"body"` // plain text
	HTML       string       `json:"html"` // the same message as HTML, or ""
	ThreadID   string       `json:"threadId"`
	InReplyTo  string       `json:"inReplyTo"`
	References string       `json:"references"`
	Files      []string     `json:"files"`
	Forwarded  []Attachment `json:"forwarded"`
}

// file is an attachment ready to encode.
type file struct {
	Name, MimeType string
	Data           []byte
}

// collectFiles reads local files and fetches forwarded attachments, keeping
// the total under Gmail's limit.
func collectFiles(client *http.Client, d Draft) ([]file, error) {
	var out []file
	var total int64
	add := func(f file) error {
		total += int64(len(f.Data))
		if total > MaxAttachments {
			return fmt.Errorf("attachments add up to more than Gmail's 25 MB limit (%.1f MB so far); remove some or share them from Drive", float64(total)/(1<<20))
		}
		out = append(out, f)
		return nil
	}
	for _, p := range d.Files {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if total+info.Size() > MaxAttachments {
			return nil, fmt.Errorf("attachments add up to more than Gmail's 25 MB limit (%.1f MB with %s); remove some or share them from Drive",
				float64(total+info.Size())/(1<<20), filepath.Base(p))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(p)))
		if mt == "" {
			mt = "application/octet-stream"
		}
		if err := add(file{Name: filepath.Base(p), MimeType: mt, Data: b}); err != nil {
			return nil, err
		}
	}
	for _, a := range d.Forwarded {
		if total+a.Size > MaxAttachments {
			return nil, errors.New("attachments add up to more than Gmail's 25 MB limit; remove some or share them from Drive")
		}
		b, info, err := AttachmentData(client, a.MessageID, a.PartID)
		if err != nil {
			return nil, err
		}
		name, mt := a.Name, a.MimeType
		if name == "" {
			name = info.Name
		}
		if mt == "" {
			mt = info.MimeType
		}
		if err := add(file{Name: name, MimeType: mt, Data: b}); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// buildRaw renders the RFC 2822 message: text/plain, or multipart/alternative
// with HTML, wrapped in multipart/mixed when there are attachments.
// A draft may have no recipients yet; a sent message must have one.
func buildRaw(d Draft, files []file, draft bool) ([]byte, error) {
	if !draft && strings.TrimSpace(d.To+d.Cc+d.Bcc) == "" {
		return nil, errors.New("add at least one recipient")
	}
	var b bytes.Buffer
	header := func(name, value string) { b.WriteString(name + ": " + value + "\r\n") }
	for _, f := range []struct{ name, value string }{{"To", d.To}, {"Cc", d.Cc}, {"Bcc", d.Bcc}} {
		if strings.TrimSpace(f.value) == "" {
			continue
		}
		list, err := addressList(f.value)
		if err != nil {
			return nil, fmt.Errorf("check the %s field: %w", f.name, err)
		}
		header(f.name, list)
	}
	header("Subject", mime.QEncoding.Encode("utf-8", oneLine(d.Subject)))
	if id := oneLine(d.InReplyTo); id != "" {
		header("In-Reply-To", id)
		refs := strings.TrimSpace(oneLine(d.References))
		if !strings.Contains(refs, id) {
			refs = strings.TrimSpace(refs + " " + id)
		}
		header("References", refs)
	}
	header("MIME-Version", "1.0")

	body := func(w *bytes.Buffer) {
		if d.HTML == "" {
			writePart(w, "text/plain; charset=UTF-8", "", []byte(crlf(d.Body)))
			return
		}
		alt := boundary()
		w.WriteString("Content-Type: multipart/alternative; boundary=\"" + alt + "\"\r\n\r\n")
		w.WriteString("--" + alt + "\r\n")
		writePart(w, "text/plain; charset=UTF-8", "", []byte(crlf(d.Body)))
		w.WriteString("--" + alt + "\r\n")
		writePart(w, "text/html; charset=UTF-8", "", []byte(d.HTML))
		w.WriteString("--" + alt + "--\r\n")
	}

	if len(files) == 0 {
		body(&b)
		return b.Bytes(), nil
	}
	mixed := boundary()
	b.WriteString("Content-Type: multipart/mixed; boundary=\"" + mixed + "\"\r\n\r\n")
	b.WriteString("--" + mixed + "\r\n")
	body(&b)
	for _, f := range files {
		b.WriteString("--" + mixed + "\r\n")
		name := mime.QEncoding.Encode("utf-8", oneLine(f.Name))
		writePart(&b, f.MimeType+"; name=\""+name+"\"", "attachment; filename=\""+name+"\"", f.Data)
	}
	b.WriteString("--" + mixed + "--\r\n")
	return b.Bytes(), nil
}

// writePart writes one part's headers and its base64 body wrapped at 76 characters.
func writePart(w *bytes.Buffer, contentType, disposition string, data []byte) {
	w.WriteString("Content-Type: " + contentType + "\r\n")
	if disposition != "" {
		w.WriteString("Content-Disposition: " + disposition + "\r\n")
	}
	w.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 76 {
		w.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	w.WriteString(enc + "\r\n")
}

func boundary() string {
	b := make([]byte, 12)
	rand.Read(b)
	return "gp_" + hex.EncodeToString(b)
}

func crlf(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}

// addressList parses "a@x.com, Name <b@y.com>" and re-renders it safely encoded.
func addressList(s string) (string, error) {
	list, err := mail.ParseAddressList(oneLine(s))
	if err != nil {
		return "", err
	}
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.String()
	}
	return strings.Join(out, ", "), nil
}

// oneLine drops CR and LF so a header value can't add headers of its own.
func oneLine(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(s))
}

// uploadBody wraps metadata and a raw message as multipart/related, the form
// Gmail's upload endpoints take; it allows messages up to 35 MB.
func uploadBody(meta any, raw []byte) (string, []byte) {
	bd := boundary()
	var b bytes.Buffer
	m, _ := json.Marshal(meta)
	b.WriteString("--" + bd + "\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n")
	b.Write(m)
	b.WriteString("\r\n--" + bd + "\r\nContent-Type: message/rfc822\r\n\r\n")
	b.Write(raw)
	b.WriteString("\r\n--" + bd + "--\r\n")
	return "multipart/related; boundary=" + bd, b.Bytes()
}

func uploadRaw(client *http.Client, method, u string, meta any, raw []byte, out any) error {
	ctype, body := uploadBody(meta, raw)
	req, err := http.NewRequest(method, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ctype)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func message(client *http.Client, d Draft, draft bool) ([]byte, error) {
	files, err := collectFiles(client, d)
	if err != nil {
		return nil, err
	}
	return buildRaw(d, files, draft)
}

// Send sends the email as the signed-in user, then drops its saved draft, if any.
func Send(client *http.Client, d Draft) error {
	raw, err := message(client, d, false)
	if err != nil {
		return err
	}
	meta := map[string]any{}
	if d.ThreadID != "" {
		meta["threadId"] = d.ThreadID
	}
	if err := uploadRaw(client, "POST", upload+"/messages/send?uploadType=multipart", meta, raw, nil); err != nil {
		return err
	}
	if d.DraftID != "" {
		DeleteDraft(client, d.DraftID) // already sent; a leftover draft is only clutter
	}
	return nil
}

// ---- drafts ----

type SavedDraft struct {
	DraftID   string `json:"draftId"`
	MessageID string `json:"messageId"`
	ThreadID  string `json:"threadId"`
}

// SaveDraft creates the draft, or replaces it when d.DraftID is set.
func SaveDraft(client *http.Client, d Draft) (SavedDraft, error) {
	raw, err := message(client, d, true)
	if err != nil {
		return SavedDraft{}, err
	}
	msg := map[string]any{}
	if d.ThreadID != "" {
		msg["threadId"] = d.ThreadID
	}
	var r struct {
		ID      string `json:"id"`
		Message struct {
			ID       string `json:"id"`
			ThreadID string `json:"threadId"`
		} `json:"message"`
	}
	if d.DraftID == "" {
		err = uploadRaw(client, "POST", upload+"/drafts?uploadType=multipart", map[string]any{"message": msg}, raw, &r)
	} else {
		err = uploadRaw(client, "PUT", upload+"/drafts/"+url.PathEscape(d.DraftID)+"?uploadType=multipart",
			map[string]any{"id": d.DraftID, "message": msg}, raw, &r)
	}
	return SavedDraft{DraftID: r.ID, MessageID: r.Message.ID, ThreadID: r.Message.ThreadID}, err
}

// DeleteDraft discards a draft (only the draft; sent mail is never touched).
func DeleteDraft(client *http.Client, id string) error {
	return do(client, "DELETE", api+"/drafts/"+url.PathEscape(id), nil, nil)
}

// DraftSummary is one row in the Drafts folder.
type DraftSummary struct {
	DraftID   string `json:"draftId"`
	MessageID string `json:"messageId"`
	ThreadID  string `json:"threadId"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	Snippet   string `json:"snippet"`
	Date      int64  `json:"date"`
}

// ListDrafts lists saved drafts, newest first.
func ListDrafts(client *http.Client, pageToken string) ([]DraftSummary, string, error) {
	q := url.Values{"maxResults": {"50"}}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	var r struct {
		Drafts []struct {
			ID      string `json:"id"`
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		} `json:"drafts"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := getJSON(client, api+"/drafts?"+q.Encode(), &r); err != nil {
		return nil, "", err
	}
	out := make([]DraftSummary, len(r.Drafts))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, d := range r.Drafts {
		wg.Add(1)
		go func(i int, d struct {
			ID      string `json:"id"`
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var m apiMessage
			mq := url.Values{"format": {"metadata"}, "metadataHeaders": {"To", "Subject"}}
			getJSON(client, api+"/messages/"+url.PathEscape(d.Message.ID)+"?"+mq.Encode(), &m)
			s := DraftSummary{DraftID: d.ID, MessageID: d.Message.ID, ThreadID: m.ThreadID, To: decodeHeader(m.Payload.header("To")),
				Subject: decodeHeader(m.Payload.header("Subject")), Snippet: m.Snippet}
			fmt.Sscan(m.InternalDate, &s.Date)
			out[i] = s
		}(i, d)
	}
	wg.Wait()
	return out, r.NextPageToken, nil
}

// GetDraft opens a saved draft to keep editing it.
func GetDraft(client *http.Client, id string) (Draft, error) {
	var r struct {
		ID      string     `json:"id"`
		Message apiMessage `json:"message"`
	}
	if err := getJSON(client, api+"/drafts/"+url.PathEscape(id)+"?format=full", &r); err != nil {
		return Draft{}, err
	}
	m := toMessage(r.Message)
	p := r.Message.Payload
	d := Draft{DraftID: r.ID, To: m.To, Cc: m.Cc, Bcc: decodeHeader(p.header("Bcc")), Subject: m.Subject, Body: m.Text, HTML: m.HTML,
		ThreadID: m.ThreadID, InReplyTo: p.header("In-Reply-To"), References: m.References}
	for _, a := range m.Attachments {
		if !a.Inline {
			d.Forwarded = append(d.Forwarded, a)
		}
	}
	return d, nil
}
