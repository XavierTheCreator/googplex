import { useEffect, useState } from 'react'
import { CalendarDays, Video } from 'lucide-react'
import Card from './ui/Card.jsx'
import Button from './ui/Button.jsx'
import { getCalendars, getEvents, openUrl, refreshEvents } from '../lib/api.js'
import { addDays, colorOf, startOfDay, timeLabel, withDates } from '../lib/calendar.js'

// Home card: what's left on the calendar today. Hidden until Calendar is set up.
export default function TodayEvents({ onOpen }) {
  const [events, setEvents] = useState(null)
  const [byId, setById] = useState({})

  useEffect(() => {
    const start = startOfDay(new Date())
    const end = addDays(start, 1)
    let stale = false
    const show = (list) => !stale && setEvents((list || []).map(withDates))
    getCalendars()
      .then((cals) => {
        if (stale) return
        setById(Object.fromEntries(cals.map((c) => [c.id, c])))
        getEvents(start, end).then(show)
        return refreshEvents(start, end).then((r) => show(r.events))
      })
      .catch(() => !stale && setEvents(null)) // not connected yet: the service card covers setup
    return () => { stale = true }
  }, [])

  if (!events) return null
  const now = new Date()
  const upcoming = events.filter((e) => e.e > now).sort((a, b) => b.allDay - a.allDay || a.s - b.s).slice(0, 6)

  return (
    <Card className="p-5">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-medium"><CalendarDays size={16} className="text-muted" /> Today</h2>
        <button onClick={onOpen} className="text-[13px] text-muted hover:text-fg">Open calendar</button>
      </div>
      {upcoming.length === 0 ? (
        <p className="text-[13px] text-muted">Nothing else on your calendar today.</p>
      ) : (
        <ul className="space-y-1.5">
          {upcoming.map((ev) => (
            <li key={ev.calendarId + ev.id} className="flex items-center gap-3">
              <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: colorOf(ev, byId) }} />
              <span className="w-32 shrink-0 text-[13px] text-muted tabular-nums">
                {ev.allDay ? 'All day' : `${timeLabel(ev.s)} – ${timeLabel(ev.e)}`}
              </span>
              <span className="min-w-0 flex-1 truncate">{ev.summary || '(no title)'}</span>
              {ev.meetLink && ev.s - now < 15 * 60000 && (
                <Button size="sm" variant="primary" onClick={() => openUrl(ev.meetLink)}><Video size={14} /> Join</Button>
              )}
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}
