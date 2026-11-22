package gmail

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parse(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	m, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("not a valid message: %v\n%s", err, raw)
	}
	return m
}

func decodePart(t *testing.T, p *multipart.Part) string {
	t.Helper()
	b, _ := io.ReadAll(p)
	for _, line := range strings.Split(strings.TrimRight(string(b), "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Errorf("base64 line longer than 76: %d", len(line))
		}
	}
	out, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(b), "\r\n", ""))
	if err != nil {
		t.Fatalf("part not base64: %v", err)
	}
	return string(out)
}

func TestPlainMessage(t *testing.T) {
	raw, err := buildRaw(Draft{To: "Ana <ana@example.com>, bo@example.com", Bcc: "hidden@example.com", Subject: "Hello", Body: "line one\nline two"}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	m := parse(t, raw)
	if m.Header.Get("To") != `"Ana" <ana@example.com>, <bo@example.com>` || m.Header.Get("Bcc") != "<hidden@example.com>" || m.Header.Get("Cc") != "" {
		t.Errorf("headers = %v", m.Header)
	}
	if m.Header.Get("In-Reply-To") != "" || !strings.HasPrefix(m.Header.Get("Content-Type"), "text/plain") {
		t.Errorf("headers = %v", m.Header)
	}
	b, _ := io.ReadAll(m.Body)
	got, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(b), "\r\n", ""))
	if string(got) != "line one\r\nline two" {
		t.Errorf("body = %q", got)
	}
}

func TestAlternativeAndAttachments(t *testing.T) {
	big := strings.Repeat("x", 500)
	raw, err := buildRaw(Draft{To: "a@example.com", Subject: "Réunion ☕", Body: "Hi", HTML: "<b>Hi</b>"},
		[]file{{Name: "café.pdf", MimeType: "application/pdf", Data: []byte(big)}}, false)
	if err != nil {
		t.Fatal(err)
	}
	m := parse(t, raw)
	if got := decodeHeader(m.Header.Get("Subject")); got != "Réunion ☕" || !strings.HasPrefix(m.Header.Get("Subject"), "=?utf-8?q?") {
		t.Errorf("subject = %s / %s", m.Header.Get("Subject"), got)
	}
	mt, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if mt != "multipart/mixed" {
		t.Fatalf("top type = %s", mt)
	}
	mr := multipart.NewReader(m.Body, params["boundary"])
	alt, _ := mr.NextPart()
	at, ap, _ := mime.ParseMediaType(alt.Header.Get("Content-Type"))
	if at != "multipart/alternative" {
		t.Fatalf("first part = %s", at)
	}
	ar := multipart.NewReader(alt, ap["boundary"])
	txt, _ := ar.NextPart() // each part must be read before asking for the next
	if decodePart(t, txt) != "Hi" {
		t.Error("text part wrong")
	}
	htm, _ := ar.NextPart()
	if decodePart(t, htm) != "<b>Hi</b>" || !strings.HasPrefix(htm.Header.Get("Content-Type"), "text/html") {
		t.Error("html part wrong")
	}
	att, _ := mr.NextPart()
	if decodePart(t, att) != big || !strings.HasPrefix(att.Header.Get("Content-Disposition"), "attachment") {
		t.Errorf("attachment wrong: %v", att.Header)
	}
	if fn := att.FileName(); decodeHeader(fn) != "café.pdf" {
		t.Errorf("filename = %q", fn)
	}
}

func TestReplyHeaders(t *testing.T) {
	raw, _ := buildRaw(Draft{To: "a@example.com", Subject: "Re: x", Body: "ok", InReplyTo: "<m2@x>", References: "<m1@x>"}, nil, false)
	m := parse(t, raw)
	if m.Header.Get("In-Reply-To") != "<m2@x>" || m.Header.Get("References") != "<m1@x> <m2@x>" {
		t.Errorf("reply headers = %v", m.Header)
	}
}

func TestRejects(t *testing.T) {
	if _, err := buildRaw(Draft{To: " "}, nil, false); err == nil {
		t.Error("no recipient should fail to send")
	}
	if _, err := buildRaw(Draft{Subject: "draft"}, nil, true); err != nil {
		t.Errorf("a draft may have no recipient: %v", err)
	}
	if _, err := buildRaw(Draft{To: "not an address"}, nil, false); err == nil {
		t.Error("bad To should fail")
	}
	raw, _ := buildRaw(Draft{To: "a@example.com", Subject: "hi\r\nBcc: evil@example.com"}, nil, false)
	if strings.Contains(string(raw), "\r\nBcc:") {
		t.Error("subject newline injected a header")
	}
}

func TestAttachmentLimit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big.bin")
	f, _ := os.Create(p)
	f.Truncate(MaxAttachments + 1)
	f.Close()
	_, err := collectFiles(http.DefaultClient, Draft{Files: []string{p}})
	if err == nil || !strings.Contains(err.Error(), "25 MB") {
		t.Errorf("err = %v", err)
	}
}

func TestReplyAllRecipients(t *testing.T) {
	m := Message{From: "Ana <ana@example.com>", To: "me@example.com, Bo <bo@example.com>", Cc: "cy@example.com, ME@example.com"}
	to, cc := ReplyRecipients(m, "me@example.com", true)
	if to != `"Ana" <ana@example.com>, "Bo" <bo@example.com>` || cc != "<cy@example.com>" {
		t.Errorf("reply all = %q / %q", to, cc)
	}
	to, cc = ReplyRecipients(m, "me@example.com", false)
	if to != `"Ana" <ana@example.com>` || cc != "" {
		t.Errorf("reply = %q / %q", to, cc)
	}
	// Replying to your own message goes back to its recipients.
	mine := Message{From: "Me <me@example.com>", To: "ana@example.com"}
	if to, _ := ReplyRecipients(mine, "me@example.com", false); to != "<ana@example.com>" {
		t.Errorf("reply to own = %q", to)
	}
}

func TestReplyAndForwardDrafts(t *testing.T) {
	m := Message{ID: "m1", ThreadID: "t1", MessageID: "<m1@x>", References: "<m0@x>", From: "Ana <ana@example.com>", To: "me@example.com",
		Subject: "Plans", Text: "Line 1\nLine <2>", Date: 1767225600000,
		Attachments: []Attachment{{MessageID: "m1", PartID: "2", Name: "a.pdf"}, {MessageID: "m1", PartID: "3", Name: "logo", Inline: true, ContentID: "c1"}}}
	r := ReplyDraft(m, "reply", "me@example.com")
	if r.Subject != "Re: Plans" || r.InReplyTo != "<m1@x>" || r.References != "<m0@x> <m1@x>" || r.ThreadID != "t1" {
		t.Errorf("reply = %+v", r)
	}
	if !strings.Contains(r.Body, "> Line 1\n> Line <2>") || !strings.Contains(r.HTML, "Line &lt;2&gt;") || strings.Contains(r.HTML, "<2>") {
		t.Errorf("quote = %q / %q", r.Body, r.HTML)
	}
	f := ReplyDraft(m, "forward", "me@example.com")
	if f.Subject != "Fwd: Plans" || f.To != "" || f.InReplyTo != "" || !strings.Contains(f.Body, "---------- Forwarded message ---------\nFrom: Ana <ana@example.com>") {
		t.Errorf("forward = %+v", f)
	}
	if len(f.Forwarded) != 1 || f.Forwarded[0].PartID != "2" {
		t.Errorf("forward attachments = %+v", f.Forwarded)
	}
	if ReplyDraft(Message{Subject: "RE: x"}, "reply", "").Subject != "RE: x" || ReplyDraft(Message{Subject: "Fw: x"}, "forward", "").Subject != "Fw: x" {
		t.Error("prefixes doubled")
	}
}

// fakeGmail records requests to the Gmail API and its upload endpoint.
func fakeGmail(t *testing.T, reply func(r *http.Request) string) (*[]*http.Request, *[]string, func()) {
	var reqs []*http.Request
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs, bodies = append(reqs, r), append(bodies, string(b))
		io.WriteString(w, reply(r))
	}))
	oldAPI, oldUp := api, upload
	api, upload = srv.URL+"/gmail", srv.URL+"/upload"
	return &reqs, &bodies, func() { api, upload = oldAPI, oldUp; srv.Close() }
}

func TestDraftCreateThenUpdate(t *testing.T) {
	reqs, bodies, done := fakeGmail(t, func(*http.Request) string {
		return `{"id": "r123", "message": {"id": "m9", "threadId": "t1"}}`
	})
	defer done()
	s, err := SaveDraft(http.DefaultClient, Draft{Subject: "Hi", Body: "first", ThreadID: "t1"})
	if err != nil || s.DraftID != "r123" {
		t.Fatalf("create = %+v, %v", s, err)
	}
	if _, err := SaveDraft(http.DefaultClient, Draft{DraftID: "r123", To: "a@example.com", Subject: "Hi", Body: "second", ThreadID: "t1"}); err != nil {
		t.Fatal(err)
	}
	c, u := (*reqs)[0], (*reqs)[1]
	if c.Method != "POST" || c.URL.Path != "/upload/drafts" || c.URL.Query().Get("uploadType") != "multipart" {
		t.Errorf("create = %s %s", c.Method, c.URL)
	}
	if u.Method != "PUT" || u.URL.Path != "/upload/drafts/r123" {
		t.Errorf("update = %s %s", u.Method, u.URL)
	}
	for i, r := range *reqs {
		mt, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if mt != "multipart/related" {
			t.Fatalf("upload %d type = %s", i, mt)
		}
		mr := multipart.NewReader(strings.NewReader((*bodies)[i]), params["boundary"])
		meta, _ := mr.NextPart()
		var m map[string]any
		json.NewDecoder(meta).Decode(&m)
		if m["message"].(map[string]any)["threadId"] != "t1" || (i == 1 && m["id"] != "r123") {
			t.Errorf("metadata %d = %v", i, m)
		}
		msg, _ := mr.NextPart()
		if msg.Header.Get("Content-Type") != "message/rfc822" {
			t.Errorf("message part = %v", msg.Header)
		}
	}
}

func TestSendDropsDraft(t *testing.T) {
	reqs, _, done := fakeGmail(t, func(*http.Request) string { return `{"id": "sent"}` })
	defer done()
	if err := Send(http.DefaultClient, Draft{DraftID: "r1", To: "a@example.com", Subject: "x", Body: "y", ThreadID: "t1"}); err != nil {
		t.Fatal(err)
	}
	if (*reqs)[0].URL.Path != "/upload/messages/send" || (*reqs)[1].Method != "DELETE" || (*reqs)[1].URL.Path != "/gmail/drafts/r1" {
		t.Errorf("calls = %s %s, %s %s", (*reqs)[0].Method, (*reqs)[0].URL.Path, (*reqs)[1].Method, (*reqs)[1].URL.Path)
	}
}
