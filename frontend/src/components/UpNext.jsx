import { useState } from 'react'
import { GripVertical, ListVideo, SkipForward, X } from 'lucide-react'
import { usePlayer } from './PlayerProvider.jsx'
import { formatDuration } from '../lib/format.js'

// The Up next queue beside the player: drag to reorder, remove, autoplay.
export default function UpNext() {
  const { queue, autoplay, setAutoplay, next, play, removeFromQueue, moveInQueue, clearQueue } = usePlayer()
  const [drag, setDrag] = useState(null) // { from, to }

  const handle = (i) => ({
    onPointerDown: (e) => { e.preventDefault(); e.currentTarget.setPointerCapture(e.pointerId); setDrag({ from: i, to: i }) },
    onPointerMove: (e) => {
      if (!drag) return
      const rows = [...document.querySelectorAll('[data-queue-row]')]
      const to = rows.findIndex((r) => { const b = r.getBoundingClientRect(); return e.clientY >= b.top && e.clientY <= b.bottom })
      if (to >= 0 && to !== drag.to) setDrag({ ...drag, to })
    },
    onPointerUp: () => { if (drag && drag.to !== drag.from) moveInQueue(drag.from, drag.to); setDrag(null) },
    onPointerCancel: () => setDrag(null),
  })

  return (
    <aside className="rounded-lg border border-border bg-panel">
      <div className="flex items-center gap-2 border-b border-border px-3 py-2">
        <ListVideo size={15} className="text-muted" />
        <span className="flex-1 text-[13px] font-medium">Up next {queue.length > 0 && <span className="text-muted">({queue.length})</span>}</span>
        <label className="flex items-center gap-1.5 text-[12px] text-muted" title="Play the next video when one ends">
          <input type="checkbox" checked={autoplay} onChange={(e) => setAutoplay(e.target.checked)} className="accent-[rgb(var(--accent))]" /> Autoplay
        </label>
        {queue.length > 0 && <button onClick={next} title="Next (shift-N)" aria-label="Next" className="text-muted hover:text-fg"><SkipForward size={15} /></button>}
      </div>
      {queue.length === 0 ? (
        <p className="px-3 py-4 text-[12px] text-muted">Use a video's ⋯ menu: Play next or Add to queue. Play all on a playlist fills this too.</p>
      ) : (
        <ul className="max-h-[420px] overflow-y-auto py-1">
          {queue.map((v, i) => (
            <li key={`${v.id}-${i}`} data-queue-row
              className={`group flex items-center gap-2 px-2 py-1.5 ${drag?.to === i && drag.from !== i ? 'border-t-2 border-accent' : ''} ${drag?.from === i ? 'opacity-40' : ''}`}>
              <span {...handle(i)} className="cursor-grab text-muted/40 group-hover:text-muted" style={{ touchAction: 'none' }}><GripVertical size={14} /></span>
              <button onClick={() => { removeFromQueue(i); play(v) }} className="flex min-w-0 flex-1 items-center gap-2 text-left">
                <img src={v.thumbnail} alt="" referrerPolicy="no-referrer" className="aspect-video w-20 shrink-0 rounded object-cover" />
                <span className="min-w-0">
                  <span className="line-clamp-2 text-[12px] font-medium leading-snug">{v.title}</span>
                  <span className="block truncate text-[11px] text-muted">{[v.channelTitle, formatDuration(v.durationSeconds)].filter(Boolean).join(' · ')}</span>
                </span>
              </button>
              <button onClick={() => removeFromQueue(i)} aria-label="Remove" className="text-muted opacity-0 hover:text-fg group-hover:opacity-100"><X size={14} /></button>
            </li>
          ))}
        </ul>
      )}
      {queue.length > 1 && <button onClick={clearQueue} className="w-full border-t border-border py-1.5 text-[12px] text-muted hover:text-fg">Clear queue</button>}
    </aside>
  )
}
