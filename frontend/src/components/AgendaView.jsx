import { addDays, onDay, sameDay, timeLabel } from '../lib/calendar.js'

// The next 14 days as a list, skipping empty days.
export default function AgendaView({ start, events, colorOf, onSelect }) {
  const days = Array.from({ length: 14 }, (_, i) => addDays(start, i))
  const today = new Date()
  const groups = days.map((d) => ({ d, list: onDay(events, d).sort((a, b) => b.allDay - a.allDay || a.s - b.s) }))
    .filter((g) => g.list.length > 0)

  if (groups.length === 0) return <p className="py-16 text-center text-muted">Nothing scheduled in the next two weeks.</p>

  return (
    <div className="h-full overflow-y-auto px-6 py-4">
      {groups.map(({ d, list }) => (
        <section key={d.toISOString()} className="flex gap-6 border-b border-border py-3">
          <div className="w-28 shrink-0">
            <div className={`text-[22px] font-semibold leading-none ${sameDay(d, today) ? 'text-accent' : ''}`}>{d.getDate()}</div>
            <div className="mt-1 text-[12px] uppercase text-muted">{d.toLocaleDateString([], { month: 'short', weekday: 'short' })}</div>
          </div>
          <ul className="min-w-0 flex-1 space-y-1">
            {list.map((ev) => (
              <li key={ev.calendarId + ev.id}>
                <button onClick={() => onSelect(ev)} className="flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left hover:bg-raised">
                  <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: colorOf(ev) }} />
                  <span className="w-36 shrink-0 text-[13px] text-muted tabular-nums">
                    {ev.allDay || ev.e - ev.s >= 86400000 ? 'All day' : `${timeLabel(ev.s)} – ${timeLabel(ev.e)}`}
                  </span>
                  <span className="truncate">{ev.summary || '(no title)'}</span>
                  {ev.location && <span className="hidden truncate text-[13px] text-muted lg:inline">· {ev.location}</span>}
                </button>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}
