import { useEffect, useState } from 'react'
import { ArrowDown, ArrowUp, FolderOpen, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import { driveCancel, errorText, onDriveProgress, revealFile } from '../lib/api.js'

const isMac = navigator.userAgent.includes('Mac')
const running = (t) => t.status === 'running'

// Drive uploads, downloads and exports in this session, bottom-left like the
// video download tray. onUploaded is called when an upload finishes.
export default function TransferTray({ onUploaded }) {
  const [items, setItems] = useState({})
  const [error, setError] = useState('')

  useEffect(() => onDriveProgress((p) => {
    setItems((m) => ({ ...m, [p.id]: { ...m[p.id], ...p } }))
    if (p.kind === 'upload' && p.status === 'done') onUploaded?.()
  }), [onUploaded])

  const list = Object.values(items).reverse()
  if (list.length === 0) return null

  const dismiss = (id) => setItems((m) => {
    const next = { ...m }
    delete next[id]
    return next
  })

  return (
    <div className="fixed bottom-4 left-20 z-30 w-80 overflow-hidden rounded-xl border border-border bg-panel shadow-2xl">
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-[13px] font-medium">Drive transfers</span>
        {list.some((t) => !running(t)) && (
          <button className="text-[12px] text-muted hover:text-fg"
            onClick={() => setItems((m) => Object.fromEntries(Object.entries(m).filter(([, t]) => running(t))))}>
            Clear finished
          </button>
        )}
      </div>
      <ul className="max-h-72 divide-y divide-border overflow-y-auto">
        {list.map((t) => (
          <li key={t.id} className="px-3 py-2.5">
            <div className="flex items-center gap-2">
              {t.kind === 'upload' ? <ArrowUp size={13} className="shrink-0 text-muted" /> : <ArrowDown size={13} className="shrink-0 text-muted" />}
              <span className="min-w-0 flex-1 truncate text-[13px]">{t.name}</span>
              <button aria-label={running(t) ? 'Cancel' : 'Dismiss'} title={running(t) ? 'Cancel' : 'Dismiss'}
                onClick={() => (running(t) ? driveCancel(t.id) : dismiss(t.id))} className="text-muted hover:text-fg"><X size={14} /></button>
            </div>
            {running(t) && (
              <>
                <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-raised">
                  <div className="h-full bg-accent transition-[width]" style={{ width: `${Math.min(100, t.percent || 0)}%` }} />
                </div>
                <div className="mt-1 text-[12px] text-muted">{(t.percent || 0).toFixed(0)}%</div>
              </>
            )}
            {t.status === 'done' && t.kind !== 'upload' && t.path && (
              <Button className="mt-1.5" size="sm" variant="ghost"
                onClick={() => revealFile(t.path).catch((err) => setError(errorText(err)))}>
                <FolderOpen size={14} /> {isMac ? 'Show in Finder' : 'Show in folder'}
              </Button>
            )}
            {t.status === 'done' && t.kind === 'upload' && <p className="mt-1 text-[12px] text-success">Uploaded</p>}
            {t.status === 'error' && <p className="selectable mt-1 break-words text-[12px] text-danger">{t.error}</p>}
          </li>
        ))}
      </ul>
      {error && <p className="selectable border-t border-border px-3 py-2 text-[12px] text-danger">{error}</p>}
    </div>
  )
}
