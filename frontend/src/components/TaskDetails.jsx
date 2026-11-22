import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Loader2, Trash2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import Input from './ui/Input.jsx'
import Textarea from './ui/Textarea.jsx'
import ExternalLink from './ExternalLink.jsx'
import { dueShortcuts } from '../lib/tasks.js'

// Notes, due date, which list, and delete, for one task.
export default function TaskDetails({ task, lists, listId, onClose, onSave, onMove, onDelete }) {
  const [title, setTitle] = useState('')
  const [notes, setNotes] = useState('')
  const [due, setDue] = useState('')
  const [dest, setDest] = useState(listId)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!task) return
    setTitle(task.title)
    setNotes(task.notes || '')
    setDue(task.due || '')
    setDest(listId)
  }, [task, listId])

  const save = async () => {
    setBusy(true)
    try {
      const patch = {}
      if (title.trim() && title.trim() !== task.title) patch.title = title.trim()
      if (notes !== (task.notes || '')) patch.notes = notes
      if (due !== (task.due || '')) patch.due = due
      if (Object.keys(patch).length) await onSave(task, patch)
      if (dest !== listId) await onMove(task, dest)
      onClose()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog.Root open={!!task} onOpenChange={(o) => !o && !busy && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        {task && (
          <Dialog.Content
            onKeyDown={(e) => { if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); save() } }}
            className="fixed left-1/2 top-1/2 z-50 w-[min(520px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-panel shadow-2xl focus:outline-none">
            <div className="flex items-center justify-between border-b border-border px-5 py-3">
              <Dialog.Title className="font-semibold">Task details</Dialog.Title>
              <Dialog.Description className="sr-only">Edit the task's notes, due date and list.</Dialog.Description>
              <Dialog.Close className="grid h-7 w-7 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg"><X size={16} /></Dialog.Close>
            </div>
            <div className="space-y-3 px-5 py-4">
              <Input value={title} onChange={(e) => setTitle(e.target.value)} className="h-10 text-[15px]" />
              <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Notes" className="h-28" />
              <div>
                <div className="mb-1.5 text-[13px] text-muted">Due</div>
                <div className="flex flex-wrap items-center gap-2">
                  <Input type="date" value={due} onChange={(e) => setDue(e.target.value)} className="h-8 w-40" />
                  {dueShortcuts().map(([label, date]) => (
                    <Button key={label} size="sm" variant={due === date ? 'primary' : 'secondary'} onClick={() => setDue(date)}>{label}</Button>
                  ))}
                  {due && <Button size="sm" variant="ghost" onClick={() => setDue('')}>No date</Button>}
                </div>
                <p className="mt-1 text-[12px] text-muted">Google Tasks keeps the day only, not a time.</p>
              </div>
              {lists.length > 1 && (
                <label className="flex items-center gap-3 text-[13px]">
                  <span className="text-muted">List</span>
                  <select value={dest} onChange={(e) => setDest(e.target.value)}
                    className="h-8 flex-1 rounded-md border border-border bg-bg px-2 text-fg focus:outline-none focus:ring-2 focus:ring-accent/50">
                    {lists.map((l) => <option key={l.id} value={l.id}>{l.title}</option>)}
                  </select>
                </label>
              )}
            </div>
            <div className="flex items-center justify-between border-t border-border px-5 py-3">
              <div className="flex gap-2">
                <Button variant="ghost" size="sm" onClick={() => { onClose(); onDelete(task) }}><Trash2 size={14} /> Delete</Button>
                {task.webViewLink && <ExternalLink href={task.webViewLink} size="sm">Open in Google</ExternalLink>}
              </div>
              <div className="flex gap-2">
                <Button variant="ghost" onClick={onClose} disabled={busy}>Cancel</Button>
                <Button variant="primary" onClick={save} disabled={busy}>{busy && <Loader2 className="animate-spin" size={14} />} Save</Button>
              </div>
            </div>
          </Dialog.Content>
        )}
      </Dialog.Portal>
    </Dialog.Root>
  )
}
