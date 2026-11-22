// Package calendar reads and writes Google Calendar events over plain HTTP
// and caches them in SQLite so the calendar draws instantly.
package calendar

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var api = "https://www.googleapis.com/calendar/v3" // tests point this at httptest

// Calendar is one calendar in the user's list.
type Calendar struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	Color      string `json:"color"`
	Primary    bool   `json:"primary"`
	Selected   bool   `json:"selected"` // shown in Google Calendar's own UI
	AccessRole string `json:"accessRole"`
	Visible    bool   `json:"visible"` // shown in Googleplex
}

// Writable reports whether events can be created in the calendar.
func (c Calendar) Writable() bool { return c.AccessRole == "owner" || c.AccessRole == "writer" }

type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	ResponseStatus string `json:"responseStatus"`
	Self           bool   `json:"self"`
	Organizer      bool   `json:"organizer"`
}

// Event is one event (recurring events arrive as their single instances).
// Start and End are RFC 3339 for timed events and YYYY-MM-DD for all-day
// ones, where End is the day after the last day, as Google sends it.
type Event struct {
	ID               string     `json:"id"`
	CalendarID       string     `json:"calendarId"`
	Summary          string     `json:"summary"`
	Description      string     `json:"description"`
	Location         string     `json:"location"`
	Start            string     `json:"start"`
	End              string     `json:"end"`
	AllDay           bool       `json:"allDay"`
	HTMLLink         string     `json:"htmlLink"`
	MeetLink         string     `json:"meetLink"`
	Attendees        []Attendee `json:"attendees"`
	Organizer        string     `json:"organizer"`
	ColorID          string     `json:"colorId"`
	RecurringEventID string     `json:"recurringEventId"`
	OriginalStart    string     `json:"originalStart"` // for an occurrence of a repeating event
	Recurrence       []string   `json:"recurrence"`    // only on a series master, from GetEvent
	OrganizerSelf    bool       `json:"organizerSelf"`
	GuestsCanModify  bool       `json:"guestsCanModify"`
	Status           string     `json:"status"`
}

// ---- Google API shapes ----

type apiTime struct {
	Date     string `json:"date,omitempty"`
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type apiEvent struct {
	ID                string     `json:"id"`
	Status            string     `json:"status"`
	HTMLLink          string     `json:"htmlLink"`
	Summary           string     `json:"summary"`
	Description       string     `json:"description"`
	Location          string     `json:"location"`
	ColorID           string     `json:"colorId"`
	Start             apiTime    `json:"start"`
	End               apiTime    `json:"end"`
	RecurringEventID  string     `json:"recurringEventId"`
	OriginalStartTime apiTime    `json:"originalStartTime"`
	Recurrence        []string   `json:"recurrence"`
	HangoutLink       string     `json:"hangoutLink"`
	Attendees         []Attendee `json:"attendees"`
	GuestsCanModify   bool       `json:"guestsCanModify"`
	Organizer         struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Self        bool   `json:"self"`
	} `json:"organizer"`
	ConferenceData struct {
		EntryPoints []struct {
			EntryPointType string `json:"entryPointType"`
			URI            string `json:"uri"`
		} `json:"entryPoints"`
	} `json:"conferenceData"`
}

func do(client *http.Client, method, u string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
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
			return fmt.Errorf("calendar: %s", e.Error.Message)
		}
		return fmt.Errorf("calendar: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func calURL(calendarID string, rest ...string) string {
	u := api + "/calendars/" + url.PathEscape(calendarID) + "/events"
	for _, r := range rest {
		u += "/" + url.PathEscape(r)
	}
	return u
}

// ---- reading ----

// ListCalendars returns every calendar in the user's list.
func ListCalendars(client *http.Client) ([]Calendar, error) {
	var cals []Calendar
	page := ""
	for {
		var r struct {
			Items []struct {
				ID              string `json:"id"`
				Summary         string `json:"summary"`
				SummaryOverride string `json:"summaryOverride"`
				BackgroundColor string `json:"backgroundColor"`
				Primary         bool   `json:"primary"`
				Selected        bool   `json:"selected"`
				AccessRole      string `json:"accessRole"`
			} `json:"items"`
			NextPageToken string `json:"nextPageToken"`
		}
		q := url.Values{"maxResults": {"250"}}
		if page != "" {
			q.Set("pageToken", page)
		}
		if err := do(client, "GET", api+"/users/me/calendarList?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for _, it := range r.Items {
			name := it.Summary
			if it.SummaryOverride != "" {
				name = it.SummaryOverride
			}
			cals = append(cals, Calendar{ID: it.ID, Summary: name, Color: it.BackgroundColor, Primary: it.Primary,
				Selected: it.Selected, AccessRole: it.AccessRole})
		}
		if page = r.NextPageToken; page == "" {
			return cals, nil
		}
	}
}

// ListEvents fetches events between min and max from each calendar, 4
// calendars at a time. Calendars that fail are skipped and counted.
func ListEvents(client *http.Client, calendarIDs []string, min, max time.Time) (events []Event, failed int) {
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, id := range calendarIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			evs, err := calendarEvents(client, id, min, max)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				return
			}
			events = append(events, evs...)
		}(id)
	}
	wg.Wait()
	return events, failed
}

func calendarEvents(client *http.Client, calendarID string, min, max time.Time) ([]Event, error) {
	var out []Event
	page := ""
	for {
		q := url.Values{
			"singleEvents": {"true"},
			"orderBy":      {"startTime"},
			"timeMin":      {min.Format(time.RFC3339)},
			"timeMax":      {max.Format(time.RFC3339)},
			"maxResults":   {"2500"},
		}
		if page != "" {
			q.Set("pageToken", page)
		}
		var r struct {
			Items         []apiEvent `json:"items"`
			NextPageToken string     `json:"nextPageToken"`
		}
		if err := do(client, "GET", calURL(calendarID)+"?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for _, it := range r.Items {
			if it.Status == "cancelled" {
				continue
			}
			out = append(out, toEvent(calendarID, it))
		}
		if page = r.NextPageToken; page == "" {
			return out, nil
		}
	}
}

func toEvent(calendarID string, a apiEvent) Event {
	e := Event{
		ID: a.ID, CalendarID: calendarID, Summary: a.Summary, Description: a.Description, Location: a.Location,
		HTMLLink: a.HTMLLink, ColorID: a.ColorID, RecurringEventID: a.RecurringEventID, Status: a.Status,
		Attendees: a.Attendees, Organizer: a.Organizer.Email, Recurrence: a.Recurrence,
		OrganizerSelf: a.Organizer.Self, GuestsCanModify: a.GuestsCanModify,
	}
	if a.Organizer.DisplayName != "" {
		e.Organizer = a.Organizer.DisplayName
	}
	e.Start, e.AllDay = when(a.Start)
	e.End, _ = when(a.End)
	if a.OriginalStartTime.Date != "" || a.OriginalStartTime.DateTime != "" {
		e.OriginalStart, _ = when(a.OriginalStartTime)
	}
	e.MeetLink = a.HangoutLink
	for _, ep := range a.ConferenceData.EntryPoints {
		if ep.EntryPointType == "video" && ep.URI != "" {
			e.MeetLink = ep.URI
			break
		}
	}
	return e
}

// when returns an all-day date as is, or a timed one as RFC 3339 with its
// offset. A dateTime without an offset is read in its timeZone.
func when(t apiTime) (string, bool) {
	if t.Date != "" {
		return t.Date, true
	}
	if ts, err := time.Parse(time.RFC3339, t.DateTime); err == nil {
		return ts.Format(time.RFC3339), false
	}
	loc := time.Local
	if l, err := time.LoadLocation(t.TimeZone); t.TimeZone != "" && err == nil {
		loc = l
	}
	if ts, err := time.ParseInLocation("2006-01-02T15:04:05", t.DateTime, loc); err == nil {
		return ts.Format(time.RFC3339), false
	}
	return t.DateTime, false
}

// Bounds returns an event's start and end as instants; all-day dates are
// midnight in the computer's own time zone.
func Bounds(e Event) (time.Time, time.Time) {
	parse := func(s string) time.Time {
		if e.AllDay {
			t, _ := time.ParseInLocation("2006-01-02", s, time.Local)
			return t
		}
		t, _ := time.Parse(time.RFC3339, s)
		return t
	}
	return parse(e.Start), parse(e.End)
}

// ---- cache ----

// SaveRange replaces the cached events of these calendars that overlap
// [min, max) with what Google just returned.
func SaveRange(conn *sql.DB, calendarIDs []string, min, max time.Time, events []Event) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range calendarIDs {
		if _, err := tx.Exec("DELETE FROM cal_events WHERE calendar_id = ? AND start_ms < ? AND end_ms > ?",
			id, max.UnixMilli(), min.UnixMilli()); err != nil {
			return err
		}
	}
	for _, e := range events {
		if err := saveOne(tx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func saveOne(tx interface {
	Exec(string, ...any) (sql.Result, error)
}, e Event) error {
	s, en := Bounds(e)
	data, _ := json.Marshal(e)
	_, err := tx.Exec(`INSERT OR REPLACE INTO cal_events (calendar_id, event_id, start_ms, end_ms, data) VALUES (?, ?, ?, ?, ?)`,
		e.CalendarID, e.ID, s.UnixMilli(), en.UnixMilli(), string(data))
	return err
}

// SaveEvent caches one created or updated event.
func SaveEvent(conn *sql.DB, e Event) error { return saveOne(conn, e) }

// ForgetEvent drops a deleted event from the cache.
func ForgetEvent(conn *sql.DB, calendarID, eventID string) error {
	_, err := conn.Exec("DELETE FROM cal_events WHERE calendar_id = ? AND event_id = ?", calendarID, eventID)
	return err
}

// CachedRange returns cached events of these calendars overlapping [min, max), by start.
func CachedRange(conn *sql.DB, calendarIDs []string, min, max time.Time) ([]Event, error) {
	events := []Event{}
	if len(calendarIDs) == 0 {
		return events, nil
	}
	args := []any{max.UnixMilli(), min.UnixMilli()}
	marks := make([]string, len(calendarIDs))
	for i, id := range calendarIDs {
		marks[i] = "?"
		args = append(args, id)
	}
	rows, err := conn.Query(`SELECT data FROM cal_events WHERE start_ms < ? AND end_ms > ? AND calendar_id IN (`+
		strings.Join(marks, ",")+`) ORDER BY start_ms`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var e Event
		if json.Unmarshal([]byte(data), &e) == nil {
			events = append(events, e)
		}
	}
	return events, rows.Err()
}
