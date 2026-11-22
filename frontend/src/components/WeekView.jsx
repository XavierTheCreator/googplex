import { useEffect, useRef, useState } from 'react'
import EventChip from './EventChip.jsx'
import { addDays, layoutDay, onDay, sameDay, timeLabel } from '../lib/calendar.js'

const HOUR = 48 // px per hour
const GUTTER = 56 // px, the hour labels column
const SNAP = 15 // minutes
const hours = Array.from({ length: 24 }, (_, h) => h)
const noDrag = { '--wails-draggable': 'no-drag' }

function hourLabel(h) {
  return new Date(2000, 0, 1, h).toLocaleTimeString([], { hour: 'numeric' })
}

// Where a dragged event would land. mode is move or resize (the bottom edge).
function dropTimes(d) {
  const { event, mode, days, mins } = d
  if (mode === 'resize') {
    return { s: event.s, e: new Date(Math.max(event.e.getTime() + mins * 60000, event.s.getTime() + SNAP * 60000)) }
  }
  const s = new Date(addDays(event.s, days).getTime() + mins * 60000)
  const e = new Date(addDays(event.e, days).getTime() + mins * 60000)
  return { s, e }
}

// Seven day columns on an hour grid, with an all-day row on top. Click an empty
// slot to create; drag an event to move it (across days too) or its bottom
// edge to change when it ends.
export default function WeekView({ days, events, colorOf, canDrag, onSelect, onCreate, onDrop }) {
  const scroller = useRef(null)
  const grid = useRef(null)
  const dragRef = useRef(null)
  const [drag, setDrag] = useState(null)
  const [now, setNow] = useState(new Date())

  useEffect(() => {
    const t = setInterval(() => setNow(new Date()), 60000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    if (scroller.current) scroller.current.scrollTop = 7 * HOUR // start the day at 7am
  }, [])

  const allDay = events.filter((e) => e.allDay || e.e - e.s >= 86400000)
  const timed = events.filter((e) => !allDay.includes(e))

  const createAt = (day, e) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const half = Math.floor(((e.clientY - rect.top) / HOUR) * 2)
    const start = new Date(day)
    start.setHours(0, half * 30, 0, 0)
    onCreate(start)
  }

  // Pointer handlers shared by timed blocks, their resize handles and all-day chips.
  const dragProps = (event, mode, area) => ({
    onPointerDown: (e) => {
      if (e.button !== 0) return
      e.stopPropagation()
      const movable = canDrag(event)
      if (movable) e.currentTarget.setPointerCapture(e.pointerId)
      const width = grid.current.getBoundingClientRect().width - GUTTER
      dragRef.current = { event, mode, area, movable, x0: e.clientX, y0: e.clientY, colW: width / 7, days: 0, mins: 0, moved: false }
    },
    onPointerMove: (e) => {
      const d = dragRef.current
      if (!d || !d.movable) return
      const dx = e.clientX - d.x0
      const dy = e.clientY - d.y0
      if (!d.moved && Math.hypot(dx, dy) < 4) return
      d.moved = true
      d.days = mode === 'resize' ? 0 : Math.round(dx / d.colW)
      d.mins = area === 'allday' ? 0 : Math.round(((dy / HOUR) * 60) / SNAP) * SNAP
      setDrag({ ...d })
    },
    onPointerUp: (e) => {
      e.stopPropagation() // a resize handle sits inside its event block
      const d = dragRef.current
      dragRef.current = null
      setDrag(null)
      if (!d) return
      if (!d.moved) return onSelect(event)
      const { s, e: end } = dropTimes(d)
      if (s.getTime() !== event.s.getTime() || end.getTime() !== event.e.getTime()) onDrop(event, s, end)
    },
    onPointerCancel: () => {
      dragRef.current = null
      setDrag(null)
    },
    onClick: (e) => {
      e.stopPropagation()
      if (e.detail === 0) onSelect(event) // keyboard
    },
  })

  const ghost = drag ? { ...drag.event, ...dropTimes(drag) } : null
  const dragging = (ev) => drag && drag.event.id === ev.id && drag.event.calendarId === ev.calendarId
  const cols = 'grid grid-cols-[56px_repeat(7,minmax(0,1fr))]'

  return (
    <div className="flex h-full min-h-0 select-none flex-col" style={noDrag}>
      <div className={`${cols} border-b border-border`}>
        <div />
        {days.map((d) => {
          const today = sameDay(d, now)
          return (
            <div key={d.toISOString()} className="border-l border-border px-2 py-2 text-center">
              <div className="text-[11px] uppercase text-muted">{d.toLocaleDateString([], { weekday: 'short' })}</div>
              <div className={`mx-auto mt-0.5 grid h-8 w-8 place-items-center rounded-full text-[17px] ${today ? 'bg-accent font-semibold text-on-accent' : ''}`}>
                {d.getDate()}
              </div>
            </div>
          )
        })}
      </div>
      <div className={`${cols} border-b border-border`}>
        <div className="px-1 py-1 text-right text-[10px] text-muted">all day</div>
        {days.map((d) => {
          const target = ghost && drag.area === 'allday' && onDay([ghost], d).length > 0
          return (
            <div key={d.toISOString()} className={`min-h-[28px] space-y-0.5 border-l border-border p-0.5 ${target ? 'bg-accent/15' : ''}`}>
              {onDay(allDay, d).map((ev) => (
                <div key={ev.calendarId + ev.id} className={dragging(ev) ? 'opacity-40' : ''}>
                  <EventChip event={{ ...ev, allDay: true }} color={colorOf(ev)} {...dragProps(ev, 'move', 'allday')} />
                </div>
              ))}
            </div>
          )
        })}
      </div>
      <div ref={scroller} className="min-h-0 flex-1 overflow-y-auto">
        <div ref={grid} className={`${cols} relative`} style={{ height: 24 * HOUR }}>
          <div className="relative">
            {hours.map((h) => (
              <div key={h} className="absolute right-2 -translate-y-1/2 text-[10px] text-muted" style={{ top: h * HOUR }}>
                {h > 0 && hourLabel(h)}
              </div>
            ))}
          </div>
          {days.map((d) => {
            const today = sameDay(d, now)
            const ghostHere = ghost && drag.area === 'timed' && onDay([ghost], d).length > 0 ? layoutDay([ghost], d)[0] : null
            return (
              <div key={d.toISOString()} className="relative cursor-pointer border-l border-border" onClick={(e) => createAt(d, e)}>
                {hours.map((h) => (
                  <div key={h} className="absolute inset-x-0 border-t border-border/60" style={{ top: h * HOUR }} />
                ))}
                {layoutDay(onDay(timed, d), d).map(({ event, top, height, col, cols }) => {
                  const color = colorOf(event)
                  const movable = canDrag(event)
                  return (
                    <button
                      key={event.calendarId + event.id}
                      {...dragProps(event, 'move', 'timed')}
                      className={`absolute overflow-hidden rounded-md border-l-[3px] px-1.5 py-0.5 text-left text-[12px] leading-tight hover:brightness-110 ${movable ? 'cursor-grab active:cursor-grabbing' : ''} ${dragging(event) ? 'opacity-40' : ''}`}
                      style={{
                        top: (top / 60) * HOUR + 1,
                        height: Math.max((height / 60) * HOUR - 2, 18),
                        left: `calc(${(col / cols) * 100}% + 2px)`,
                        width: `calc(${100 / cols}% - 4px)`,
                        borderColor: color,
                        backgroundColor: `${color}33`,
                        touchAction: 'none',
                      }}
                    >
                      <div className="truncate font-medium">{event.summary || '(no title)'}</div>
                      {height >= 40 && <div className="truncate text-muted">{timeLabel(event.s)} – {timeLabel(event.e)}</div>}
                      {movable && (
                        <div {...dragProps(event, 'resize', 'timed')} className="absolute inset-x-0 bottom-0 h-1.5 cursor-ns-resize" />
                      )}
                    </button>
                  )
                })}
                {ghostHere && (
                  <div
                    className="pointer-events-none absolute inset-x-0.5 z-20 rounded-md border-2 border-dashed px-1.5 py-0.5 text-[12px] leading-tight shadow-lg"
                    style={{
                      top: (ghostHere.top / 60) * HOUR + 1,
                      height: Math.max((ghostHere.height / 60) * HOUR - 2, 18),
                      borderColor: colorOf(ghost),
                      backgroundColor: `${colorOf(ghost)}55`,
                    }}
                  >
                    <div className="truncate font-medium">{ghost.summary || '(no title)'}</div>
                    <div className="truncate">{timeLabel(ghost.s)} – {timeLabel(ghost.e)}</div>
                  </div>
                )}
                {today && (
                  <div className="pointer-events-none absolute inset-x-0 z-10" style={{ top: ((now.getHours() * 60 + now.getMinutes()) / 60) * HOUR }}>
                    <div className="absolute -left-1 -top-1 h-2 w-2 rounded-full bg-danger" />
                    <div className="h-0.5 bg-danger" />
                  </div>
                )}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
