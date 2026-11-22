import { useEffect, useRef, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { File, Loader2, Paperclip, Send, Trash2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import Input from './ui/Input.jsx'
import AddressInput from './AddressInput.jsx'
import RichEditor from './RichEditor.jsx'
import UploadDropzone from './UploadDropzone.jsx'
import { errorText, gmailContacts, gmailDeleteDraft, gmailFileInfo, gmailPickFiles, gmailSaveDraft, gmailSignature, sendEmail } from '../lib/api.js'
import { formatBytes, MAX_ATTACH, sanitizeForEditor, textToHtml } from '../lib/mail.js'

const AUTOSAVE_MS = 3000

// Plain text of editor HTML, for the message's text/plain part.
function htmlText(html) {
  const doc = new DOMParser().parseFromString(html.replace(/<br\s*\/?>/gi, '\n').replace(/<\/(p|div|li|blockquote)>/gi, '$&\n'), 'text/html')
  return (doc.body.textContent || '').replace(/\n{3,}/g, '\n\n')
}

// The first HTML in the editor: the draft's own content, or the signature
// above any quoted reply. Everything goes through sanitizeForEditor because
// the editor sits in the app's own page.
function startingHTML(initial, signature) {
  if (initial?.draftId) return sanitizeForEditor(initial.html || textToHtml(initial.body))
  const sig = signature ? `<br><br><div>--<br>${sanitizeForEditor(signature)}</div>` : ''
  const quoted = initial?.html ? sanitizeForEditor(initial.html) : textToHtml(initial?.body || '')
  return (sig || '<br>') + quoted
}

// New email, reply, reply all, forward, or a saved draft (initial.draftId).
// While open it saves itself to Gmail's drafts a few seconds after each change.
export default function ComposeDialog({ open, initial, onClose, onSent }) {
  const [f, setF] = useState({})
  const [editorHTML, setEditorHTML] = useState('')
  const [contacts, setContacts] = useState([])
  const [showCc, setShowCc] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [saveState, setSaveState] = useState('') // '', saving, saved, failed
  const dirty = useRef(false)
  const saving = useRef(false)
  const latest = useRef(f)
  latest.current = f

  useEffect(() => {
    if (!open) return
    const base = { to: '', cc: '', bcc: '', subject: '', body: '', html: '', threadId: '', inReplyTo: '', references: '', draftId: '', files: [], forwarded: [], ...initial }
    base.files = base.files || []
    base.forwarded = base.forwarded || []
    setF(base)
    setShowCc(!!(base.cc || base.bcc))
    setError('')
    setSaveState(base.draftId ? 'saved' : '')
    dirty.current = false
    gmailContacts().then((c) => setContacts(c || []))
    // The editor's first content is also the message until the user types.
    const begin = (html) => {
      setEditorHTML(html)
      setF((x) => ({ ...x, html, body: htmlText(html) }))
    }
    if (base.draftId) begin(startingHTML(base, ''))
    else gmailSignature().then((s) => begin(startingHTML(base, s?.html))).catch(() => begin(startingHTML(base, '')))
  }, [open, initial])

  const set = (patch) => {
    dirty.current = true
    setF((x) => ({ ...x, ...patch }))
  }

  const payload = (x) => ({
    draftId: x.draftId || '', to: x.to, cc: x.cc, bcc: x.bcc, subject: x.subject, body: x.body, html: x.html,
    threadId: x.threadId || '', inReplyTo: x.inReplyTo || '', references: x.references || '',
    files: (x.files || []).map((l) => l.path), forwarded: x.forwarded || [],
  })
  const isEmpty = (x) => !(x.to || x.cc || x.bcc || x.subject || (x.body || '').trim() || x.files?.length)

  const save = async () => {
    const x = latest.current
    if (saving.current || isEmpty(x) || !dirty.current) return
    saving.current = true
    dirty.current = false
    setSaveState('saving')
    try {
      const r = await gmailSaveDraft(payload(x))
      setF((cur) => ({ ...cur, draftId: r.draftId || cur.draftId, threadId: cur.threadId || r.threadId }))
      setSaveState('saved')
    } catch {
      dirty.current = true
      setSaveState('failed')
    } finally {
      saving.current = false
    }
  }

  // Autosave: a few seconds after the last change.
  useEffect(() => {
    if (!open || !dirty.current) return
    const t = setTimeout(save, AUTOSAVE_MS)
    return () => clearTimeout(t)
  }, [f, open]) // eslint-disable-line react-hooks/exhaustive-deps

  const total = (f.files || []).reduce((n, x) => n + x.size, 0) + (f.forwarded || []).reduce((n, x) => n + (x.size || 0), 0)
  const tooBig = total > MAX_ATTACH

  const send = async () => {
    if (sending || tooBig) return
    setSending(true)
    setError('')
    try {
      await sendEmail(payload(latest.current))
      dirty.current = false
      onSent?.()
      onClose()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setSending(false)
    }
  }

  // Closing keeps the draft (saved now if needed); Discard deletes it.
  const close = async () => {
    if (dirty.current && !isEmpty(latest.current)) await save()
    onClose()
  }
  const discard = async () => {
    dirty.current = false
    if (latest.current.draftId) await gmailDeleteDraft(latest.current.draftId).catch(() => {})
    onClose()
  }

  const addFiles = (list) => {
    if (!list?.length) return
    set({ files: [...(latest.current.files || []), ...list.filter((n) => !(latest.current.files || []).some((o) => o.path === n.path))] })
  }

  const onKeyDown = (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      send()
    }
  }

  const title = f.draftId && !f.inReplyTo && !f.threadId ? 'Draft' : f.inReplyTo ? 'Reply' : /^fwd?:/i.test(f.subject || '') ? 'Forward' : 'New email'

  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && !sending && close()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content onKeyDown={onKeyDown}
          className="fixed left-1/2 top-1/2 z-50 flex h-[min(760px,92vh)] w-[min(820px,94vw)] -translate-x-1/2 -translate-y-1/2 flex-col rounded-xl border border-border bg-panel shadow-2xl focus:outline-none">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <Dialog.Title className="font-semibold">{title}</Dialog.Title>
            <Dialog.Description className="sr-only">Write an email and send it from your Gmail account.</Dialog.Description>
            <div className="flex items-center gap-3">
              <span className="text-[12px] text-muted">
                {saveState === 'saving' ? 'Saving draft…' : saveState === 'saved' ? 'Draft saved' : saveState === 'failed' ? 'Draft not saved' : ''}
              </span>
              <button onClick={close} aria-label="Close" disabled={sending} className="grid h-7 w-7 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg"><X size={16} /></button>
            </div>
          </div>
          <UploadDropzone onDrop={(paths) => gmailFileInfo(paths).then(addFiles)} className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto px-5 py-4">
            <Field label="To">
              <AddressInput value={f.to} onChange={(to) => set({ to })} contacts={contacts} placeholder="name@example.com" autoFocus={!f.inReplyTo} />
              {!showCc && <button onClick={() => setShowCc(true)} className="shrink-0 self-center text-[12px] text-muted hover:text-fg">Cc Bcc</button>}
            </Field>
            {showCc && <Field label="Cc"><AddressInput value={f.cc} onChange={(cc) => set({ cc })} contacts={contacts} /></Field>}
            {showCc && <Field label="Bcc"><AddressInput value={f.bcc} onChange={(bcc) => set({ bcc })} contacts={contacts} /></Field>}
            <Field label="Subject"><Input value={f.subject || ''} onChange={(e) => set({ subject: e.target.value })} /></Field>
            <RichEditor initialHTML={editorHTML} onChange={({ html, text }) => set({ html, body: text })} className="min-h-[220px] flex-1" />
            {((f.files || []).length > 0 || (f.forwarded || []).length > 0) && (
              <div className="flex flex-wrap gap-2">
                {[...(f.forwarded || []).map((a) => ({ key: `fw-${a.partId}`, name: a.name, size: a.size, remove: () => set({ forwarded: f.forwarded.filter((x) => x !== a) }) })),
                  ...(f.files || []).map((l) => ({ key: l.path, name: l.name, size: l.size, remove: () => set({ files: f.files.filter((x) => x !== l) }) }))]
                  .map((a) => (
                    <span key={a.key} className="flex items-center gap-2 rounded-lg border border-border bg-bg py-1 pl-2 pr-1 text-[13px]">
                      <File size={14} className="text-muted" /> <span className="max-w-48 truncate">{a.name}</span>
                      <span className="text-[11px] text-muted">{formatBytes(a.size)}</span>
                      <button aria-label={`Remove ${a.name}`} onClick={a.remove} className="grid h-5 w-5 place-items-center rounded text-muted hover:bg-raised hover:text-fg"><X size={12} /></button>
                    </span>
                  ))}
              </div>
            )}
            {tooBig && <p className="text-[13px] text-danger">Attachments add up to {formatBytes(total)}, over Gmail's 25 MB limit. Remove some, or share them from Drive.</p>}
            {error && <p className="selectable text-[13px] text-danger">{error}</p>}
          </UploadDropzone>
          <div className="flex items-center justify-between border-t border-border px-5 py-3">
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="sm" onClick={() => gmailPickFiles().then(addFiles).catch((err) => setError(errorText(err)))}>
                <Paperclip size={14} /> Attach
              </Button>
              <span className="text-[12px] text-muted">or drop files here · ⌘/Ctrl + Enter to send</span>
            </div>
            <div className="flex gap-2">
              <Button variant="ghost" onClick={discard} disabled={sending} title="Delete this draft"><Trash2 size={14} /> Discard</Button>
              <Button variant="primary" onClick={send} disabled={sending || tooBig || !(f.to || f.cc || f.bcc)}>
                {sending ? <Loader2 className="animate-spin" size={14} /> : <Send size={14} />} Send
              </Button>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function Field({ label, children }) {
  return (
    <div className="flex items-start gap-3">
      <span className="w-14 shrink-0 pt-2 text-[13px] text-muted">{label}</span>
      {children}
    </div>
  )
}
