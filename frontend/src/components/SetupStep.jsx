import clsx from 'clsx'
import { Check } from 'lucide-react'

// One numbered step in a setup checklist.
export default function SetupStep({ number, title, done, active, children }) {
  return (
    <li className={clsx('flex gap-4 rounded-lg border p-5', active ? 'border-accent/50 bg-panel' : 'border-border bg-panel/60')}>
      <div
        className={clsx(
          'grid h-7 w-7 shrink-0 place-items-center rounded-full text-[13px] font-semibold',
          done ? 'bg-success/15 text-success' : active ? 'bg-accent text-on-accent' : 'bg-raised text-muted',
        )}
      >
        {done ? <Check size={15} strokeWidth={2.5} /> : number}
      </div>
      <div className="min-w-0 flex-1">
        <h3 className={clsx('font-medium', !active && !done && 'text-muted')}>{title}</h3>
        {(active || done) && <div className="mt-2 space-y-3 text-muted">{children}</div>}
      </div>
    </li>
  )
}
