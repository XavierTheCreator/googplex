import { useEffect, useRef, useState } from 'react'
import {
  AlertOctagon, Archive, ChevronDown, ChevronUp, ExternalLink as ExternalIcon, Forward, Inbox, Loader2, Mail, MailOpen, Printer,
  Reply, ReplyAll, Star, Trash2, TriangleAlert, Undo2,
} from 'lucide-react'
import EmailFrame from './EmailFrame.jsx'
import Button from './ui/Button.jsx'
import ExternalLink from './ExternalLink.jsx'
import AttachmentChips from './AttachmentChips.jsx'
import LabelPicker from './LabelPicker.jsx'
import PreviewDialog from './PreviewDialog.jsx'
import { errorText, gmailAttachmentUrl, gmailDownloadAttachment, gmailSaveToDrive, openThread, openUrl } from '../lib/api.js'
import { longDate } from '../lib/format.js'
import { gmailWebLinks } from '../lib/mail.js'
import { useTheme } from '../lib/theme.js'

const showHeaders = ['From', 'To', 'Cc', 'Reply-To', 'Date', 'Subject', 'Mailed-By', 'Message-ID']

function MessageItem({ message, open, onToggle, onReply, frameProps, onPreview, onDownload, onSaveToDrive }) {
  const [details, setDetails] = useState(false)
  const [darkEmail, setDarkEmail] = useState(false) // per message, off by default
  const { dark } = useTheme()
  return (
    <article className="border-b border-border pb-5">
      <div className="flex items-baseline gap-3 py-3">
        <button onClick={onToggle} className="flex min-w-0 flex-1 items-baseline gap-3 text-left">
          <span className="font-medium">{message.fromName}</span>
          <span className="min-w-0 flex-1 truncate text-[13px] text-muted">{open ? `to ${message.to}` : ''}</span>
        </button>
        <span className="shrink-0 text-[12px] text-muted">{longDate(message.date)}</span>
        {open && dark && message.html && (
          <label className="flex shrink-0 items-center gap-1.5 text-[12px] text-muted" title="Darken this email (pictures keep their colors)">
            <input type="checkbox" checked={darkEmail} onChange={(e) => setDarkEmail(e.target.checked)} className="accent-[rgb(var(--accent))]" /> Dark email
          </label>
        )}
        {open && (
          <button onClick={() => setDetails((d) => !d)} title="Details" aria-label="Details" className="text-muted hover:text-fg">
            {details ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
          </button>
        )}
      </div>
      {open && details && (
        <div className="mb-3 rounded-md border border-border bg-raised/40 p-3 text-[12px]">
          <table className="w-full">
            <tbody>
              {(message.headers || []).filter((h) => showHeaders.includes(h.name)).map((h, i) => (
                <tr key={i}><td className="w-24 py-0.5 pr-3 align-top text-muted">{h.name}</td><td className="break-all py-0.5">{h.value}</td></tr>
              ))}
            </tbody>
          </table>
          <button onClick={() => openUrl(gmailWebLinks.original(message.id))} className="mt-2 flex items-center gap-1 text-accent hover:underline">
            Show original <ExternalIcon size={11} />
          </button>
        </div>
      )}
      {open && (
        <>
          <EmailFrame html={message.html} text={message.text} darkEmail={dark && darkEmail} {...frameProps} />
          <AttachmentChips attachments={message.attachments} onPreview={onPreview} onDownload={onDownload} onSaveToDrive={onSaveToDrive} />
          <div className="mt-3 flex gap-2">
            <Button size="sm" onClick={() => onReply(message, 'reply')}><Reply size={14} /> Reply</Button>
            <Button size="sm" onClick={() => onReply(message, 'all')}><ReplyAll size={14} /> Reply all</Button>
            <Button size="sm" onClick={() => onReply(message, 'forward')}><Forward size={14} /> Forward</Button>
          </div>
        </>
      )}
    </article>
  )
}

// The open thread with its actions. onAction(kind, labelChange?) runs list
// actions in the page (archive, trash, spam, star, read, label…), so the list
// and this view stay in step; onReply(message, mode) opens compose.
export default function MessageView({ threadId, thread, folder, labels, driveReady, onAction, onReply, onMessages, onCompose, frameProps }) {
  const [messages, setMessages] = useState(null)
  const [openIds, setOpenIds] = useState(new Set())
  const [error, setError] = useState('')
  const [preview, setPreview] = useState(null)
  const [note, setNote] = useState('')
  const reported = useRef(onMessages)
  reported.current = onMessages

  useEffect(() => setMessages(null), [threadId])

  useEffect(() => {
    if (!threadId) return
    setError('')
    openThread(threadId)
      .then((msgs) => {
        setMessages(msgs)
        setOpenIds(new Set([msgs[msgs.length - 1]?.id])) // newest open, older ones collapsed
        reported.current?.(msgs)
      })
      .catch((err) => setError(errorText(err)))
  }, [threadId])

  if (!threadId) {
    return (
      <div className="grid h-full place-items-center text-muted">
        <div className="text-center"><Mail className="mx-auto mb-2" size={22} />Select an email to read it.</div>
      </div>
    )
  }
  if (error) return <p className="selectable p-8 text-danger">{error}</p>
  if (!messages) return <div className="grid h-full place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>

  const toggle = (id) => setOpenIds((prev) => {
    const next = new Set(prev)
    next.has(id) ? next.delete(id) : next.add(id)
    return next
  })
  const allOpen = messages.every((m) => openIds.has(m.id))
  const labelIds = thread?.labelIds || messages.flatMap((m) => m.labelIds || [])
  const inTrash = folder === 'TRASH'
  const inSpam = folder === 'SPAM'

  const act = (label, run) => run().then(() => setNote(label)).catch((err) => setError(errorText(err)))
  const attach = {
    onPreview: (a) => gmailAttachmentUrl(a.messageId, a.partId).then((src) => setPreview({ ...a, src })),
    onDownload: (a) => act('Downloading to your download folder', () => gmailDownloadAttachment(a.messageId, a.partId, a.name)),
    onSaveToDrive: driveReady ? (a) => act('Saving to My Drive', () => gmailSaveToDrive(a.messageId, a.partId, a.name)) : null,
  }

  return (
    <div className="selectable flex min-h-full flex-col">
      <div className="sticky top-0 z-10 flex flex-wrap items-center gap-1 border-b border-border bg-bg/95 px-6 py-2 backdrop-blur">
        {!inTrash && labelIds.includes('INBOX') && <Tool title="Archive (e)" onClick={() => onAction('archive')}><Archive size={15} /></Tool>}
        {!inTrash && !labelIds.includes('INBOX') && !inSpam && <Tool title="Move to Inbox" onClick={() => onAction('inbox')}><Inbox size={15} /></Tool>}
        {inTrash ? <Tool title="Restore from trash" onClick={() => onAction('untrash')}><Undo2 size={15} /></Tool>
          : <Tool title="Move to trash (#)" onClick={() => onAction('trash')}><Trash2 size={15} /></Tool>}
        {inSpam ? <Tool title="Not spam" onClick={() => onAction('notspam')}><Inbox size={15} /></Tool>
          : !inTrash && <Tool title="Report spam" onClick={() => onAction('spam')}><AlertOctagon size={15} /></Tool>}
        <Tool title="Mark as unread (U)" onClick={() => onAction('unread')}><MailOpen size={15} /></Tool>
        <Tool title={labelIds.includes('STARRED') ? 'Unstar (s)' : 'Star (s)'} onClick={() => onAction('star')}>
          <Star size={15} className={labelIds.includes('STARRED') ? 'fill-warning text-warning' : ''} />
        </Tool>
        <Tool title={labelIds.includes('IMPORTANT') ? 'Not important' : 'Mark important'} onClick={() => onAction('important')}>
          <TriangleAlert size={15} className={labelIds.includes('IMPORTANT') ? 'text-warning' : ''} />
        </Tool>
        <LabelPicker compact labels={labels} applied={labelIds}
          onToggle={(l, on) => onAction(on ? 'addLabel' : 'removeLabel', l)}
          onMove={!inTrash && labelIds.includes('INBOX') ? (l) => onAction('move', l) : null} />
        <div className="flex-1" />
        {note && <span className="text-[12px] text-muted">{note}</span>}
        <Tool title="Print (opens Gmail's print view)" onClick={() => openUrl(gmailWebLinks.print(threadId))}><Printer size={15} /></Tool>
        <ExternalLink href={gmailWebLinks.thread(threadId)} size="sm" variant="ghost">Gmail</ExternalLink>
      </div>
      <div className="px-8 py-6">
        <div className="mb-2 flex items-baseline gap-3">
          <h2 className="min-w-0 flex-1 text-[18px] font-semibold">{messages[0]?.subject || '(no subject)'}</h2>
          {messages.length > 1 && (
            <button className="shrink-0 text-[12px] text-muted hover:text-fg"
              onClick={() => setOpenIds(allOpen ? new Set([messages[messages.length - 1].id]) : new Set(messages.map((m) => m.id)))}>
              {allOpen ? 'Collapse all' : 'Expand all'}
            </button>
          )}
        </div>
        {messages.map((m) => (
          <MessageItem key={m.id} message={m} open={openIds.has(m.id)} onToggle={() => toggle(m.id)}
            onReply={onReply} frameProps={frameProps(m)} {...attach} />
        ))}
      </div>
      <PreviewDialog file={preview} onClose={() => setPreview(null)} onDownload={(a) => attach.onDownload(a)} />
    </div>
  )
}

function Tool({ title, onClick, children }) {
  return (
    <button title={title} aria-label={title} onClick={onClick} className="grid h-8 w-8 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg">
      {children}
    </button>
  )
}
