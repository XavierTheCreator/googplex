import clsx from 'clsx'
import { Star } from 'lucide-react'
import { shortDate } from '../lib/format.js'

// One thread in the list. The checkbox selects (shift-click selects a range),
// the star toggles, anywhere else opens. focused is the keyboard cursor (j/k).
export default function ThreadRow({ thread, selected, checked, focused, labelsById, onOpen, onCheck, onStar }) {
  const userLabels = (thread.labelIds || []).map((id) => labelsById[id]).filter((l) => l && l.type === 'user')
  return (
    <div
      data-thread-id={thread.id}
      onClick={() => onOpen(thread.id)}
      className={clsx(
        'relative flex cursor-default items-start gap-2 border-b border-border px-3 py-2.5 transition-colors',
        selected ? 'bg-accent/10' : checked ? 'bg-accent/5' : 'hover:bg-raised/60',
      )}
    >
      {focused && <span className="absolute inset-y-0 left-0 w-0.5 bg-accent" />}
      <input type="checkbox" checked={checked} aria-label="Select"
        onClick={(e) => { e.stopPropagation(); onCheck(thread.id, e.shiftKey) }} onChange={() => {}}
        className="mt-1 shrink-0 accent-[rgb(var(--accent))]" />
      <button aria-label={thread.starred ? 'Unstar' : 'Star'} onClick={(e) => { e.stopPropagation(); onStar(thread) }}
        className={`mt-0.5 shrink-0 ${thread.starred ? 'text-warning' : 'text-muted/50 hover:text-muted'}`}>
        <Star size={15} className={thread.starred ? 'fill-warning' : ''} />
      </button>
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline gap-2">
          {thread.unread && <span className="h-2 w-2 shrink-0 translate-y-[-1px] rounded-full bg-accent" />}
          <span className={clsx('min-w-0 flex-1 truncate', thread.unread ? 'font-semibold text-fg' : 'text-fg/90')}>
            {thread.fromName}
            {thread.messageCount > 1 && <span className="ml-1.5 text-[12px] font-normal text-muted">{thread.messageCount}</span>}
          </span>
          <span className="shrink-0 text-[12px] text-muted">{shortDate(thread.date)}</span>
        </div>
        <div className={clsx('mt-0.5 truncate text-[13px]', thread.unread ? 'font-medium text-fg' : 'text-fg/80')}>
          {thread.subject || '(no subject)'}
        </div>
        <div className="mt-0.5 flex items-center gap-1.5">
          {userLabels.slice(0, 3).map((l) => (
            <span key={l.id} className="shrink-0 rounded px-1.5 text-[11px]"
              style={{ backgroundColor: l.color?.backgroundColor || 'rgb(var(--raised))', color: l.color?.textColor || 'rgb(var(--muted))' }}>
              {l.name.split('/').pop()}
            </span>
          ))}
          <span className="min-w-0 truncate text-[13px] text-muted" dangerouslySetInnerHTML={{ __html: thread.snippet }} />
        </div>
      </div>
    </div>
  )
}
