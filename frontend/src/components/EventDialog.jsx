import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Loader2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import Input from './ui/Input.jsx'
import Textarea from './ui/Textarea.jsx'
import RepeatPicker from './RepeatPicker.jsx'
import ScopeDialog from './ScopeDialog.jsx'
import { createEvent, errorText, getEvent, updateEvent } from '../lib/api.js'
import { addDays, isoDate, isRepeating, parseDate } from '../lib/calendar.js'
import { buildRRULE, parseRecurrence } from '../lib/rrule.js'

const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone
const hm = (d) => `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`

// Form state from an event being edited, or a new one starting at `start`.
function initialForm(event, start, calendars) {
  if (event) {
    const lastDay = event.allDay ? addDays(event.e, -1) : event.e
    return {
      calendarId: event.calendarId, summary: event.summary || '', location: event.location || '',
      description: event.description || '', allDay: event.allDay,
      startDate: isoDate(event.s), startTime: hm(event.s), endDate: isoDate(lastDay), endTime: hm(event.e),
      guests: (event.attendees || []).filter((a) => !a.self).map((a) => a.email).join(', '),
    }
  }
  const s = start || new Date()
  const e = new Date(s.getTime() + 60 * 60000)
  const primary = calendars.find((c) => c.primary) || calendars[0]
  return {
    calendarId: primary?.id || '', summary: '', location: '', description: '', allDay: false,
    startDate: isoDate(s), startTime: hm(s), endDate: isoDate(e), endTime: hm(e), guests: '',
  }
}

const startOf = (f) => {
  const d = parseDate(f.startDate)
  if (!f.allDay) {
    const [h, m] = f.startTime.split(':').map(Number)
    d.setHours(h, m)
  }
  return d
}

function toInput(f, event, recurrence) {
  const at = (date, time) => {
    const [h, m] = time.split(':').map(Number)
    const d = parseDate(date)
    d.setHours(h, m, 0, 0)
    return d.toISOString()
  }
  return {
    calendarId: event?.calendarId || f.calendarId, eventId: event?.id || '', summary: f.summary.trim(), location: f.location.trim(),
    description: f.description, allDay: f.allDay, timeZone, recurrence,
    moveTo: event && f.calendarId !== event.calendarId ? f.calendarId : '',
    start: f.allDay ? f.startDate : at(f.startDate, f.startTime),
    end: f.allDay ? isoDate(addDays(parseDate(f.endDate), 1)) : at(f.endDate, f.endTime), // all-day end is exclusive
    guests: f.guests.split(/[,;\s]+/).map((s) => s.trim()).filter(Boolean),
  }
}

// Create or edit an event. Only calendars the user can write to are offered;
// picking another one for an existing event moves it there.
export default function EventDialog({ open, event, start, calendars, onClose, onSaved }) {
  const [form, setForm] = useState(() => initialForm(event, start, calendars))
  const [rule, setRule] = useState(null)
  const [custom, setCustom] = useState(false)
  const [originalLine, setOriginalLine] = useState('') // the series' RRULE when the form opened
  const [asking, setAsking] = useState(null) // the scope prompt for a repeating event
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) return
    setForm(initialForm(event, start, calendars))
    setRule(null)
    setCustom(false)
    setOriginalLine('')
    setError('')
    if (event && isRepeating(event)) {
      getEvent(event.calendarId, event.recurringEventId).then((master) => {
        const r = parseRecurrence(master.recurrence)
        setRule(r)
        setCustom(!!r?.raw)
        setOriginalLine((master.recurrence || []).find((l) => l.startsWith('RRULE:')) || '')
      }).catch(() => {}) // the form still works; the repeat menu just starts empty
    }
  }, [open, event, start]) // eslint-disable-line react-hooks/exhaustive-deps

  const set = (k) => (e) => {
    const v = e.target.type === 'checkbox' ? e.target.checked : e.target.value
    setForm((f) => {
      const next = { ...f, [k]: v }
      if (k === 'startDate' && f.endDate < v) next.endDate = v // keep the end after the start
      return next
    })
  }

  const repeating = event && isRepeating(event)
  const moving = event && form.calendarId !== event.calendarId

  // The recurrence to send: null leaves the series' rule untouched.
  const recurrenceFor = (scope) => {
    const line = rule ? buildRRULE(rule, startOf(form), form.allDay) : ''
    if (!event) return line ? [line] : []
    if (repeating && scope === 'this') return null
    if (line === originalLine) return null
    return line ? [line] : []
  }

  const save = async (scope = 'this') => {
    setAsking(null)
    setSaving(true)
    setError('')
    try {
      const input = toInput(form, event, recurrenceFor(scope))
      const saved = event ? await updateEvent(input, scope) : await createEvent(input)
      onSaved(saved)
      onClose()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setSaving(false)
    }
  }

  const submit = () => {
    if (saving) return
    if (!repeating) return save()
    setAsking(moving
      ? { title: 'Move repeating event', action: 'Save', only: ['all'], note: 'Moving to another calendar moves the whole series.' }
      : { title: 'Save repeating event', action: 'Save' })
  }

  const onKeyDown = (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      submit()
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && !saving && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/50" />
        <Dialog.Content onKeyDown={onKeyDown}
          className="fixed left-1/2 top-1/2 z-50 max-h-[90vh] w-[min(560px,92vw)] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-xl border border-border bg-panel shadow-2xl focus:outline-none">
          <div className="flex items-center justify-between border-b border-border px-5 py-3">
            <Dialog.Title className="font-semibold">{event ? 'Edit event' : 'New event'}</Dialog.Title>
            <Dialog.Description className="sr-only">Event details</Dialog.Description>
            <Dialog.Close className="grid h-7 w-7 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg" disabled={saving}><X size={16} /></Dialog.Close>
          </div>
          <div className="space-y-3 px-5 py-4">
            <Input value={form.summary} onChange={set('summary')} placeholder="Add a title" autoFocus className="h-10 text-[15px]" />
            <Row label="Calendar">
              <select value={form.calendarId} onChange={set('calendarId')}
                className="h-9 w-full rounded-md border border-border bg-bg px-2 text-fg focus:outline-none focus:ring-2 focus:ring-accent/50">
                {calendars.map((c) => <option key={c.id} value={c.id}>{c.summary}</option>)}
              </select>
            </Row>
            {moving && (
              <Row label="">
                <p className="text-[12px] text-muted">
                  {repeating ? 'Saving moves the whole series to this calendar.' : 'Saving moves the event to this calendar.'}
                </p>
              </Row>
            )}
            <Row label="">
              <label className="flex items-center gap-2 text-[13px]">
                <input type="checkbox" checked={form.allDay} onChange={set('allDay')} className="accent-[rgb(var(--accent))]" /> All day
              </label>
            </Row>
            <Row label="Starts">
              <Input type="date" value={form.startDate} onChange={set('startDate')} className="w-40" />
              {!form.allDay && <Input type="time" value={form.startTime} onChange={set('startTime')} className="w-32" />}
            </Row>
            <Row label="Ends">
              <Input type="date" value={form.endDate} min={form.startDate} onChange={set('endDate')} className="w-40" />
              {!form.allDay && <Input type="time" value={form.endTime} onChange={set('endTime')} className="w-32" />}
            </Row>
            <Row label="Repeat" top>
              <RepeatPicker rule={rule} start={startOf(form)} custom={custom} onChange={setRule} onCustom={setCustom} />
            </Row>
            <Row label="Location"><Input value={form.location} onChange={set('location')} /></Row>
            <Row label="Guests"><Input value={form.guests} onChange={set('guests')} placeholder="name@example.com, …" /></Row>
            <Textarea value={form.description} onChange={set('description')} placeholder="Description" className="h-28" />
            {form.guests.trim() && <p className="text-[12px] text-muted">Guests get an email from Google when you save.</p>}
            {error && <p className="selectable text-[13px] text-danger">{error}</p>}
          </div>
          <div className="flex justify-end gap-2 border-t border-border px-5 py-3">
            <Button variant="ghost" onClick={onClose} disabled={saving}>Cancel</Button>
            <Button variant="primary" onClick={submit} disabled={saving || !form.calendarId}>
              {saving && <Loader2 className="animate-spin" size={14} />} Save
            </Button>
          </div>
          <ScopeDialog request={asking} onChoose={save} onCancel={() => setAsking(null)} />
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

function Row({ label, top, children }) {
  return (
    <div className={`flex gap-3 ${top ? 'items-start' : 'items-center'}`}>
      <span className={`w-16 shrink-0 text-[13px] text-muted ${top ? 'pt-2' : ''}`}>{label}</span>
      <div className="flex flex-1 gap-2">{children}</div>
    </div>
  )
}
