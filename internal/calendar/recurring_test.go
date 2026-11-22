package calendar

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEndBefore(t *testing.T) {
	cut := time.Date(2026, 12, 1, 15, 0, 0, 0, time.UTC)
	got := endBefore([]string{"RRULE:FREQ=WEEKLY;COUNT=10;BYDAY=TU", "EXDATE:20261124T150000Z"}, cut, false)
	want := []string{"RRULE:FREQ=WEEKLY;BYDAY=TU;UNTIL=20261201T145959Z", "EXDATE:20261124T150000Z"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("timed: %v", got)
	}
	day := time.Date(2026, 12, 1, 0, 0, 0, 0, time.Local)
	if got := endBefore([]string{"RRULE:FREQ=DAILY;UNTIL=20270101"}, day, true); got[0] != "RRULE:FREQ=DAILY;UNTIL=20261130" {
		t.Errorf("all-day: %v", got)
	}
}

func TestWithoutCountAndShiftDays(t *testing.T) {
	if got := withoutCount([]string{"RRULE:FREQ=DAILY;COUNT=5;INTERVAL=2"}); got[0] != "RRULE:FREQ=DAILY;INTERVAL=2" {
		t.Errorf("withoutCount: %v", got)
	}
	cases := map[int]string{1: "RRULE:FREQ=WEEKLY;BYDAY=TU,SA", -1: "RRULE:FREQ=WEEKLY;BYDAY=SU,TH", 7: "RRULE:FREQ=WEEKLY;BYDAY=MO,FR"}
	for n, want := range cases {
		if got := shiftDays([]string{"RRULE:FREQ=WEEKLY;BYDAY=MO,FR"}, n); got[0] != want {
			t.Errorf("shiftDays(%d) = %v", n, got)
		}
	}
	if got := shiftDays([]string{"RRULE:FREQ=MONTHLY;BYDAY=3SA"}, 1); got[0] != "RRULE:FREQ=MONTHLY;BYDAY=3SU" {
		t.Errorf("nth weekday: %v", got)
	}
}

func TestCreateRepeatingSendsRuleAndZone(t *testing.T) {
	calls, done := serve(t, func(*http.Request) string { return `{"id": "s1"}` })
	defer done()
	_, err := CreateEvent(http.DefaultClient, Input{CalendarID: "primary", Summary: "Gym",
		Start: "2026-11-24T18:00:00-05:00", End: "2026-11-24T19:00:00-05:00", TimeZone: "America/New_York",
		Recurrence: []string{"RRULE:FREQ=WEEKLY;BYDAY=TU,TH"}})
	if err != nil {
		t.Fatal(err)
	}
	b := (*calls)[0].body
	if rec := b["recurrence"].([]any); len(rec) != 1 || rec[0] != "RRULE:FREQ=WEEKLY;BYDAY=TU,TH" {
		t.Errorf("recurrence = %v", b["recurrence"])
	}
	if b["start"].(map[string]any)["timeZone"] != "America/New_York" || b["end"].(map[string]any)["timeZone"] != "America/New_York" {
		t.Errorf("time zones missing: %v", b)
	}
}

// A weekly Tuesday 9:00-10:00 series; its Dec 1 occurrence is "m1_20261201T140000Z".
func seriesServer(t *testing.T) (*[]fake, func()) {
	return serve(t, func(r *http.Request) string {
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/m1_20261201T140000Z"):
			return `{"id": "m1_20261201T140000Z", "recurringEventId": "m1",
				"originalStartTime": {"dateTime": "2026-12-01T09:00:00-05:00"},
				"start": {"dateTime": "2026-12-01T09:00:00-05:00"}, "end": {"dateTime": "2026-12-01T10:00:00-05:00"}}`
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/m1"):
			return `{"id": "m1", "summary": "1:1", "recurrence": ["RRULE:FREQ=WEEKLY;BYDAY=TU;COUNT=20"],
				"attendees": [{"email": "me@example.com", "self": true, "responseStatus": "accepted"}, {"email": "ana@example.com", "responseStatus": "accepted"}],
				"start": {"dateTime": "2026-11-03T09:00:00-05:00", "timeZone": "America/New_York"}, "end": {"dateTime": "2026-11-03T10:00:00-05:00"}}`
		}
		return `{"id": "new"}`
	})
}

func TestUpdateAllMovesSeriesByDelta(t *testing.T) {
	calls, done := seriesServer(t)
	defer done()
	// Dragged the Dec 1 occurrence to Wednesday Dec 2, 10:30-12:00.
	_, err := UpdateEventScoped(http.DefaultClient, ScopeAll, Input{CalendarID: "primary", EventID: "m1_20261201T140000Z",
		OnlyTimes: true, Start: "2026-12-02T10:30:00-05:00", End: "2026-12-02T12:00:00-05:00", TimeZone: "America/New_York"})
	if err != nil {
		t.Fatal(err)
	}
	p := (*calls)[len(*calls)-1]
	if p.method != "PATCH" || !strings.HasSuffix(p.path, "/events/m1") {
		t.Fatalf("last call = %s %s", p.method, p.path)
	}
	start := p.body["start"].(map[string]any)["dateTime"].(string)
	end := p.body["end"].(map[string]any)["dateTime"].(string)
	s, _ := time.Parse(time.RFC3339, start)
	e, _ := time.Parse(time.RFC3339, end)
	if !s.Equal(time.Date(2026, 11, 4, 15, 30, 0, 0, time.UTC)) || e.Sub(s) != 90*time.Minute {
		t.Errorf("master moved to %s - %s", start, end)
	}
	if rec := p.body["recurrence"].([]any); rec[0] != "RRULE:FREQ=WEEKLY;BYDAY=WE;COUNT=20" {
		t.Errorf("recurrence = %v", rec)
	}
	if _, has := p.body["summary"]; has {
		t.Error("a drag must not touch the title")
	}
}

func TestUpdateFollowingSplitsSeries(t *testing.T) {
	calls, done := seriesServer(t)
	defer done()
	_, err := UpdateEventScoped(http.DefaultClient, ScopeFollowing, Input{CalendarID: "primary", EventID: "m1_20261201T140000Z",
		Summary: "1:1 (new time)", Start: "2026-12-01T11:00:00-05:00", End: "2026-12-01T11:30:00-05:00",
		TimeZone: "America/New_York", Guests: []string{"ana@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	var truncate, create *fake
	for i := range *calls {
		c := &(*calls)[i]
		if c.method == "PATCH" {
			truncate = c
		}
		if c.method == "POST" {
			create = c
		}
	}
	if truncate == nil || create == nil {
		t.Fatalf("calls = %+v", *calls)
	}
	if rec := truncate.body["recurrence"].([]any); rec[0] != "RRULE:FREQ=WEEKLY;BYDAY=TU;UNTIL=20261201T135959Z" {
		t.Errorf("old series rule = %v", rec)
	}
	if rec := create.body["recurrence"].([]any); rec[0] != "RRULE:FREQ=WEEKLY;BYDAY=TU" {
		t.Errorf("new series rule = %v", rec)
	}
	if create.body["summary"] != "1:1 (new time)" || create.body["start"].(map[string]any)["timeZone"] != "America/New_York" {
		t.Errorf("new series body = %v", create.body)
	}
	att := create.body["attendees"].([]any)
	if len(att) != 2 { // ana plus yourself, kept even though the form only lists other guests
		t.Errorf("attendees = %v", att)
	}
}

func TestDeleteScopes(t *testing.T) {
	calls, done := seriesServer(t)
	defer done()
	if err := DeleteEventScoped(http.DefaultClient, "primary", "m1_20261201T140000Z", ScopeFollowing); err != nil {
		t.Fatal(err)
	}
	last := (*calls)[len(*calls)-1]
	if last.method != "PATCH" || last.body["recurrence"].([]any)[0] != "RRULE:FREQ=WEEKLY;BYDAY=TU;UNTIL=20261201T135959Z" {
		t.Errorf("following delete = %s %v", last.method, last.body)
	}
	*calls = nil
	DeleteEventScoped(http.DefaultClient, "primary", "m1_20261201T140000Z", ScopeAll)
	if last := (*calls)[len(*calls)-1]; last.method != "DELETE" || !strings.HasSuffix(last.path, "/events/m1") {
		t.Errorf("all delete = %s %s", last.method, last.path)
	}
	*calls = nil
	DeleteEventScoped(http.DefaultClient, "primary", "m1_20261201T140000Z", ScopeThis)
	if len(*calls) != 1 || (*calls)[0].method != "DELETE" || !strings.HasSuffix((*calls)[0].path, "/m1_20261201T140000Z") {
		t.Errorf("this delete = %+v", *calls)
	}
}

func TestMoveAfterSave(t *testing.T) {
	calls, done := serve(t, func(r *http.Request) string {
		if r.Method == "GET" {
			return `{"id": "e1", "start": {"dateTime": "2026-11-22T10:00:00Z"}, "end": {"dateTime": "2026-11-22T11:00:00Z"}}`
		}
		return `{"id": "e1"}`
	})
	defer done()
	e, err := UpdateEventScoped(http.DefaultClient, ScopeThis, Input{CalendarID: "primary", EventID: "e1", MoveTo: "work@example.com",
		Summary: "x", Start: "2026-11-22T10:00:00Z", End: "2026-11-22T11:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	m := (*calls)[len(*calls)-1]
	if m.method != "POST" || m.path != "/calendars/primary/events/e1/move" || m.query != "destination=work%40example.com" {
		t.Errorf("move = %s %s?%s", m.method, m.path, m.query)
	}
	if e.CalendarID != "work@example.com" {
		t.Errorf("moved event calendar = %s", e.CalendarID)
	}
}
