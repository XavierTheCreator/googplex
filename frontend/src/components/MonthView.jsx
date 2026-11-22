import { useRef, useState } from 'react'
import EventChip from './EventChip.jsx'
import { addDays, isoDate, onDay, sameDay } from '../lib/calendar.js'

const MAX = 3
const noDrag = { '--wails-draggable': 'no-drag' }

const dayAt = (x, y) => document.elementFromPoint(x, y)?.closest('[data-day]')?.dataset.day

// Six weeks of days with up to three events each. "N more" opens that week.
// Drag an event to another day to move it; its time of day stays the same.
export default function MonthView({ days, month, events, colorOf, canDrag, onSelect, onCreate, onOpenDay, onDrop }) {
  const today = new Date()
  const dragRef = useRef(null)
  const [drag, setDrag] = useState(null) // { event, from, over } while dragging

  const dragProps = (event, day) => ({
    onPointerDown: (e) => {
      if (e.button !== 0) return
      e.stopPropagation()
      const movable = canDrag(event)
      if (movable) e.currentTarget.setPointerCapture(e.pointerId)
      dragRef.current = { event, movable, from: isoDate(day), over: isoDate(day), x0: e.clientX, y0: e.clientY, moved: false }
    },
    onPointerMove: (e) => {
      const d = dragRef.current
      if (!d || !d.movable) return
      if (!d.moved && Math.hypot(e.clientX - d.x0, e.clientY - d.y0) < 4) return
      d.moved = true
      d.over = dayAt(e.clientX, e.clientY) || d.over
      setDrag({ ...d })
    },
    onPointerUp: (e) => {
      e.stopPropagation()
      const d = dragRef.current
      dragRef.current = null
      setDrag(null)
      if (!d) return
      if (!d.moved) return onSelect(event)
      const [fy, fm, fd] = d.from.split('-').map(Number)
      const [oy, om, od] = d.over.split('-').map(Number)
      const shift = Math.round((new Date(oy, om - 1, od) - new Date(fy, fm - 1, fd)) / 86400000)
      if (shift !== 0) onDrop(event, addDays(event.s, shift), addDays(event.e, shift))
    },
    onPointerCancel: () => {
      dragRef.current = null
      setDrag(null)
    },
    onClick: (e) => { if (e.detail === 0) onSelect(event) }, // keyboard
  })

  return (
    <div className="flex h-full min-h-0 select-none flex-col" style={noDrag}>
      <div className="grid grid-cols-7 border-b border-border">
        {days.slice(0, 7).map((d) => (
          <div key={d.toISOString()} className="px-2 py-1.5 text-[11px] uppercase text-muted">
            {d.toLocaleDateString([], { weekday: 'short' })}
          </div>
        ))}
      </div>
      <div className="grid min-h-0 flex-1 grid-cols-7 grid-rows-6">
        {days.map((d) => {
          const list = onDay(events, d).sort((a, b) => b.allDay - a.allDay || a.s - b.s)
          const extra = list.length - MAX
          const target = drag && drag.over === isoDate(d)
          return (
            <div
              key={d.toISOString()}
              data-day={isoDate(d)}
              onClick={() => { const s = new Date(d); s.setHours(9); onCreate(s) }}
              className={`min-h-0 cursor-pointer overflow-hidden border-b border-r border-border p-1 ${target ? 'bg-accent/15 ring-2 ring-inset ring-accent/60' : d.getMonth() === month ? '' : 'bg-raised/40'}`}
            >
              <div className="mb-0.5 flex justify-center">
                <span className={`grid h-6 min-w-6 place-items-center rounded-full px-1 text-[12px] ${sameDay(d, today) ? 'bg-accent font-semibold text-on-accent' : d.getMonth() === month ? '' : 'text-muted'}`}>
                  {d.getDate()}
                </span>
              </div>
              <div className="space-y-0.5">
                {list.slice(0, extra > 0 ? MAX - 1 : MAX).map((ev) => (
                  <div key={ev.calendarId + ev.id}
                    className={drag && drag.event.id === ev.id && drag.event.calendarId === ev.calendarId ? 'opacity-40' : ''}>
                    <EventChip event={ev} color={colorOf(ev)} {...dragProps(ev, d)} />
                  </div>
                ))}
                {extra > 0 && (
                  <button onClick={(e) => { e.stopPropagation(); onOpenDay(d) }}
                    className="w-full rounded px-1.5 text-left text-[12px] font-medium text-muted hover:bg-raised hover:text-fg">
                    {extra + 1} more
                  </button>
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
