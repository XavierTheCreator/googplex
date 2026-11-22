import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import Button from './ui/Button.jsx'

const options = [
  ['this', 'This event'],
  ['following', 'This and following events'],
  ['all', 'All events'],
]

// "Which events?" for a change to a repeating event, like Google Calendar asks.
// request: { title, action, only?: ['all'] } or null; onChoose(scope) or onCancel().
export default function ScopeDialog({ request, onChoose, onCancel }) {
  const allowed = request?.only || options.map(([id]) => id)
  const [scope, setScope] = useState('this')

  useEffect(() => {
    if (request) setScope(allowed[0])
  }, [request]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Dialog.Root open={!!request} onOpenChange={(o) => !o && onCancel()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-[60] bg-black/40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-[70] w-[min(360px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-panel p-5 shadow-2xl focus:outline-none">
          <Dialog.Title className="font-semibold">{request?.title || 'Change repeating event'}</Dialog.Title>
          <Dialog.Description className="sr-only">Choose which occurrences this applies to.</Dialog.Description>
          <div className="mt-4 space-y-2" role="radiogroup">
            {options.filter(([id]) => allowed.includes(id)).map(([id, label]) => (
              <label key={id} className="flex cursor-pointer items-center gap-2.5 text-[14px]">
                <input type="radio" name="scope" checked={scope === id} onChange={() => setScope(id)} className="accent-[rgb(var(--accent))]" />
                {label}
              </label>
            ))}
          </div>
          {request?.note && <p className="mt-3 text-[12px] text-muted">{request.note}</p>}
          <div className="mt-5 flex justify-end gap-2">
            <Button variant="ghost" onClick={onCancel}>Cancel</Button>
            <Button variant={request?.action === 'Delete' ? 'danger' : 'primary'} onClick={() => onChoose(scope)}>
              {request?.action || 'OK'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
