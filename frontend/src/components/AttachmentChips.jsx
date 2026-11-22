import { Download, Eye, File, FileImage, HardDriveUpload } from 'lucide-react'
import { formatBytes } from '../lib/mail.js'

// The files attached to a message: preview images and PDFs, download, or
// copy into Drive (when Drive is set up).
export default function AttachmentChips({ attachments, onPreview, onDownload, onSaveToDrive }) {
  const files = (attachments || []).filter((a) => !a.inline)
  if (!files.length) return null
  return (
    <div className="mt-3 flex flex-wrap gap-2">
      {files.map((a) => {
        const previewable = a.mimeType?.startsWith('image/') || a.mimeType === 'application/pdf'
        const Icon = a.mimeType?.startsWith('image/') ? FileImage : File
        return (
          <div key={a.partId} className="flex max-w-xs items-center gap-2 rounded-lg border border-border bg-panel py-1.5 pl-2.5 pr-1 text-[13px]">
            <Icon size={16} className="shrink-0 text-muted" />
            <div className="min-w-0">
              <div className="truncate">{a.name}</div>
              <div className="text-[11px] text-muted">{formatBytes(a.size)}</div>
            </div>
            <div className="ml-1 flex">
              {previewable && <Chip title="Preview" onClick={() => onPreview(a)}><Eye size={14} /></Chip>}
              <Chip title="Download" onClick={() => onDownload(a)}><Download size={14} /></Chip>
              {onSaveToDrive && <Chip title="Save to Drive" onClick={() => onSaveToDrive(a)}><HardDriveUpload size={14} /></Chip>}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function Chip({ title, onClick, children }) {
  return (
    <button title={title} aria-label={title} onClick={onClick} className="grid h-7 w-7 place-items-center rounded text-muted hover:bg-raised hover:text-fg">
      {children}
    </button>
  )
}
