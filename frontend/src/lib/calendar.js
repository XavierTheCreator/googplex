// Date math and layout for the calendar views. weekStart is 0 (Sunday) or 1 (Monday).

export function startOfDay(d) {
  const x = new Date(d)
  x.setHours(0, 0, 0, 0)
  return x
}

export function addDays(d, n) {
  const x = new Date(d)
  x.setDate(x.getDate() + n)
  return x
}

export const startOfWeek = (d, weekStart = 0) => addDays(startOfDay(d), -((new Date(d).getDay() - weekStart + 7) % 7))

export const sameDay = (a, b) => a.toDateString() === b.toDateString()

// Local YYYY-MM-DD, the format all-day events use.
export function isoDate(d) {
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}

export function parseDate(s) {
  const [y, m, d] = s.split('-').map(Number)
  return new Date(y, m - 1, d)
}

// Adds Date objects: s (start) and e (end, exclusive).
export function withDates(event) {
  const s = event.allDay ? parseDate(event.start) : new Date(event.start)
  const e = event.allDay ? parseDate(event.end) : new Date(event.end)
  return { ...event, s, e }
}

export function rangeFor(view, anchor, weekStart = 0) {
  if (view === 'month') {
    const start = startOfWeek(new Date(anchor.getFullYear(), anchor.getMonth(), 1), weekStart)
    return { start, end: addDays(start, 42) }
  }
  if (view === 'agenda') {
    const start = startOfDay(anchor)
    return { start, end: addDays(start, 14) }
  }
  const start = startOfWeek(anchor, weekStart)
  return { start, end: addDays(start, 7) }
}

export function shift(view, anchor, dir) {
  if (view === 'month') return new Date(anchor.getFullYear(), anchor.getMonth() + dir, 1)
  return addDays(anchor, dir * (view === 'agenda' ? 14 : 7))
}

export function rangeLabel(view, anchor, weekStart = 0) {
  if (view === 'month') return anchor.toLocaleDateString([], { month: 'long', year: 'numeric' })
  const { start, end } = rangeFor(view, anchor, weekStart)
  const last = addDays(end, -1)
  const sameMonth = start.getMonth() === last.getMonth()
  const a = start.toLocaleDateString([], { month: 'short', day: 'numeric' })
  const b = last.toLocaleDateString([], sameMonth ? { day: 'numeric' } : { month: 'short', day: 'numeric' })
  return `${a} – ${b}, ${last.getFullYear()}`
}

// Events overlapping one day.
export function onDay(events, day) {
  const start = startOfDay(day)
  const end = addDays(start, 1)
  return events.filter((ev) => ev.s < end && ev.e > start)
}

// Places a day's timed events side by side where they overlap. Returns
// { event, top, height, col, cols } with top and height in minutes from midnight.
export function layoutDay(events, day) {
  const dayStart = startOfDay(day).getTime()
  const dayEnd = addDays(day, 1).getTime() - 1
  const items = events
    .map((event) => {
      const s = Math.max(event.s.getTime(), dayStart)
      const e = Math.min(Math.max(event.e.getTime(), s + 15 * 60000), dayEnd) // at least 15 minutes tall
      return { event, s, e }
    })
    .sort((a, b) => a.s - b.s || b.e - a.e)

  const out = []
  let cluster = []
  let clusterEnd = 0
  const flush = () => {
    const colEnds = []
    for (const it of cluster) {
      let col = colEnds.findIndex((end) => end <= it.s)
      if (col === -1) {
        col = colEnds.length
        colEnds.push(0)
      }
      colEnds[col] = it.e
      it.col = col
    }
    for (const it of cluster) it.cols = colEnds.length
    out.push(...cluster)
    cluster = []
  }
  for (const it of items) {
    if (cluster.length && it.s >= clusterEnd) flush()
    cluster.push(it)
    clusterEnd = Math.max(clusterEnd, it.e)
  }
  flush()
  return out.map((it) => ({
    event: it.event, col: it.col, cols: it.cols,
    top: (it.s - dayStart) / 60000, height: (it.e - it.s) / 60000,
  }))
}

// Google's event colors (colorId 1-11); otherwise the calendar's own color.
const eventColors = {
  1: '#7986cb', 2: '#33b679', 3: '#8e24aa', 4: '#e67c73', 5: '#f6bf26', 6: '#f4511e',
  7: '#039be5', 8: '#616161', 9: '#3f51b5', 10: '#0b8043', 11: '#d50000',
}

export function colorOf(event, calendarsById) {
  return eventColors[event.colorId] || calendarsById[event.calendarId]?.color || '#3b6df6'
}

export const timeLabel = (d) => d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })

export function whenLabel(ev) {
  if (ev.allDay) {
    const last = addDays(ev.e, -1)
    const day = (d) => d.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })
    return sameDay(ev.s, last) ? `${day(ev.s)} · All day` : `${day(ev.s)} – ${day(last)}`
  }
  const day = ev.s.toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })
  if (sameDay(ev.s, ev.e)) return `${day} · ${timeLabel(ev.s)} – ${timeLabel(ev.e)}`
  return `${day} ${timeLabel(ev.s)} – ${ev.e.toLocaleDateString([], { month: 'short', day: 'numeric' })} ${timeLabel(ev.e)}`
}

// Whether the user may change an event: the calendar must be writable, and
// for an invitation the organizer must allow guests to edit.
export function canEdit(event, calendar) {
  const writable = calendar && (calendar.accessRole === 'owner' || calendar.accessRole === 'writer')
  return !!writable && (event.organizerSelf || event.guestsCanModify || !event.organizer)
}

export const isRepeating = (event) => !!event.recurringEventId

// Black or white text, whichever is more readable on a hex color.
export function textOn(hex) {
  const m = /^#?([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})/i.exec(hex || '')
  if (!m) return '#ffffff'
  const lin = (c) => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4 }
  const L = 0.2126 * lin(parseInt(m[1], 16)) + 0.7152 * lin(parseInt(m[2], 16)) + 0.0722 * lin(parseInt(m[3], 16))
  return (L + 0.05) / 0.05 > 1.05 / (L + 0.05) ? '#111317' : '#ffffff'
}
