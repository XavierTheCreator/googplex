package gmail

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestLabelTree(t *testing.T) {
	labels := []Label{
		{ID: "INBOX", Name: "INBOX", Type: "system"},
		{ID: "L3", Name: "Work/Clients/Acme", Type: "user"},
		{ID: "L1", Name: "Work", Type: "user"},
		{ID: "L4", Name: "receipts", Type: "user"},
		{ID: "L5", Name: "Trips/2026", Type: "user"}, // its parent "Trips" doesn't exist as a label
	}
	tree := LabelTree(labels)
	if len(tree) != 3 || tree[0].Short != "receipts" || tree[1].Short != "Trips" || tree[2].Short != "Work" {
		t.Fatalf("roots = %+v", tree)
	}
	if tree[1].Label.ID != "" || tree[1].Children[0].Label.ID != "L5" {
		t.Errorf("placeholder parent wrong: %+v", tree[1])
	}
	work := tree[2]
	if work.Label.ID != "L1" || work.Children[0].Short != "Clients" || work.Children[0].Children[0].Label.ID != "L3" || work.Children[0].Children[0].Short != "Acme" {
		t.Errorf("work = %+v", work)
	}
}

func TestModifyBodies(t *testing.T) {
	reqs, bodies, done := fakeGmail(t, func(*http.Request) string { return `{}` })
	defer done()
	ModifyThread(http.DefaultClient, "t1", nil, []string{"INBOX"})
	ids := make([]string, 1001)
	for i := range ids {
		ids[i] = "m"
	}
	BatchModify(http.DefaultClient, ids, []string{"STARRED"}, []string{"UNREAD"})
	TrashThread(http.DefaultClient, "t1")
	UntrashThread(http.DefaultClient, "t1")

	if (*reqs)[0].URL.Path != "/gmail/threads/t1/modify" || (*bodies)[0] != `{"removeLabelIds":["INBOX"]}` {
		t.Errorf("archive = %s %s", (*reqs)[0].URL.Path, (*bodies)[0])
	}
	var b map[string][]string
	json.Unmarshal([]byte((*bodies)[1]), &b)
	if (*reqs)[1].URL.Path != "/gmail/messages/batchModify" || len(b["ids"]) != 1000 || b["addLabelIds"][0] != "STARRED" || b["removeLabelIds"][0] != "UNREAD" {
		t.Errorf("batch = %s ids=%d %v", (*reqs)[1].URL.Path, len(b["ids"]), b)
	}
	json.Unmarshal([]byte((*bodies)[2]), &b)
	if len(b["ids"]) != 1 {
		t.Errorf("second batch should hold the 1001st id: %d", len(b["ids"]))
	}
	if !strings.HasSuffix((*reqs)[3].URL.Path, "/threads/t1/trash") || !strings.HasSuffix((*reqs)[4].URL.Path, "/threads/t1/untrash") {
		t.Errorf("trash = %s / %s", (*reqs)[3].URL.Path, (*reqs)[4].URL.Path)
	}
}

func TestListThreadsQuery(t *testing.T) {
	reqs, _, done := fakeGmail(t, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/threads") {
			return `{"threads": [{"id": "t1"}], "nextPageToken": "p2"}`
		}
		return `{"messages": [{"id": "m1", "labelIds": ["INBOX", "UNREAD", "STARRED"], "internalDate": "5",
			"payload": {"headers": [{"name": "From", "value": "Ana <ana@example.com>"}, {"name": "Subject", "value": "Hi"}]}}]}`
	})
	defer done()
	p, err := ListThreads(http.DefaultClient, "has:attachment from:ana", []string{"TRASH"}, "tok", 25)
	if err != nil || p.NextPageToken != "p2" || len(p.Threads) != 1 {
		t.Fatalf("page = %+v, %v", p, err)
	}
	th := p.Threads[0]
	if !th.Unread || !th.Starred || th.FromName != "Ana" || len(th.MessageIDs) != 1 {
		t.Errorf("thread = %+v", th)
	}
	q := (*reqs)[0].URL.Query()
	if q.Get("q") != "has:attachment from:ana" || q.Get("labelIds") != "TRASH" || q.Get("includeSpamTrash") != "true" || q.Get("pageToken") != "tok" {
		t.Errorf("query = %v", q)
	}
}

func TestResolveCIDs(t *testing.T) {
	m := Message{HTML: `<img src="cid:logo@x"><img src="cid:other">`, Attachments: []Attachment{{MessageID: "m1", PartID: "1.2", ContentID: "logo@x"}}}
	ResolveCIDs(&m, func(a Attachment) string { return "http://127.0.0.1/att?p=" + a.PartID })
	if m.HTML != `<img src="http://127.0.0.1/att?p=1.2"><img src="cid:other">` {
		t.Errorf("html = %s", m.HTML)
	}
}
