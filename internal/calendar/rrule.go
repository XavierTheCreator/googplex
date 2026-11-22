package calendar

import (
	"strings"
	"time"
)

// Recurrence lines are what Google stores on a repeating event's master, e.g.
// "RRULE:FREQ=WEEKLY;BYDAY=TU;UNTIL=20270302T235959Z" plus optional EXDATE
// and RDATE lines. These helpers rewrite just the RRULE part.

type rrule struct {
	keys []string // in their original order
	vals map[string]string
}

func parseRRULE(line string) (rrule, bool) {
	body, ok := strings.CutPrefix(line, "RRULE:")
	if !ok {
		return rrule{}, false
	}
	r := rrule{vals: map[string]string{}}
	for _, part := range strings.Split(body, ";") {
		k, v, ok := strings.Cut(part, "=")
		if !ok || k == "" {
			continue
		}
		k = strings.ToUpper(k)
		if _, seen := r.vals[k]; !seen {
			r.keys = append(r.keys, k)
		}
		r.vals[k] = v
	}
	return r, true
}

func (r *rrule) set(k, v string) {
	if _, ok := r.vals[k]; !ok {
		r.keys = append(r.keys, k)
	}
	r.vals[k] = v
}

func (r *rrule) del(k string) {
	if _, ok := r.vals[k]; !ok {
		return
	}
	delete(r.vals, k)
	for i, x := range r.keys {
		if x == k {
			r.keys = append(r.keys[:i], r.keys[i+1:]...)
			break
		}
	}
}

func (r rrule) String() string {
	parts := make([]string, 0, len(r.keys))
	for _, k := range r.keys {
		if v, ok := r.vals[k]; ok {
			parts = append(parts, k+"="+v)
		}
	}
	return "RRULE:" + strings.Join(parts, ";")
}

// editRRULE applies f to every RRULE line and keeps the other lines as they are.
func editRRULE(lines []string, f func(r *rrule)) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if r, ok := parseRRULE(l); ok {
			f(&r)
			l = r.String()
		}
		out = append(out, l)
	}
	return out
}

// endBefore makes the series stop just before cut: COUNT is dropped and UNTIL
// set to one second earlier in UTC, or to the day before for all-day events.
func endBefore(lines []string, cut time.Time, allDay bool) []string {
	until := cut.Add(-time.Second).UTC().Format("20060102T150405Z")
	if allDay {
		until = cut.AddDate(0, 0, -1).Format("20060102")
	}
	return editRRULE(lines, func(r *rrule) {
		r.del("COUNT")
		r.set("UNTIL", until)
	})
}

// withoutCount drops COUNT: a series continued from a split can't know how
// many occurrences were already used. UNTIL is kept, so it still ends on time.
func withoutCount(lines []string) []string {
	return editRRULE(lines, func(r *rrule) { r.del("COUNT") })
}

var weekdays = []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}

// shiftDays moves BYDAY weekdays by n days, so dragging a weekly series from
// Tuesday to Wednesday keeps it on the day it was dropped on.
func shiftDays(lines []string, n int) []string {
	n = ((n % 7) + 7) % 7
	if n == 0 {
		return lines
	}
	return editRRULE(lines, func(r *rrule) {
		v, ok := r.vals["BYDAY"]
		if !ok {
			return
		}
		days := strings.Split(v, ",")
		for i, d := range days {
			prefix, day := d[:len(d)-min(2, len(d))], d[len(d)-min(2, len(d)):]
			for j, w := range weekdays {
				if w == day {
					days[i] = prefix + weekdays[(j+n)%7]
				}
			}
		}
		r.set("BYDAY", strings.Join(days, ","))
	})
}
