import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Download, Loader2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import ExternalLink from './ExternalLink.jsx'
import { drivePreviewUrl } from '../lib/api.js'
import { fileKind } from '../lib/files.js'

// Quick look at an image or PDF through the app's loopback server: a Drive file
// (by id), or anything else with its own src (a Gmail attachment).
export default function PreviewDialog({ file, onClose, onDownload }) {
  const [src, setSrc] = useState('')
  const [loaded, setLoaded] = useState(false)

  useEffect(() => {
    setSrc('')
    setLoaded(false)
    if (file) (file.src ? Promise.resolve(file.src) : drivePreviewUrl(file.id)).then(setSrc)
  }, [file])

  const kind = file ? fileKind(file.mimeType) : ''

  return (
    <Dialog.Root open={!!file} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/70" />
        <Dialog.Content className="fixed inset-6 z-50 flex flex-col overflow-hidden rounded-xl border border-border bg-panel shadow-2xl focus:outline-none">
          <div className="flex items-center gap-3 border-b border-border px-4 py-2.5">
            <Dialog.Title className="min-w-0 flex-1 truncate font-medium">{file?.name}</Dialog.Title>
            <Dialog.Description className="sr-only">Preview</Dialog.Description>
            <Button size="sm" onClick={() => onDownload(file)}><Download size={14} /> Download</Button>
            {file?.webViewLink && <ExternalLink href={file.webViewLink} size="sm">Open in browser</ExternalLink>}
            <Dialog.Close className="grid h-7 w-7 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg"><X size={16} /></Dialog.Close>
          </div>
          <div className="relative grid min-h-0 flex-1 place-items-center bg-raised">
            {!loaded && <Loader2 className="absolute animate-spin text-muted" size={20} />}
            {src && kind === 'image' && (
              <img src={src} alt="" onLoad={() => setLoaded(true)} className="max-h-full max-w-full object-contain" />
            )}
            {src && kind === 'pdf' && (
              <iframe src={src} title="PDF preview" onLoad={() => setLoaded(true)} className="h-full w-full border-0 bg-white" />
            )}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
