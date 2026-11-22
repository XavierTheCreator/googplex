package youtube

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"googleplex/internal/db"
)

type ytCall struct {
	method, path string
	q            map[string][]string
	body         map[string]any
}

func fakeYT(t *testing.T, reply func(r *http.Request) string) (*[]ytCall, func()) {
	var calls []ytCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := ytCall{method: r.Method, path: r.URL.Path, q: r.URL.Query()}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			json.Unmarshal(b, &c.body)
		}
		calls = append(calls, c)
		io.WriteString(w, reply(r))
	}))
	old := ytAPI
	ytAPI = srv.URL
	return &calls, func() { ytAPI = old; srv.Close() }
}

func get(m map[string]any, keys ...string) any {
	var v any = m
	for _, k := range keys {
		mm, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = mm[k]
	}
	return v
}

func TestRateAndSubscribe(t *testing.T) {
	calls, done := fakeYT(t, func(*http.Request) string { return `{"id": "sub1"}` })
	defer done()
	Rate(http.DefaultClient, "vid12345678", "like")
	if err := Rate(http.DefaultClient, "vid12345678", "love"); err == nil {
		t.Error("bad rating should fail")
	}
	id, _ := Subscribe(http.DefaultClient, "UCabc")
	Unsubscribe(http.DefaultClient, "sub1")

	c := (*calls)[0]
	if c.method != "POST" || c.path != "/videos/rate" || c.q["id"][0] != "vid12345678" || c.q["rating"][0] != "like" {
		t.Errorf("rate = %+v", c)
	}
	s := (*calls)[1]
	if s.method != "POST" || s.path != "/subscriptions" || get(s.body, "snippet", "resourceId", "channelId") != "UCabc" ||
		get(s.body, "snippet", "resourceId", "kind") != "youtube#channel" || id != "sub1" {
		t.Errorf("subscribe = %+v", s)
	}
	if u := (*calls)[2]; u.method != "DELETE" || u.q["id"][0] != "sub1" {
		t.Errorf("unsubscribe = %+v", u)
	}
}

func TestCommentBodies(t *testing.T) {
	calls, done := fakeYT(t, func(*http.Request) string {
		return `{"id": "c1", "snippet": {"topLevelComment": {"id": "c1", "snippet": {"textOriginal": "Nice", "authorDisplayName": "Me"}}}}`
	})
	defer done()
	th, err := PostComment(http.DefaultClient, "vid12345678", "Nice")
	if err != nil || th.Top.Text != "Nice" {
		t.Fatalf("post = %+v, %v", th, err)
	}
	Reply(http.DefaultClient, "c1", "Thanks")
	DeleteComment(http.DefaultClient, "c1")
	if _, err := PostComment(http.DefaultClient, "v", " "); err == nil {
		t.Error("empty comment should fail")
	}
	p := (*calls)[0]
	if p.path != "/commentThreads" || get(p.body, "snippet", "videoId") != "vid12345678" ||
		get(p.body, "snippet", "topLevelComment", "snippet", "textOriginal") != "Nice" {
		t.Errorf("post = %+v", p)
	}
	r := (*calls)[1]
	if r.path != "/comments" || get(r.body, "snippet", "parentId") != "c1" || get(r.body, "snippet", "textOriginal") != "Thanks" {
		t.Errorf("reply = %+v", r)
	}
	if d := (*calls)[2]; d.method != "DELETE" || d.path != "/comments" || d.q["id"][0] != "c1" {
		t.Errorf("delete = %+v", d)
	}
}

func TestPlaylistWrites(t *testing.T) {
	calls, done := fakeYT(t, func(*http.Request) string { return `{"id": "pi1", "snippet": {"title": "Mix"}}` })
	defer done()
	CreatePlaylist(http.DefaultClient, " Mix ", "")
	RenamePlaylist(http.DefaultClient, "PL1", "Mix 2", "for running")
	itemID, _ := AddToPlaylist(http.DefaultClient, "PL1", "vid12345678")
	MovePlaylistItem(http.DefaultClient, "pi1", "PL1", "vid12345678", 3)
	RemoveFromPlaylist(http.DefaultClient, "pi1")

	c := (*calls)[0]
	if c.method != "POST" || c.path != "/playlists" || get(c.body, "snippet", "title") != "Mix" || get(c.body, "status", "privacyStatus") != "private" {
		t.Errorf("create = %+v", c)
	}
	r := (*calls)[1]
	if r.method != "PUT" || get(r.body, "id") != "PL1" || get(r.body, "snippet", "title") != "Mix 2" || get(r.body, "snippet", "description") != "for running" {
		t.Errorf("rename = %+v", r)
	}
	a := (*calls)[2]
	if a.path != "/playlistItems" || get(a.body, "snippet", "playlistId") != "PL1" || get(a.body, "snippet", "resourceId", "videoId") != "vid12345678" || itemID != "pi1" {
		t.Errorf("add = %+v", a)
	}
	m := (*calls)[3]
	if m.method != "PUT" || get(m.body, "id") != "pi1" || get(m.body, "snippet", "position") != float64(3) || get(m.body, "snippet", "resourceId", "kind") != "youtube#video" {
		t.Errorf("move = %+v", m)
	}
	if d := (*calls)[4]; d.method != "DELETE" || d.q["id"][0] != "pi1" {
		t.Errorf("remove = %+v", d)
	}
}

func TestUploadsPlaylistID(t *testing.T) {
	if UploadsPlaylistID("UCabc123") != "UUabc123" || UploadsPlaylistID("HCxyz") != "" {
		t.Error("uploads id")
	}
}

func TestChannelAndQuota(t *testing.T) {
	_, done := fakeYT(t, func(r *http.Request) string {
		return `{"items": [{"id": "UCx", "snippet": {"title": "Chan", "thumbnails": {"high": {"url": "a.jpg"}}},
			"statistics": {"subscriberCount": "1200", "videoCount": "40"}, "contentDetails": {"relatedPlaylists": {}}}]}`
	})
	c, err := Channel(http.DefaultClient, "UCx")
	done()
	if err != nil || c.Uploads != "UUx" || c.Subscribers != 1200 || c.Avatar != "a.jpg" {
		t.Errorf("channel = %+v, %v", c, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		io.WriteString(w, `{"error": {"errors": [{"reason": "quotaExceeded"}]}}`)
	}))
	defer srv.Close()
	old := ytAPI
	ytAPI = srv.URL
	defer func() { ytAPI = old }()
	if _, _, err := LikedVideos(http.DefaultClient, ""); err != ErrQuota {
		t.Errorf("quota error = %v", err)
	}
}

func TestLocalLists(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	AddWatchLater(conn, Video{ID: "a", Title: "A"})
	AddWatchLater(conn, Video{ID: "b", Title: "B"})
	AddWatchLater(conn, Video{ID: "a", Title: "A2"}) // already there: updated, not doubled
	wl, _ := WatchLater(conn)
	if len(wl) != 2 {
		t.Errorf("watch later = %+v", wl)
	}
	RemoveWatchLater(conn, "b")
	if wl, _ := WatchLater(conn); len(wl) != 1 || wl[0].Title != "A2" {
		t.Errorf("after remove = %+v", wl)
	}

	AddHistory(conn, Video{ID: "x"})
	AddHistory(conn, Video{ID: "y"})
	AddHistory(conn, Video{ID: "x"}) // replayed: moves to the top
	h, _ := History(conn)
	if len(h) != 2 || h[0].ID != "x" {
		t.Errorf("history = %+v", h)
	}
	ClearHistory(conn)
	if h, _ := History(conn); len(h) != 0 {
		t.Error("history not cleared")
	}

	AddRecentSearch(conn, "lofi")
	AddRecentSearch(conn, "jazz")
	if got := AddRecentSearch(conn, "LOFI "); len(got) != 2 || got[0] != "LOFI" {
		t.Errorf("recent = %v", got)
	}
	if got := RemoveRecentSearch(conn, "jazz"); len(got) != 1 {
		t.Errorf("after remove = %v", got)
	}
	if got := ClearRecentSearches(conn); len(got) != 0 || len(RecentSearches(conn)) != 0 {
		t.Error("recent not cleared")
	}
}
