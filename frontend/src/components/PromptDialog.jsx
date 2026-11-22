import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Loader2 } from 'lucide-react'
import Button from './ui/Button.jsx'
import Input from './ui/Input.jsx'
import { errorText } from '../lib/api.js'

// A small dialog: either a text prompt (input set) or a confirmation.
// request: { title, message?, input?, action, danger?, run(value) } or null.
export default function PromptDialog({ request, onClose }) {
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setValue(request?.input ?? '')
    setError('')
  }, [request])

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await request.run(value)
      onClose()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog.Root open={!!request} onOpenChange={(o) => !o && !busy && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[min(420px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-panel p-5 shadow-2xl focus:outline-none">
          <Dialog.Title className="font-semibold">{request?.title}</Dialog.Title>
          <Dialog.Description className={request?.message ? 'mt-1.5 text-[13px] text-muted' : 'sr-only'}>
            {request?.message || request?.title}
          </Dialog.Description>
          {request?.input !== undefined && (
            <Input className="mt-4" value={value} autoFocus onChange={(e) => setValue(e.target.value)}
              onFocus={(e) => e.target.select()} onKeyDown={(e) => e.key === 'Enter' && submit()} />
          )}
          {error && <p className="selectable mt-3 text-[13px] text-danger">{error}</p>}
          <div className="mt-5 flex justify-end gap-2">
            <Button variant="ghost" onClick={onClose} disabled={busy}>Cancel</Button>
            <Button variant={request?.danger ? 'danger' : 'primary'} onClick={submit} disabled={busy}>
              {busy && <Loader2 className="animate-spin" size={14} />} {request?.action}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
