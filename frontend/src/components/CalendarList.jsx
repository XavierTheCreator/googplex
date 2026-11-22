import { Check } from 'lucide-react'
import { textOn } from '../lib/calendar.js'

// Checkboxes for which calendars are shown, in each calendar's color.
export default function CalendarList({ calendars, onToggle }) {
  return (
    <div>
      <h3 className="mb-2 px-1 text-[12px] font-medium uppercase tracking-wide text-muted">Calendars</h3>
      <ul className="space-y-0.5">
        {calendars.map((c) => (
          <li key={c.id}>
            <label className="flex cursor-pointer items-center gap-2.5 rounded-md px-1 py-1.5 hover:bg-raised">
              <input type="checkbox" className="sr-only" checked={c.visible} onChange={(e) => onToggle(c, e.target.checked)} />
              <span
                className="grid h-4 w-4 shrink-0 place-items-center rounded-[4px] border-2"
                style={{ borderColor: c.color, backgroundColor: c.visible ? c.color : 'transparent' }}
              >
                {c.visible && <Check size={11} strokeWidth={3} style={{ color: textOn(c.color) }} />}
              </span>
              <span className="truncate text-[13px]">{c.summary}</span>
            </label>
          </li>
        ))}
      </ul>
    </div>
  )
}
