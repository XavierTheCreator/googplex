// Package gmail lists and searches threads, reads messages with their
// attachments, applies labels and actions, builds and sends mail (with
// drafts), and caches the inbox in SQLite.
package gmail

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

var (
	api    = "https://gmail.googleapis.com/gmail/v1/users/me"        // tests point these at httptest
	upload = "https://gmail.googleapis.com/upload/gmail/v1/users/me" //
)

// Thread is one row in a thread list.
type Thread struct {
	ID           string   `json:"id"`
	Subject      string   `json:"subject"`
	FromName     string   `json:"fromName"`
	FromEmail    string   `json:"fromEmail"`
	Snippet      string   `json:"snippet"`
	Date         int64    `json:"date"` // unix ms of the newest message
	Unread       bool     `json:"unread"`
	Starred      bool     `json:"starred"`
	Important    bool     `json:"important"`
	MessageCount int      `json:"messageCount"`
	LabelIDs     []string `json:"labelIds"`   // across all its messages
	MessageIDs   []string `json:"messageIds"` // for batchModify on several threads at once
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Attachment is a file in a message. PartID finds it again (inline parts
// without an attachment id included); ContentID is set for cid: images.
type Attachment struct {
	MessageID string `json:"messageId"`
	PartID    string `json:"partId"`
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Size      int64  `json:"size"`
	ContentID string `json:"contentId"`
	Inline    bool   `json:"inline"`
}

// Message is one email inside an opened thread.
type Message struct {
	ID          string       `json:"id"`
	ThreadID    string       `json:"threadId"`
	MessageID   string       `json:"messageId"`  // RFC 2822 Message-ID, for In-Reply-To
	References  string       `json:"references"` // for threading a reply
	FromName    string       `json:"fromName"`
	From        string       `json:"from"`
	To          string       `json:"to"`
	Cc          string       `json:"cc"`
	Subject     string       `json:"subject"`
	Date        int64        `json:"date"`
	HTML        string       `json:"html"`
	Text        string       `json:"text"`
	LabelIDs    []string     `json:"labelIds"`
	Headers     []Header     `json:"headers"` // every header, for "show details"
	Attachments []Attachment `json:"attachments"`
}

// ---- Gmail API shapes ----

type part struct {
	PartID   string   `json:"partId"`
	MimeType string   `json:"mimeType"`
	Filename string   `json:"filename"`
	Headers  []Header `json:"headers"`
	Body     struct {
		AttachmentID string `json:"attachmentId"`
		Data         string `json:"data"`
		Size         int64  `json:"size"`
	} `json:"body"`
	Parts []part `json:"parts"`
}

type apiMessage struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	LabelIDs     []string `json:"labelIds"`
	Snippet      string   `json:"snippet"`
	InternalDate string   `json:"internalDate"`
	Payload      part     `json:"payload"`
}

type apiThread struct {
	ID       string       `json:"id"`
	Messages []apiMessage `json:"messages"`
}

func (p part) header(name string) string {
	for _, h := range p.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// do sends a JSON request and decodes a JSON reply (out may be nil).
func do(client *http.Client, method, u string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return apiError(resp)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func apiError(resp *http.Response) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
		return fmt.Errorf("gmail: %s", e.Error.Message)
	}
	return fmt.Errorf("gmail: %s", resp.Status)
}

func getJSON(client *http.Client, u string, out any) error { return do(client, "GET", u, nil, out) }

func splitFrom(from string) (name, email string) {
	if a, err := mail.ParseAddress(from); err == nil {
		if a.Name != "" {
			return a.Name, a.Address
		}
		return a.Address, a.Address
	}
	return from, from
}

// decodeHeader turns =?UTF-8?...?= encoded words into text.
func decodeHeader(s string) string {
	if out, err := new(mimeDecoder).DecodeHeader(s); err == nil {
		return out
	}
	return s
}

func decodeData(s string) []byte {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		b, _ = base64.RawURLEncoding.DecodeString(s)
	}
	return b
}

// ---- thread lists ----

// Page is one page of a thread list.
type Page struct {
	Threads       []Thread `json:"threads"`
	NextPageToken string   `json:"nextPageToken"`
}

// ListThreads lists threads matching a Gmail search (q, the same syntax as
// Gmail's search box) and/or labels, then reads each thread's headers, 8 at a time.
func ListThreads(client *http.Client, q string, labelIDs []string, pageToken string, max int) (Page, error) {
	v := url.Values{"maxResults": {strconv.Itoa(max)}}
	if q != "" {
		v.Set("q", q)
	}
	for _, l := range labelIDs {
		v.Add("labelIds", l)
		if l == "SPAM" || l == "TRASH" {
			v.Set("includeSpamTrash", "true")
		}
	}
	if pageToken != "" {
		v.Set("pageToken", pageToken)
	}
	var list struct {
		Threads []struct {
			ID string `json:"id"`
		} `json:"threads"`
		NextPageToken string `json:"nextPageToken"`
	}
	if err := getJSON(client, api+"/threads?"+v.Encode(), &list); err != nil {
		return Page{}, err
	}

	threads := make([]Thread, len(list.Threads))
	errs := make([]error, len(list.Threads))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, t := range list.Threads {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			threads[i], errs[i] = fetchThreadMeta(client, id)
		}(i, t.ID)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return Page{}, err
		}
	}
	return Page{Threads: threads, NextPageToken: list.NextPageToken}, nil
}

// FetchInbox gets the newest inbox threads.
func FetchInbox(client *http.Client, max int) ([]Thread, error) {
	p, err := ListThreads(client, "", []string{"INBOX"}, "", max)
	return p.Threads, err
}

func fetchThreadMeta(client *http.Client, id string) (Thread, error) {
	var t apiThread
	q := url.Values{"format": {"metadata"}, "metadataHeaders": {"From", "Subject"}}
	if err := getJSON(client, api+"/threads/"+url.PathEscape(id)+"?"+q.Encode(), &t); err != nil {
		return Thread{}, err
	}
	return threadMeta(id, t), nil
}

func threadMeta(id string, t apiThread) Thread {
	if len(t.Messages) == 0 {
		return Thread{ID: id}
	}
	first, last := t.Messages[0], t.Messages[len(t.Messages)-1]
	th := Thread{ID: id, Subject: decodeHeader(first.Payload.header("Subject")), Snippet: last.Snippet, MessageCount: len(t.Messages)}
	th.FromName, th.FromEmail = splitFrom(decodeHeader(last.Payload.header("From")))
	th.Date, _ = strconv.ParseInt(last.InternalDate, 10, 64)
	seen := map[string]bool{}
	for _, m := range t.Messages {
		th.MessageIDs = append(th.MessageIDs, m.ID)
		for _, l := range m.LabelIDs {
			switch l {
			case "UNREAD":
				th.Unread = true
			case "STARRED":
				th.Starred = true
			case "IMPORTANT":
				th.Important = true
			}
			if !seen[l] {
				seen[l] = true
				th.LabelIDs = append(th.LabelIDs, l)
			}
		}
	}
	return th
}

// ---- inbox cache ----

// SaveInbox replaces the cached inbox with what Gmail just returned.
func SaveInbox(conn *sql.DB, threads []Thread) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("UPDATE gmail_threads SET in_inbox = 0"); err != nil {
		return err
	}
	for _, t := range threads {
		_, err := tx.Exec(`INSERT INTO gmail_threads (id, subject, from_name, from_email, snippet, date, unread, message_count, in_inbox)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT(id) DO UPDATE SET subject=excluded.subject, from_name=excluded.from_name, from_email=excluded.from_email,
			snippet=excluded.snippet, date=excluded.date, unread=excluded.unread, message_count=excluded.message_count, in_inbox=1`,
			t.ID, t.Subject, t.FromName, t.FromEmail, t.Snippet, t.Date, t.Unread, t.MessageCount)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec("DELETE FROM gmail_threads WHERE in_inbox = 0"); err != nil {
		return err
	}
	return tx.Commit()
}

// CachedInbox returns the inbox as last saved, newest first.
func CachedInbox(conn *sql.DB) ([]Thread, error) {
	rows, err := conn.Query(`SELECT id, subject, from_name, from_email, snippet, date, unread, message_count
		FROM gmail_threads WHERE in_inbox = 1 ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	threads := []Thread{}
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.Subject, &t.FromName, &t.FromEmail, &t.Snippet, &t.Date, &t.Unread, &t.MessageCount); err != nil {
			return nil, err
		}
		t.LabelIDs = []string{"INBOX"}
		threads = append(threads, t)
	}
	return threads, rows.Err()
}

// Contact is someone the user has mail from, for address autocomplete.
type Contact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CachedContacts lists senders from the cached inbox, most recent first.
func CachedContacts(conn *sql.DB) []Contact {
	out := []Contact{}
	rows, err := conn.Query(`SELECT from_name, from_email FROM gmail_threads WHERE from_email LIKE '%@%' GROUP BY lower(from_email) ORDER BY max(date) DESC LIMIT 300`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var c Contact
		if rows.Scan(&c.Name, &c.Email) == nil {
			out = append(out, c)
		}
	}
	return out
}

// ---- reading ----

// GetThread fetches every message in a thread with its body and attachments.
func GetThread(client *http.Client, id string) ([]Message, error) {
	var t apiThread
	if err := getJSON(client, api+"/threads/"+url.PathEscape(id)+"?format=full", &t); err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(t.Messages))
	for _, m := range t.Messages {
		msgs = append(msgs, toMessage(m))
	}
	return msgs, nil
}

func toMessage(m apiMessage) Message {
	p := m.Payload
	msg := Message{
		ID:         m.ID,
		ThreadID:   m.ThreadID,
		MessageID:  p.header("Message-ID"),
		References: p.header("References"),
		From:       decodeHeader(p.header("From")),
		To:         decodeHeader(p.header("To")),
		Cc:         decodeHeader(p.header("Cc")),
		Subject:    decodeHeader(p.header("Subject")),
		LabelIDs:   m.LabelIDs,
		Headers:    p.Headers,
	}
	if msg.MessageID == "" {
		msg.MessageID = p.header("Message-Id")
	}
	msg.FromName, _ = splitFrom(msg.From)
	msg.Date, _ = strconv.ParseInt(m.InternalDate, 10, 64)
	msg.HTML = findBody(p, "text/html")
	msg.Text = findBody(p, "text/plain")
	msg.Attachments = attachments(m.ID, p)
	return msg
}

// findBody walks the MIME tree and returns the first non-attachment part of the given type.
func findBody(p part, mimeType string) string {
	if p.MimeType == mimeType && p.Filename == "" && p.Body.Data != "" {
		return string(decodeData(p.Body.Data))
	}
	for _, c := range p.Parts {
		if s := findBody(c, mimeType); s != "" {
			return s
		}
	}
	return ""
}

// attachments lists the parts that are files, including inline cid: images.
func attachments(messageID string, p part) []Attachment {
	var out []Attachment
	var walk func(p part)
	walk = func(p part) {
		cid := strings.Trim(p.header("Content-ID"), "<> ")
		if p.Filename != "" || (cid != "" && strings.HasPrefix(p.MimeType, "image/")) {
			name := decodeHeader(p.Filename)
			if name == "" {
				name = "image"
			}
			out = append(out, Attachment{
				MessageID: messageID, PartID: p.PartID, Name: name, MimeType: p.MimeType, Size: p.Body.Size,
				ContentID: cid, Inline: cid != "" && strings.HasPrefix(strings.ToLower(p.header("Content-Disposition")), "inline"),
			})
		}
		for _, c := range p.Parts {
			walk(c)
		}
	}
	walk(p)
	return out
}

// ResolveCIDs points cid: image references in a message's HTML at url(attachment).
func ResolveCIDs(m *Message, url func(a Attachment) string) {
	for _, a := range m.Attachments {
		if a.ContentID == "" {
			continue
		}
		u := url(a)
		m.HTML = strings.ReplaceAll(m.HTML, "cid:"+a.ContentID, u)
	}
}

// AttachmentData downloads one attachment's bytes.
func AttachmentData(client *http.Client, messageID, partID string) ([]byte, Attachment, error) {
	var m apiMessage
	if err := getJSON(client, api+"/messages/"+url.PathEscape(messageID)+"?format=full", &m); err != nil {
		return nil, Attachment{}, err
	}
	var found *part
	var walk func(p *part)
	walk = func(p *part) {
		if p.PartID == partID {
			found = p
			return
		}
		for i := range p.Parts {
			walk(&p.Parts[i])
		}
	}
	walk(&m.Payload)
	if found == nil {
		return nil, Attachment{}, fmt.Errorf("gmail: attachment not found")
	}
	info := Attachment{MessageID: messageID, PartID: partID, Name: decodeHeader(found.Filename), MimeType: found.MimeType, Size: found.Body.Size}
	if found.Body.AttachmentID == "" {
		return decodeData(found.Body.Data), info, nil
	}
	var a struct {
		Data string `json:"data"`
	}
	err := getJSON(client, api+"/messages/"+url.PathEscape(messageID)+"/attachments/"+url.PathEscape(found.Body.AttachmentID), &a)
	return decodeData(a.Data), info, err
}

// MarkRead removes the UNREAD label from a whole thread (needs gmail.modify).
func MarkRead(client *http.Client, conn *sql.DB, id string) error {
	if err := ModifyThread(client, id, nil, []string{"UNREAD"}); err != nil {
		return err
	}
	_, err := conn.Exec("UPDATE gmail_threads SET unread = 0 WHERE id = ?", id)
	return err
}
