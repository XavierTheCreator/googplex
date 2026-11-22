import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import FileIcon from './FileIcon.jsx'
import { driveRecent, errorText } from '../lib/api.js'
import { modifiedLabel, ownerOf } from '../lib/files.js'

// The landing list for the Docs and Sheets viewers: recently viewed files of one kind.
export default function RecentFiles({ kind, onOpen }) {
  const [files, setFiles] = useState(null)
  const [error, setError] = useState('')

  useEffect(() => {
    driveRecent(kind).then((f) => setFiles(f || [])).catch((err) => setError(errorText(err)))
  }, [kind])

  if (error) return <p className="selectable px-8 py-6 text-danger">{error}</p>
  if (!files) return <div className="grid h-48 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>
  if (files.length === 0) return <p className="px-8 py-12 text-center text-muted">Nothing opened recently. Open one from Drive.</p>

  return (
    <div className="mx-auto max-w-3xl px-8 py-6">
      <h2 className="mb-3 text-[13px] font-medium uppercase tracking-wide text-muted">Recent</h2>
      <ul className="divide-y divide-border rounded-lg border border-border bg-panel">
        {files.map((f) => (
          <li key={f.id}>
            <button onClick={() => onOpen(f)} className="flex w-full items-center gap-3 px-4 py-2.5 text-left hover:bg-raised/60">
              <FileIcon mimeType={f.mimeType} />
              <span className="min-w-0 flex-1 truncate">{f.name}</span>
              <span className="w-40 truncate text-[13px] text-muted">{ownerOf(f)}</span>
              <span className="w-24 text-right text-[13px] text-muted">{modifiedLabel(f.modifiedTime)}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
