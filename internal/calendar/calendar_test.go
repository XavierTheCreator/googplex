package calendar

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

func TestToEvent(t *testing.T) {
	var timed, allDay, naive apiEvent
	json.Unmarshal([]byte(`{"id": "a", "summary": "Standup", "status": "confirmed",
		"start": {"dateTime": "2026-11-21T09:00:00-05:00", "timeZone": "America/New_York"},
		"end": {"dateTime": "2026-11-21T09:15:00-05:00"},
		"hangoutLink": "https://meet.google.com/old",
		"conferenceData": {"entryPoints": [{"entryPointType": "phone", "uri": "tel:+1"}, {"entryPointType": "video", "uri": "https://meet.google.com/abc"}]},
		"organizer": {"email": "boss@example.com", "displayName": "Boss"},
		"attendees": [{"email": "me@example.com", "self": true, "responseStatus": "accepted"}]}`), &timed)
	json.Unmarshal([]byte(`{"id": "b", "start": {"date": "2026-11-26"}, "end": {"date": "2026-11-28"}}`), &allDay)
	json.Unmarshal([]byte(`{"id": "c", "start": {"dateTime": "2026-11-21T18:30:00", "timeZone": "Asia/Tokyo"}, "end": {"dateTime": "2026-11-21T19:30:00", "timeZone": "Asia/Tokyo"}}`), &naive)

	e := toEvent("primary", timed)
	if e.AllDay || e.Start != "2026-11-21T09:00:00-05:00" || e.End != "2026-11-21T09:15:00-05:00" {
		t.Errorf("timed: %+v", e)
	}
	if e.MeetLink != "https://meet.google.com/abc" || e.Organizer != "Boss" || len(e.Attendees) != 1 || !e.Attendees[0].Self {
		t.Errorf("timed details: %+v", e)
	}

	a := toEvent("primary", allDay)
	if !a.AllDay || a.Start != "2026-11-26" || a.End != "2026-11-28" {
		t.Errorf("all-day: %+v", a)
	}
	s, en := Bounds(a)
	if s.Hour() != 0 || s.Location() != time.Local || en.Sub(s) != 48*time.Hour && en.Sub(s) != 47*time.Hour && en.Sub(s) != 49*time.Hour {
		t.Errorf("all-day bounds: %v %v", s, en)
	}

	n := toEvent("primary", naive)
	if n.Start != "2026-11-21T18:30:00+09:00" {
		t.Errorf("no-offset dateTime should be read in its zone: %s", n.Start)
	}
	ns, _ := Bounds(n)
	if !ns.Equal(time.Date(2026, 11, 21, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("bounds = %v", ns)
	}
}

// fake records requests and answers like the Calendar API.
type fake struct {
	method, path, query string
	body                map[string]any
}

func serve(t *testing.T, reply func(r *http.Request) string) (*[]fake, func()) {
	var calls []fake
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f := fake{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			json.Unmarshal(b, &f.body)
		}
		calls = append(calls, f)
		io.WriteString(w, reply(r))
	}))
	old := api
	api = srv.URL
	return &calls, func() { api = old; srv.Close() }
}

func TestCreateEventBody(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string {
		return `{"id": "new1", "start": {"dateTime": "2026-11-22T10:00:00Z"}, "end": {"dateTime": "2026-11-22T11:00:00Z"}}`
	})
	defer done()
	e, err := CreateEvent(http.DefaultClient, Input{
		CalendarID: "me@example.com", Summary: "Dentist", Location: "Main St",
		Start: "2026-11-22T10:00:00Z", End: "2026-11-22T11:00:00Z", TimeZone: "America/Chicago",
		Guests: []string{" ana@example.com", "ANA@example.com", ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.method != "POST" || c.path != "/calendars/me@example.com/events" || c.query != "sendUpdates=all" {
		t.Errorf("request = %s %s?%s", c.method, c.path, c.query)
	}
	start := c.body["start"].(map[string]any)
	if start["dateTime"] != "2026-11-22T10:00:00Z" || start["timeZone"] != "America/Chicago" || start["date"] != nil {
		t.Errorf("start = %v", start)
	}
	if att := c.body["attendees"].([]any); len(att) != 1 || att[0].(map[string]any)["email"] != "ana@example.com" {
		t.Errorf("attendees = %v", c.body["attendees"])
	}
	if c.body["summary"] != "Dentist" || e.ID != "new1" || e.CalendarID != "me@example.com" {
		t.Errorf("summary %v, event %+v", c.body["summary"], e)
	}
}

func TestCreateAllDayNoGuests(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string {
		return `{"id": "x", "start": {"date": "2026-12-24"}, "end": {"date": "2026-12-26"}}`
	})
	defer done()
	if _, err := CreateEvent(http.DefaultClient, Input{CalendarID: "primary", Summary: "Holiday", AllDay: true, Start: "2026-12-24", End: "2026-12-26"}); err != nil {
		t.Fatal(err)
	}
	c := (*calls)[0]
	if c.query != "sendUpdates=none" {
		t.Errorf("no guests should not notify: %s", c.query)
	}
	if start := c.body["start"].(map[string]any); start["date"] != "2026-12-24" || start["dateTime"] != nil {
		t.Errorf("all-day start = %v", start)
	}
}

func TestUpdateKeepsReplies(t *testing.T) {
	calls, done := serve(t, func(r *http.Request) string {
		if r.Method == "GET" {
			return `{"id": "e1", "attendees": [{"email": "ana@example.com", "responseStatus": "accepted"}, {"email": "bo@example.com", "responseStatus": "declined"}]}`
		}
		return `{"id": "e1", "start": {"dateTime": "2026-11-22T10:00:00Z"}, "end": {"dateTime": "2026-11-22T12:00:00Z"}}`
	})
	defer done()
	_, err := UpdateEventScoped(http.DefaultClient, ScopeThis, Input{CalendarID: "primary", EventID: "e1", Summary: "Longer",
		Start: "2026-11-22T10:00:00Z", End: "2026-11-22T12:00:00Z", Guests: []string{"ana@example.com", "cy@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || (*calls)[1].method != "PATCH" || (*calls)[1].path != "/calendars/primary/events/e1" {
		t.Fatalf("calls = %+v", *calls)
	}
	att := (*calls)[1].body["attendees"].([]any)
	if len(att) != 2 || att[0].(map[string]any)["responseStatus"] != "accepted" || att[1].(map[string]any)["email"] != "cy@example.com" {
		t.Errorf("attendees = %v", att)
	}
}

func TestCalURLEscapesHash(t *testing.T) {
	if got := calURL("en.usa#holiday@group.v.calendar.google.com", "ev 1"); !strings.HasSuffix(got, "/calendars/en.usa%23holiday@group.v.calendar.google.com/events/ev%201") {
		t.Errorf("calURL = %s", got)
	}
}

func TestValidate(t *testing.T) {
	bad := []Input{
		{Summary: "no calendar", Start: "2026-11-22T10:00:00Z", End: "2026-11-22T11:00:00Z"},
		{CalendarID: "c", Start: "2026-11-22T11:00:00Z", End: "2026-11-22T10:00:00Z"},
		{CalendarID: "c", AllDay: true, Start: "2026-11-22", End: "2026-11-22"},
		{CalendarID: "c", Start: "tomorrow", End: "later"},
	}
	for _, in := range bad {
		if in.validate() == nil {
			t.Errorf("should reject %+v", in)
		}
	}
}

func TestListAndCache(t *testing.T) {
	_, done := serve(t, func(r *http.Request) string {
		if strings.Contains(r.URL.Path, "calendarList") {
			return `{"items": [{"id": "primary@example.com", "summary": "Me", "primary": true, "accessRole": "owner", "backgroundColor": "#9fc6e7"},
				{"id": "holidays", "summary": "Holidays", "summaryOverride": "Days off", "accessRole": "reader", "selected": false}]}`
		}
		if strings.Contains(r.URL.Path, "holidays") {
			return `{"items": [{"id": "h1", "start": {"date": "2026-11-26"}, "end": {"date": "2026-11-27"}}, {"id": "gone", "status": "cancelled"}]}`
		}
		return `{"items": [{"id": "m1", "start": {"dateTime": "2026-11-23T15:00:00Z"}, "end": {"dateTime": "2026-11-23T16:00:00Z"}}]}`
	})
	defer done()

	cals, err := ListCalendars(http.DefaultClient)
	if err != nil || len(cals) != 2 || cals[1].Summary != "Days off" || !cals[0].Writable() || cals[1].Writable() {
		t.Fatalf("cals = %+v, %v", cals, err)
	}
	conn, err := db.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cals = ApplyVisibility(conn, cals)
	if !cals[0].Visible || cals[1].Visible {
		t.Errorf("default visibility: %+v", cals)
	}
	SetVisible(conn, "holidays", true)
	cals = CachedCalendars(conn)
	ids := VisibleIDs(cals)
	if len(ids) != 2 {
		t.Fatalf("visible = %v", ids)
	}

	min := time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)
	max := min.AddDate(0, 0, 7)
	events, failed := ListEvents(http.DefaultClient, ids, min, max)
	if failed != 0 || len(events) != 2 {
		t.Fatalf("events = %+v, failed %d", events, failed)
	}
	if err := SaveRange(conn, ids, min, max, events); err != nil {
		t.Fatal(err)
	}
	got, err := CachedRange(conn, ids, min, max)
	if err != nil || len(got) != 2 {
		t.Fatalf("cached = %+v, %v", got, err)
	}
	if got, _ := CachedRange(conn, ids[:1], min, max); len(got) != 1 || got[0].ID != "m1" {
		t.Errorf("per-calendar cache = %+v", got)
	}
	// A refresh with nothing left removes the old rows in that range.
	SaveRange(conn, ids, min, max, nil)
	if got, _ := CachedRange(conn, ids, min, max); len(got) != 0 {
		t.Errorf("stale rows kept: %+v", got)
	}
}
