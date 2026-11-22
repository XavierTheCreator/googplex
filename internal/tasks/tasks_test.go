package tasks

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"googleplex/internal/db"
)

type call struct {
	method, path, query string
	body                map[string]any
	raw                 string
}

func serve(t *testing.T, reply func(r *http.Request) string) (*[]call, func()) {
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c := call{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, raw: string(b)}
		json.Unmarshal(b, &c.body)
		calls = append(calls, c)
		io.WriteString(w, reply(r))
	}))
	old := api
	api = srv.URL
	return &calls, func() { api = old; srv.Close() }
}

func TestDueDates(t *testing.T) {
	if got, _ := dueToAPI("2026-11-22"); got != "2026-11-22T00:00:00.000Z" {
		t.Errorf("dueToAPI = %s", got)
	}
	if got, _ := dueToAPI(""); got != "" {
		t.Errorf("empty due = %q", got)
	}
	if _, err := dueToAPI("22/11/2026"); err == nil {
		t.Error("bad date should fail")
	}
	// The stored date must come back unchanged in any time zone.
	for _, tz := range []string{"America/Los_Angeles", "Pacific/Kiritimati", "UTC"} {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			t.Skip("no tzdata")
		}
		old := time.Local
		time.Local = loc
		got := dueFromAPI("2026-11-22T00:00:00.000Z")
		time.Local = old
		if got != "2026-11-22" {
			t.Errorf("dueFromAPI in %s = %s", tz, got)
		}
	}
}

func TestCreateBodyAndPlacement(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string {
		return `{"id": "t1", "title": "Milk", "due": "2026-11-22T00:00:00.000Z", "status": "needsAction"}`
	})
	defer done()
	task, err := Create(http.DefaultClient, "L1", New{Title: " Milk ", Due: "2026-11-22", Parent: "p1", Previous: "s1"})
	if err != nil || task.Due != "2026-11-22" {
		t.Fatalf("create = %+v, %v", task, err)
	}
	c := (*calls)[0]
	if c.method != "POST" || c.path != "/lists/L1/tasks" || c.query != "parent=p1&previous=s1" {
		t.Errorf("request = %s %s?%s", c.method, c.path, c.query)
	}
	if c.body["title"] != "Milk" || c.body["due"] != "2026-11-22T00:00:00.000Z" || c.body["notes"] != nil {
		t.Errorf("body = %v", c.body)
	}
}

func TestCompleteAndReopen(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string { return `{"id": "t1"}` })
	defer done()
	completed, open, empty := "completed", "needsAction", ""
	Update(http.DefaultClient, "L1", "t1", Patch{Status: &completed})
	Update(http.DefaultClient, "L1", "t1", Patch{Status: &open})
	Update(http.DefaultClient, "L1", "t1", Patch{Due: &empty})
	if (*calls)[0].raw != `{"status":"completed"}` {
		t.Errorf("complete = %s", (*calls)[0].raw)
	}
	if (*calls)[1].raw != `{"completed":null,"status":"needsAction"}` {
		t.Errorf("reopen must clear completed: %s", (*calls)[1].raw)
	}
	if (*calls)[2].raw != `{"due":null}` || (*calls)[2].method != "PATCH" || (*calls)[2].path != "/lists/L1/tasks/t1" {
		t.Errorf("clear due = %s %s %s", (*calls)[2].method, (*calls)[2].path, (*calls)[2].raw)
	}
	blank := " "
	if _, err := Update(http.DefaultClient, "L1", "t1", Patch{Title: &blank}); err == nil {
		t.Error("blank title should fail")
	}
}

func TestMoveAndClear(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string { return `{"id": "t1"}` })
	defer done()
	Move(http.DefaultClient, "L1", "t1", "p1", "s1", "")
	Move(http.DefaultClient, "L1", "t1", "", "", "L2")
	Move(http.DefaultClient, "L1", "t1", "", "", "")
	ClearCompleted(http.DefaultClient, "L1")
	want := []string{
		"POST /lists/L1/tasks/t1/move?parent=p1&previous=s1",
		"POST /lists/L1/tasks/t1/move?destinationTasklist=L2",
		"POST /lists/L1/tasks/t1/move?",
		"POST /lists/L1/clear?",
	}
	for i, w := range want {
		got := (*calls)[i].method + " " + (*calls)[i].path + "?" + (*calls)[i].query
		if got != w {
			t.Errorf("call %d = %s, want %s", i, got, w)
		}
	}
}

func TestTasksAllPages(t *testing.T) {
	calls, done := serve(t, func(r *http.Request) string {
		if r.URL.Query().Get("pageToken") == "" {
			return `{"items": [{"id": "a", "title": "A"}, {"id": "gone", "deleted": true}], "nextPageToken": "p2"}`
		}
		return `{"items": [{"id": "b", "title": "B", "status": "completed", "hidden": true}]}`
	})
	defer done()
	ts, err := Tasks(http.DefaultClient, "L1")
	if err != nil || len(ts) != 2 || ts[1].ID != "b" || !ts[1].Hidden {
		t.Fatalf("tasks = %+v, %v", ts, err)
	}
	q := (*calls)[0].query
	if !strings.Contains(q, "showCompleted=true") || !strings.Contains(q, "showHidden=true") || (*calls)[1].query == q {
		t.Errorf("queries = %s / %s", q, (*calls)[1].query)
	}
}

func TestListsAndDeleteDefault(t *testing.T) {
	_, done := serve(t, func(r *http.Request) string {
		if strings.HasSuffix(r.URL.Path, "/@default") {
			return `{"id": "L1", "title": "My Tasks"}`
		}
		return `{"items": [{"id": "L1", "title": "My Tasks"}, {"id": "L2", "title": "Groceries"}]}`
	})
	defer done()
	lists, err := Lists(http.DefaultClient)
	if err != nil || len(lists) != 2 || !lists[0].Default || lists[1].Default {
		t.Fatalf("lists = %+v, %v", lists, err)
	}
	if err := DeleteList(http.DefaultClient, "L1"); err == nil || !strings.Contains(err.Error(), "default") {
		t.Errorf("deleting the default list should fail: %v", err)
	}
	if err := DeleteList(http.DefaultClient, "L2"); err != nil {
		t.Errorf("delete L2: %v", err)
	}
}

func TestCache(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if len(CachedLists(conn)) != 0 || len(CachedTasks(conn, "L1")) != 0 {
		t.Error("empty cache should be empty")
	}
	SaveLists(conn, []List{{ID: "L1", Title: "My Tasks", Default: true}})
	SaveTasks(conn, "L1", []Task{{ID: "a", Title: "A", Due: "2026-11-22"}})
	SaveTasks(conn, "L1", []Task{{ID: "b", Title: "B"}}) // replaces
	if l := CachedLists(conn); len(l) != 1 || !l[0].Default {
		t.Errorf("lists = %+v", l)
	}
	if ts := CachedTasks(conn, "L1"); len(ts) != 1 || ts[0].ID != "b" {
		t.Errorf("tasks = %+v", ts)
	}
}
