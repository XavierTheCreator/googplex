import { textOn, timeLabel } from '../lib/calendar.js'

// One event in a month cell, the all-day row or the agenda. Extra props (such
// as drag handlers) go to the button; onClick gets (domEvent, event).
export default function EventChip({ event, color, onClick, showTime = true, ...rest }) {
  const filled = event.allDay
  return (
    <button
      {...rest}
      onClick={(e) => { e.stopPropagation(); onClick?.(e, event) }}
      title={event.summary || '(no title)'}
      style={{ touchAction: 'none', ...(filled ? { backgroundColor: color, color: textOn(color) } : {}) }}
      className={`flex w-full min-w-0 items-center gap-1.5 rounded px-1.5 py-0.5 text-left text-[12px] leading-tight ${filled ? 'hover:brightness-110' : 'hover:bg-raised'}`}
    >
      {!filled && <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />}
      {!filled && showTime && <span className="shrink-0 text-muted tabular-nums">{timeLabel(event.s)}</span>}
      <span className={`truncate ${event.status === 'tentative' ? 'italic' : ''}`}>{event.summary || '(no title)'}</span>
    </button>
  )
}
