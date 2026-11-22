import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { CalendarDays, Clock, Loader2, MapPin, Pencil, Repeat, Trash2, Users, Video, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import ExternalLink from './ExternalLink.jsx'
import ScopeDialog from './ScopeDialog.jsx'
import { deleteEvent, errorText, getEvent, openUrl } from '../lib/api.js'
import { canEdit, isRepeating, whenLabel } from '../lib/calendar.js'
import { describe, parseRecurrence } from '../lib/rrule.js'

const replies = {
  accepted: ['Going', 'text-success'],
  declined: ['Not going', 'text-danger'],
  tentative: ['Maybe', 'text-warning'],
  needsAction: ['Awaiting reply', 'text-muted'],
}

// Plain text with http(s) links that open in the browser. Calendar
// descriptions can hold HTML; it is shown as text, never rendered.
function Linkified({ text }) {
  const clean = text.replace(/<br\s*\/?>/gi, '\n').replace(/<[^>]+>/g, '')
  const parts = clean.split(/(https?:\/\/[^\s<>"]+)/g)
  return (
    <p className="selectable whitespace-pre-wrap break-words text-[13px]">
      {parts.map((p, i) => (/^https?:\/\//.test(p)
        ? <a key={i} href={p} onClick={(e) => { e.preventDefault(); openUrl(p) }} className="text-accent hover:underline">{p}</a>
        : p))}
    </p>
  )
}

export default function EventDetails({ event, calendar, color, onClose, onEdit, onDeleted }) {
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const [error, setError] = useState('')
  const [repeat, setRepeat] = useState('') // "Weekly on Tuesday", from the series
  const [asking, setAsking] = useState(null)

  useEffect(() => {
    setRepeat('')
    if (!event || !isRepeating(event)) return
    let stale = false
    getEvent(event.calendarId, event.recurringEventId)
      .then((master) => {
        const rule = parseRecurrence(master.recurrence)
        if (!stale && rule) setRepeat(describe(rule, new Date(master.allDay ? `${master.start}T00:00` : master.start)))
      })
      .catch(() => {})
    return () => { stale = true }
  }, [event?.calendarId, event?.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const close = () => {
    setConfirming(false)
    setError('')
    onClose()
  }

  const remove = async (scope = 'this') => {
    setAsking(null)
    setDeleting(true)
    setError('')
    try {
      await deleteEvent(event.calendarId, event.id, scope)
      onDeleted(event, scope)
      close()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setDeleting(false)
    }
  }

  const writable = calendar && (calendar.accessRole === 'owner' || calendar.accessRole === 'writer')
  const editable = event && canEdit(event, calendar)

  return (
    <Dialog.Root open={!!event} onOpenChange={(o) => !o && close()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/40" />
        {event && (
          <Dialog.Content className="fixed left-1/2 top-1/2 z-50 max-h-[85vh] w-[min(520px,92vw)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border border-border bg-panel p-5 shadow-2xl focus:outline-none">
            <div className="flex items-start gap-3">
              <span className="mt-1.5 h-3 w-3 shrink-0 rounded" style={{ backgroundColor: color }} />
              <div className="min-w-0 flex-1">
                <Dialog.Title className="selectable text-[17px] font-semibold leading-snug">{event.summary || '(no title)'}</Dialog.Title>
                <Dialog.Description className="mt-1 flex items-center gap-1.5 text-[13px] text-muted">
                  <Clock size={13} /> {whenLabel(event)}
                </Dialog.Description>
              </div>
              {editable && (
                <button title="Edit" onClick={() => onEdit(event)} className="grid h-8 w-8 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg">
                  <Pencil size={15} />
                </button>
              )}
              <Dialog.Close className="grid h-8 w-8 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg"><X size={16} /></Dialog.Close>
            </div>

            <div className="mt-4 space-y-3 pl-6">
              {event.meetLink && (
                <Button variant="primary" size="sm" onClick={() => openUrl(event.meetLink)}><Video size={14} /> Join meeting</Button>
              )}
              {repeat && (
                <div className="flex gap-2 text-[13px] text-muted"><Repeat size={14} className="mt-0.5 shrink-0" />{repeat}</div>
              )}
              {event.location && (
                <div className="flex gap-2 text-[13px]"><MapPin size={14} className="mt-0.5 shrink-0 text-muted" /><span className="selectable">{event.location}</span></div>
              )}
              {calendar && (
                <div className="flex gap-2 text-[13px] text-muted"><CalendarDays size={14} className="mt-0.5 shrink-0" />{calendar.summary}</div>
              )}
              {event.attendees?.length > 0 && (
                <div className="flex gap-2 text-[13px]">
                  <Users size={14} className="mt-0.5 shrink-0 text-muted" />
                  <ul className="min-w-0 flex-1 space-y-0.5">
                    {event.attendees.map((a) => {
                      const [label, tone] = replies[a.responseStatus] || replies.needsAction
                      return (
                        <li key={a.email} className="flex gap-2">
                          <span className="selectable min-w-0 flex-1 truncate">
                            {a.displayName || a.email}{a.self && ' (you)'}{a.organizer && ' · organizer'}
                          </span>
                          <span className={`shrink-0 ${tone}`}>{label}</span>
                        </li>
                      )
                    })}
                  </ul>
                </div>
              )}
              {event.description && <Linkified text={event.description} />}
            </div>

            {error && <p className="selectable mt-3 text-[13px] text-danger">{error}</p>}
            <div className="mt-5 flex items-center justify-between border-t border-border pt-4">
              {event.htmlLink ? <ExternalLink href={event.htmlLink} size="sm">Open in Google Calendar</ExternalLink> : <span />}
              {writable && (confirming ? (
                <div className="flex items-center gap-2">
                  <span className="text-[13px] text-muted">{event.attendees?.length ? 'Delete and tell guests?' : 'Delete this event?'}</span>
                  <Button size="sm" variant="ghost" onClick={() => setConfirming(false)} disabled={deleting}>Keep</Button>
                  <Button size="sm" variant="danger" disabled={deleting}
                    onClick={() => (isRepeating(event) ? setAsking({ title: 'Delete repeating event', action: 'Delete' }) : remove())}>
                    {deleting && <Loader2 className="animate-spin" size={14} />} Delete
                  </Button>
                </div>
              ) : (
                <Button size="sm" variant="ghost" onClick={() => setConfirming(true)}><Trash2 size={14} /> Delete</Button>
              ))}
            </div>
            <ScopeDialog request={asking} onChoose={remove} onCancel={() => setAsking(null)} />
          </Dialog.Content>
        )}
      </Dialog.Portal>
    </Dialog.Root>
  )
}
