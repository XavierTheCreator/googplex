import { useEffect, useState } from 'react'
import { FolderOpen, Loader2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import { cancelDownload, errorText, onDownloadProgress, revealFile } from '../lib/api.js'

const isMac = navigator.userAgent.includes('Mac')
const active = (d) => d.status !== 'done' && d.status !== 'error'

const statusText = {
  starting: 'Starting…',
  merging: 'Finishing…',
  done: 'Done',
}

// Bottom-left list of downloads in this session, fed by "download:progress" events.
export default function DownloadTray() {
  const [items, setItems] = useState({}) // video id -> latest progress
  const [error, setError] = useState('')

  useEffect(() => onDownloadProgress((p) => {
    setItems((m) => ({ ...m, [p.videoId]: { ...m[p.videoId], ...p } }))
  }), [])

  const list = Object.values(items).reverse()
  if (list.length === 0) return null

  const dismiss = (id) => setItems((m) => {
    const next = { ...m }
    delete next[id]
    return next
  })

  const reveal = async (path) => {
    setError('')
    try {
      await revealFile(path)
    } catch (err) {
      setError(errorText(err))
    }
  }

  return (
    <div className="fixed bottom-4 left-20 z-30 w-80 overflow-hidden rounded-xl border border-border bg-panel shadow-2xl">
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <span className="text-[13px] font-medium">Downloads</span>
        {list.some((d) => !active(d)) && (
          <button className="text-[12px] text-muted hover:text-fg"
            onClick={() => setItems((m) => Object.fromEntries(Object.entries(m).filter(([, d]) => active(d))))}>
            Clear finished
          </button>
        )}
      </div>
      <ul className="max-h-72 divide-y divide-border overflow-y-auto">
        {list.map((d) => (
          <li key={d.videoId} className="px-3 py-2.5">
            <div className="flex items-start gap-2">
              <span className="min-w-0 flex-1 truncate text-[13px]">{d.title || d.videoId}</span>
              <button title={active(d) ? 'Cancel' : 'Dismiss'} aria-label={active(d) ? 'Cancel' : 'Dismiss'}
                onClick={() => (active(d) ? cancelDownload(d.videoId) : dismiss(d.videoId))}
                className="text-muted hover:text-fg"><X size={14} /></button>
            </div>
            {active(d) && (
              <>
                <div className="mt-1.5 h-1 overflow-hidden rounded-full bg-raised">
                  <div className="h-full bg-accent transition-[width]" style={{ width: `${Math.min(100, d.percent || 0)}%` }} />
                </div>
                <div className="mt-1 flex items-center gap-1.5 text-[12px] text-muted">
                  {(d.status === 'starting' || d.status === 'merging') && <Loader2 className="animate-spin" size={12} />}
                  {statusText[d.status] || `${(d.percent || 0).toFixed(0)}%${d.eta ? ` · ${d.eta} left` : ''}`}
                </div>
              </>
            )}
            {d.status === 'done' && (
              <Button className="mt-1.5" size="sm" variant="ghost" onClick={() => reveal(d.path)} disabled={!d.path}>
                <FolderOpen size={14} /> {isMac ? 'Show in Finder' : 'Show in folder'}
              </Button>
            )}
            {d.status === 'error' && <p className="selectable mt-1 break-words text-[12px] text-danger">{d.error}</p>}
          </li>
        ))}
      </ul>
      {error && <p className="selectable border-t border-border px-3 py-2 text-[12px] text-danger">{error}</p>}
    </div>
  )
}
