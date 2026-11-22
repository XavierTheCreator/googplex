package calendar

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Input is an event to create or update. For all-day events Start and End are
// YYYY-MM-DD with End exclusive; otherwise RFC 3339. Guests are email addresses.
type Input struct {
	CalendarID  string   `json:"calendarId"`
	EventID     string   `json:"eventId"` // set for updates
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	AllDay      bool     `json:"allDay"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	TimeZone    string   `json:"timeZone"` // the user's IANA zone, e.g. America/New_York
	Guests      []string `json:"guests"`
	// Recurrence lines (RRULE/EXDATE/RDATE). nil leaves an existing rule alone;
	// an empty list means "does not repeat".
	Recurrence []string `json:"recurrence"`
	OnlyTimes  bool     `json:"onlyTimes"` // a drag: change start and end, nothing else
	MoveTo     string   `json:"moveTo"`    // another calendar to move the event to after saving
}

// Scopes for a repeating event, like Google Calendar's own prompt.
const (
	ScopeThis      = "this"
	ScopeFollowing = "following"
	ScopeAll       = "all"
)

func (in Input) times() (time.Time, time.Time, error) {
	if in.AllDay {
		s, err1 := time.ParseInLocation("2006-01-02", in.Start, time.Local)
		e, err2 := time.ParseInLocation("2006-01-02", in.End, time.Local)
		if err1 != nil || err2 != nil {
			return s, e, errors.New("check the dates")
		}
		return s, e, nil
	}
	s, err1 := time.Parse(time.RFC3339, in.Start)
	e, err2 := time.Parse(time.RFC3339, in.End)
	if err1 != nil || err2 != nil {
		return s, e, errors.New("check the start and end times")
	}
	return s, e, nil
}

func (in Input) validate() error {
	if in.CalendarID == "" {
		return errors.New("pick a calendar")
	}
	s, e, err := in.times()
	if err != nil {
		return err
	}
	if !e.After(s) {
		return errors.New("the event must end after it starts")
	}
	return nil
}

func at(t time.Time, allDay bool, tz string) apiTime {
	if allDay {
		return apiTime{Date: t.In(time.Local).Format("2006-01-02")}
	}
	return apiTime{DateTime: t.Format(time.RFC3339), TimeZone: tz}
}

func instant(t apiTime) time.Time {
	if t.Date != "" {
		d, _ := time.ParseInLocation("2006-01-02", t.Date, time.Local)
		return d
	}
	s, _ := when(t)
	d, _ := time.Parse(time.RFC3339, s)
	return d
}

func attendeeList(attendees []Attendee) []map[string]any {
	list := make([]map[string]any, 0, len(attendees))
	for _, a := range attendees {
		m := map[string]any{"email": a.Email}
		if a.ResponseStatus != "" {
			m["responseStatus"] = a.ResponseStatus
		}
		if a.DisplayName != "" {
			m["displayName"] = a.DisplayName
		}
		list = append(list, m)
	}
	return list
}

// details is the non-time part of an event body.
func (in Input) details(attendees []Attendee) map[string]any {
	return map[string]any{
		"summary":     in.Summary,
		"description": in.Description,
		"location":    in.Location,
		"attendees":   attendeeList(attendees),
	}
}

// guests turns typed emails into attendees, keeping the details of any already
// on the event, and always keeping yourself and the organizer.
func guests(emails []string, existing []Attendee) []Attendee {
	old := map[string]Attendee{}
	for _, a := range existing {
		old[strings.ToLower(a.Email)] = a
	}
	var out []Attendee
	seen := map[string]bool{}
	add := func(a Attendee) {
		k := strings.ToLower(a.Email)
		if a.Email == "" || seen[k] {
			return
		}
		seen[k] = true
		out = append(out, a)
	}
	for _, e := range emails {
		e = strings.TrimSpace(e)
		if a, ok := old[strings.ToLower(e)]; ok {
			add(a)
		} else {
			add(Attendee{Email: e})
		}
	}
	if len(out) > 0 {
		for _, a := range existing {
			if a.Self || a.Organizer {
				add(a)
			}
		}
	}
	return out
}

// notify asks Google to email guests about the change, as its own UI does.
func notify(hasGuests bool) string {
	if hasGuests {
		return "?sendUpdates=all"
	}
	return "?sendUpdates=none"
}

// GetEvent reads one event, including a repeating event's recurrence lines.
func GetEvent(client *http.Client, calendarID, eventID string) (Event, error) {
	var a apiEvent
	if err := do(client, "GET", calURL(calendarID, eventID), nil, &a); err != nil {
		return Event{}, err
	}
	return toEvent(calendarID, a), nil
}

// CreateEvent adds an event (repeating if in.Recurrence has an RRULE).
func CreateEvent(client *http.Client, in Input) (Event, error) {
	if err := in.validate(); err != nil {
		return Event{}, err
	}
	s, e, _ := in.times()
	att := guests(in.Guests, nil)
	body := in.details(att)
	body["start"], body["end"] = at(s, in.AllDay, in.TimeZone), at(e, in.AllDay, in.TimeZone)
	if len(in.Recurrence) > 0 {
		body["recurrence"] = in.Recurrence
	}
	var a apiEvent
	if err := do(client, "POST", calURL(in.CalendarID)+notify(len(att) > 0), body, &a); err != nil {
		return Event{}, err
	}
	return toEvent(in.CalendarID, a), nil
}

// UpdateEventScoped saves an edit. For one occurrence of a repeating event,
// scope says which occurrences change. Afterwards it moves the event to
// in.MoveTo when set (a repeating event moves as a whole series).
func UpdateEventScoped(client *http.Client, scope string, in Input) (Event, error) {
	if in.EventID == "" {
		return Event{}, errors.New("missing event id")
	}
	if err := in.validate(); err != nil {
		return Event{}, err
	}
	var cur apiEvent
	if err := do(client, "GET", calURL(in.CalendarID, in.EventID), nil, &cur); err != nil {
		return Event{}, err
	}
	if cur.RecurringEventID == "" {
		scope = ScopeThis
	}

	var saved Event
	var err error
	switch scope {
	case ScopeAll:
		saved, err = updateSeries(client, in, cur)
	case ScopeFollowing:
		saved, err = updateFollowing(client, in, cur)
	default:
		saved, err = patch(client, in.CalendarID, in.EventID, in, cur, instantOf(in), nil)
	}
	if err != nil || in.MoveTo == "" || in.MoveTo == in.CalendarID {
		return saved, err
	}
	target := saved.ID
	if scope != ScopeFollowing && cur.RecurringEventID != "" {
		target = cur.RecurringEventID // a single occurrence can't move; its series does
	}
	return MoveEvent(client, in.CalendarID, target, in.MoveTo)
}

// instantOf returns the new start and end from the input.
func instantOf(in Input) [2]time.Time {
	s, e, _ := in.times()
	return [2]time.Time{s, e}
}

// patch sends the edit to one event (an occurrence, a master or a plain event).
func patch(client *http.Client, calendarID, eventID string, in Input, cur apiEvent, when [2]time.Time, recurrence []string) (Event, error) {
	tz := in.TimeZone
	if tz == "" {
		tz = cur.Start.TimeZone
	}
	body := map[string]any{"start": at(when[0], in.AllDay, tz), "end": at(when[1], in.AllDay, tz)}
	att := cur.Attendees
	if !in.OnlyTimes {
		att = guests(in.Guests, cur.Attendees)
		for k, v := range in.details(att) {
			body[k] = v
		}
	}
	if recurrence != nil {
		body["recurrence"] = recurrence
	}
	var a apiEvent
	err := do(client, "PATCH", calURL(calendarID, eventID)+notify(len(att) > 0 || len(cur.Attendees) > 0), body, &a)
	if err != nil {
		return Event{}, err
	}
	return toEvent(calendarID, a), nil
}

// updateSeries edits every occurrence: the master moves by as much as this
// occurrence was moved, and takes the new length.
func updateSeries(client *http.Client, in Input, occurrence apiEvent) (Event, error) {
	var master apiEvent
	if err := do(client, "GET", calURL(in.CalendarID, occurrence.RecurringEventID), nil, &master); err != nil {
		return Event{}, err
	}
	newStart, newEnd, _ := in.times()
	delta := newStart.Sub(instant(occurrence.OriginalStartTime))
	start := instant(master.Start).Add(delta)
	if in.AllDay { // whole days, so a DST change can't leave it at 23:00
		start = instant(master.Start).AddDate(0, 0, dayShift(instant(occurrence.OriginalStartTime), newStart))
	}
	end := start.Add(newEnd.Sub(newStart))

	recurrence := in.Recurrence
	if recurrence == nil {
		recurrence = master.Recurrence
	}
	if days := dayShift(instant(occurrence.OriginalStartTime), newStart); days != 0 {
		recurrence = shiftDays(recurrence, days)
	}
	return patch(client, in.CalendarID, master.ID, in, master, [2]time.Time{start, end}, recurrence)
}

// dayShift is how many calendar days b is after a, in local time.
func dayShift(a, b time.Time) int {
	da := time.Date(a.In(time.Local).Year(), a.In(time.Local).Month(), a.In(time.Local).Day(), 12, 0, 0, 0, time.UTC)
	db := time.Date(b.In(time.Local).Year(), b.In(time.Local).Month(), b.In(time.Local).Day(), 12, 0, 0, 0, time.UTC)
	return int(db.Sub(da).Hours() / 24)
}

// updateFollowing splits the series: the original stops before this
// occurrence and a new series starts here with the edits.
func updateFollowing(client *http.Client, in Input, occurrence apiEvent) (Event, error) {
	var master apiEvent
	if err := do(client, "GET", calURL(in.CalendarID, occurrence.RecurringEventID), nil, &master); err != nil {
		return Event{}, err
	}
	cut := instant(occurrence.OriginalStartTime)
	if !cut.After(instant(master.Start)) {
		return updateSeries(client, in, occurrence) // the first occurrence: same as all
	}
	masterAllDay := master.Start.Date != ""
	if err := do(client, "PATCH", calURL(in.CalendarID, master.ID)+"?sendUpdates=none",
		map[string]any{"recurrence": endBefore(master.Recurrence, cut, masterAllDay)}, nil); err != nil {
		return Event{}, err
	}

	next := in
	if in.OnlyTimes { // a drag: the new series keeps everything else from the original
		next.Summary, next.Description, next.Location = master.Summary, master.Description, master.Location
		for _, a := range master.Attendees {
			next.Guests = append(next.Guests, a.Email)
		}
	}
	att := guests(next.Guests, master.Attendees)
	s, e, _ := in.times()
	body := next.details(att)
	tz := in.TimeZone
	if tz == "" {
		tz = master.Start.TimeZone
	}
	body["start"], body["end"] = at(s, in.AllDay, tz), at(e, in.AllDay, tz)
	recurrence := in.Recurrence
	if recurrence == nil {
		recurrence = withoutCount(master.Recurrence)
		if days := dayShift(cut, s); days != 0 {
			recurrence = shiftDays(recurrence, days)
		}
	}
	if len(recurrence) > 0 {
		body["recurrence"] = recurrence
	}
	var a apiEvent
	if err := do(client, "POST", calURL(in.CalendarID)+notify(len(att) > 0), body, &a); err != nil {
		return Event{}, err
	}
	return toEvent(in.CalendarID, a), nil
}

// DeleteEventScoped removes an event; for a repeating one, scope says which
// occurrences. Guests are told, as Google's own UI does.
func DeleteEventScoped(client *http.Client, calendarID, eventID, scope string) error {
	if scope == ScopeAll || scope == ScopeFollowing {
		var cur apiEvent
		if err := do(client, "GET", calURL(calendarID, eventID), nil, &cur); err != nil {
			return err
		}
		if cur.RecurringEventID != "" {
			if scope == ScopeAll {
				return do(client, "DELETE", calURL(calendarID, cur.RecurringEventID)+"?sendUpdates=all", nil, nil)
			}
			var master apiEvent
			if err := do(client, "GET", calURL(calendarID, cur.RecurringEventID), nil, &master); err != nil {
				return err
			}
			cut := instant(cur.OriginalStartTime)
			if !cut.After(instant(master.Start)) {
				return do(client, "DELETE", calURL(calendarID, master.ID)+"?sendUpdates=all", nil, nil)
			}
			return do(client, "PATCH", calURL(calendarID, master.ID)+"?sendUpdates=all",
				map[string]any{"recurrence": endBefore(master.Recurrence, cut, master.Start.Date != "")}, nil)
		}
	}
	return do(client, "DELETE", calURL(calendarID, eventID)+"?sendUpdates=all", nil, nil)
}

// MoveEvent moves an event (or a whole repeating series) to another calendar.
func MoveEvent(client *http.Client, calendarID, eventID, destination string) (Event, error) {
	var a apiEvent
	u := calURL(calendarID, eventID, "move") + "?" + url.Values{"destination": {destination}}.Encode()
	if err := do(client, "POST", u, nil, &a); err != nil {
		return Event{}, err
	}
	return toEvent(destination, a), nil
}

// QuickAdd creates an event from text like "Lunch with Sam tomorrow at noon".
func QuickAdd(client *http.Client, calendarID, text string) (Event, error) {
	if strings.TrimSpace(text) == "" {
		return Event{}, errors.New("type what the event is")
	}
	var a apiEvent
	err := do(client, "POST", calURL(calendarID, "quickAdd")+"?"+url.Values{"text": {text}}.Encode(), nil, &a)
	if err != nil {
		return Event{}, err
	}
	return toEvent(calendarID, a), nil
}
