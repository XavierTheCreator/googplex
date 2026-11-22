// Package tasks reads and changes Google Tasks over plain HTTP, and caches
// lists and tasks in SQLite so the Home page draws instantly.
package tasks

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var api = "https://tasks.googleapis.com/tasks/v1" // tests point this at httptest

type List struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Default bool   `json:"default"` // Google's own list, which can't be deleted
}

// Task is one task. Due is a date (YYYY-MM-DD) or "": Google Tasks keeps no
// time of day, so the app never converts it between time zones.
type Task struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes"`
	Due       string `json:"due"`
	Status    string `json:"status"` // needsAction or completed
	Completed string `json:"completed"`
	Parent    string `json:"parent"`
	Position  string `json:"position"`
	Updated   string `json:"updated"`
	Hidden    bool   `json:"hidden"`
	WebLink   string `json:"webViewLink"`
}

type apiTask struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes"`
	Due       string `json:"due"`
	Status    string `json:"status"`
	Completed string `json:"completed"`
	Parent    string `json:"parent"`
	Position  string `json:"position"`
	Updated   string `json:"updated"`
	Hidden    bool   `json:"hidden"`
	Deleted   bool   `json:"deleted"`
	WebLink   string `json:"webViewLink"`
}

func (a apiTask) task() Task {
	return Task{ID: a.ID, Title: a.Title, Notes: a.Notes, Due: dueFromAPI(a.Due), Status: a.Status, Completed: a.Completed,
		Parent: a.Parent, Position: a.Position, Updated: a.Updated, Hidden: a.Hidden, WebLink: a.WebLink}
}

// dueToAPI turns a date into what the API expects: RFC 3339 at midnight UTC.
func dueToAPI(date string) (string, error) {
	if date == "" {
		return "", nil
	}
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", errors.New("the due date should look like 2026-11-22")
	}
	return d.Format("2006-01-02") + "T00:00:00.000Z", nil
}

// dueFromAPI keeps just the date Google stored, whatever the local time zone.
func dueFromAPI(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return ""
}

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
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
		if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
			return fmt.Errorf("tasks: %s", e.Error.Message)
		}
		return fmt.Errorf("tasks: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func listURL(id string) string { return api + "/users/@me/lists/" + url.PathEscape(id) }
func tasksURL(listID string, rest ...string) string {
	u := api + "/lists/" + url.PathEscape(listID) + "/tasks"
	for _, r := range rest {
		u += "/" + url.PathEscape(r)
	}
	return u
}

// ---- lists ----

// Lists returns every task list; the first one Google returns is the default.
func Lists(client *http.Client) ([]List, error) {
	var lists []List
	page := ""
	for {
		var r struct {
			Items []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"maxResults": {"100"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := do(client, "GET", api+"/users/@me/lists?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for _, it := range r.Items {
			lists = append(lists, List{ID: it.ID, Title: it.Title, Default: len(lists) == 0})
		}
		if page = r.NextPageToken; page == "" {
			return lists, nil
		}
	}
}

func CreateList(client *http.Client, title string) (List, error) {
	if strings.TrimSpace(title) == "" {
		return List{}, errors.New("a list needs a name")
	}
	var l List
	err := do(client, "POST", api+"/users/@me/lists", map[string]any{"title": strings.TrimSpace(title)}, &l)
	return l, err
}

func RenameList(client *http.Client, id, title string) (List, error) {
	if strings.TrimSpace(title) == "" {
		return List{}, errors.New("a list needs a name")
	}
	var l List
	err := do(client, "PATCH", listURL(id), map[string]any{"title": strings.TrimSpace(title)}, &l)
	return l, err
}

// DeleteList deletes a list and its tasks. Google's default list can't be deleted.
func DeleteList(client *http.Client, id string) error {
	var def List
	if err := do(client, "GET", listURL("@default"), nil, &def); err != nil {
		return err
	}
	if def.ID == id {
		return errors.New("this is your default list, which Google doesn't let you delete")
	}
	return do(client, "DELETE", listURL(id), nil, nil)
}

// ---- tasks ----

// Tasks returns every task in a list, all pages, including completed and hidden ones.
func Tasks(client *http.Client, listID string) ([]Task, error) {
	out := []Task{}
	page := ""
	for {
		q := url.Values{"maxResults": {"100"}, "showCompleted": {"true"}, "showHidden": {"true"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		var r struct {
			Items         []apiTask `json:"items"`
			NextPageToken string    `json:"nextPageToken"`
		}
		if err := do(client, "GET", tasksURL(listID)+"?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for _, it := range r.Items {
			if !it.Deleted {
				out = append(out, it.task())
			}
		}
		if page = r.NextPageToken; page == "" {
			return out, nil
		}
	}
}

// New is a task to create. Parent makes it a subtask; Previous places it after a sibling.
type New struct {
	Title    string `json:"title"`
	Notes    string `json:"notes"`
	Due      string `json:"due"` // YYYY-MM-DD or ""
	Parent   string `json:"parent"`
	Previous string `json:"previous"`
}

func Create(client *http.Client, listID string, n New) (Task, error) {
	if strings.TrimSpace(n.Title) == "" {
		return Task{}, errors.New("a task needs a title")
	}
	body := map[string]any{"title": strings.TrimSpace(n.Title)}
	if n.Notes != "" {
		body["notes"] = n.Notes
	}
	due, err := dueToAPI(n.Due)
	if err != nil {
		return Task{}, err
	}
	if due != "" {
		body["due"] = due
	}
	q := url.Values{}
	if n.Parent != "" {
		q.Set("parent", n.Parent)
	}
	if n.Previous != "" {
		q.Set("previous", n.Previous)
	}
	u := tasksURL(listID)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var a apiTask
	err = do(client, "POST", u, body, &a)
	return a.task(), err
}

// Patch holds the fields to change; nil leaves a field as it is.
type Patch struct {
	Title  *string `json:"title"`
	Notes  *string `json:"notes"`
	Due    *string `json:"due"`    // "" clears the due date
	Status *string `json:"status"` // needsAction or completed
}

// patchBody builds the PATCH body. Reopening a task also clears its completed
// time, which Google otherwise keeps.
func patchBody(p Patch) (map[string]any, error) {
	b := map[string]any{}
	if p.Title != nil {
		if strings.TrimSpace(*p.Title) == "" {
			return nil, errors.New("a task needs a title")
		}
		b["title"] = strings.TrimSpace(*p.Title)
	}
	if p.Notes != nil {
		b["notes"] = *p.Notes
	}
	if p.Due != nil {
		due, err := dueToAPI(*p.Due)
		if err != nil {
			return nil, err
		}
		if due == "" {
			b["due"] = nil
		} else {
			b["due"] = due
		}
	}
	if p.Status != nil {
		switch *p.Status {
		case "completed":
			b["status"] = "completed"
		case "needsAction":
			b["status"] = "needsAction"
			b["completed"] = nil
		default:
			return nil, errors.New("status is needsAction or completed")
		}
	}
	return b, nil
}

func Update(client *http.Client, listID, taskID string, p Patch) (Task, error) {
	body, err := patchBody(p)
	if err != nil {
		return Task{}, err
	}
	var a apiTask
	err = do(client, "PATCH", tasksURL(listID, taskID), body, &a)
	return a.task(), err
}

func Delete(client *http.Client, listID, taskID string) error {
	return do(client, "DELETE", tasksURL(listID, taskID), nil, nil)
}

// Move places a task under parent (or at the top level when "") after
// previous (or first when ""), optionally into another list.
func Move(client *http.Client, listID, taskID, parent, previous, destList string) (Task, error) {
	q := url.Values{}
	if parent != "" {
		q.Set("parent", parent)
	}
	if previous != "" {
		q.Set("previous", previous)
	}
	if destList != "" && destList != listID {
		q.Set("destinationTasklist", destList)
	}
	u := tasksURL(listID, taskID) + "/move"
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var a apiTask
	err := do(client, "POST", u, nil, &a)
	return a.task(), err
}

// ClearCompleted hides every completed task in a list (Google keeps them, hidden).
func ClearCompleted(client *http.Client, listID string) error {
	return do(client, "POST", api+"/lists/"+url.PathEscape(listID)+"/clear", nil, nil)
}

// ---- cache ----

func save(conn *sql.DB, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = conn.Exec(`INSERT INTO tasks_cache (key, data) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET data = excluded.data`, key, string(b))
	return err
}

func load(conn *sql.DB, key string, v any) {
	var data string
	if conn.QueryRow("SELECT data FROM tasks_cache WHERE key = ?", key).Scan(&data) == nil {
		json.Unmarshal([]byte(data), v)
	}
}

func SaveLists(conn *sql.DB, lists []List) error { return save(conn, "lists", lists) }

func CachedLists(conn *sql.DB) []List {
	lists := []List{}
	load(conn, "lists", &lists)
	return lists
}

func SaveTasks(conn *sql.DB, listID string, ts []Task) error { return save(conn, "list:"+listID, ts) }

func CachedTasks(conn *sql.DB, listID string) []Task {
	ts := []Task{}
	load(conn, "list:"+listID, &ts)
	return ts
}
