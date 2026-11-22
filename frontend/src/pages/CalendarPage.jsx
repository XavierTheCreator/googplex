import { useCallback, useEffect, useMemo, useState } from 'react'
import * as ToggleGroup from '@radix-ui/react-toggle-group'
import { ChevronLeft, ChevronRight, Loader2, Plus, RefreshCw } from 'lucide-react'
import FeatureGate from '../components/FeatureGate.jsx'
import PageHeader from '../components/PageHeader.jsx'
import Button from '../components/ui/Button.jsx'
import Input from '../components/ui/Input.jsx'
import CalendarList from '../components/CalendarList.jsx'
import WeekView from '../components/WeekView.jsx'
import MonthView from '../components/MonthView.jsx'
import AgendaView from '../components/AgendaView.jsx'
import EventDetails from '../components/EventDetails.jsx'
import EventDialog from '../components/EventDialog.jsx'
import ScopeDialog from '../components/ScopeDialog.jsx'
import { errorText, getCalendars, getEvents, getWeekStart, quickAddEvent, refreshEvents, setCalendarVisible, updateEvent } from '../lib/api.js'
import { addDays, canEdit, colorOf as colorFor, isoDate, isRepeating, rangeFor, rangeLabel, shift, withDates } from '../lib/calendar.js'

const VIEW_KEY = 'gp.calendarView'

function savedView() {
  try {
    return localStorage.getItem(VIEW_KEY) || 'week'
  } catch {
    return 'week'
  }
}

const typing = () => {
  const el = document.activeElement
  return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}

function Calendar() {
  const [view, setViewState] = useState(savedView)
  const [anchor, setAnchor] = useState(() => new Date())
  const [calendars, setCalendars] = useState([])
  const [raw, setRaw] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [failed, setFailed] = useState(0)
  const [selected, setSelected] = useState(null) // event shown in EventDetails
  const [editing, setEditing] = useState(null) // { event } or { start } for EventDialog
  const [quick, setQuick] = useState('')
  const [quickBusy, setQuickBusy] = useState(false)
  const [weekStart, setWeekStart] = useState(0)
  const [dropping, setDropping] = useState(null) // a dragged repeating event waiting for its scope
  const [toast, setToast] = useState('')

  const setView = (v) => {
    if (!v) return // toggle groups allow deselecting; keep a view
    setViewState(v)
    try { localStorage.setItem(VIEW_KEY, v) } catch { /* not saved; fine */ }
  }

  const { start, end } = useMemo(() => rangeFor(view, anchor, weekStart), [view, anchor, weekStart])
  const visibleKey = calendars.filter((c) => c.visible).map((c) => c.id).join('|')

  useEffect(() => {
    getCalendars().then(setCalendars).catch((err) => setError(errorText(err)))
    getWeekStart().then(setWeekStart)
  }, [])

  useEffect(() => {
    if (!toast) return
    const t = setTimeout(() => setToast(''), 5000)
    return () => clearTimeout(t)
  }, [toast])

  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const r = await refreshEvents(start, end)
      setRaw(r.events || [])
      setFailed(r.failed)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [start, end])

  useEffect(() => {
    if (!calendars.length) return
    let stale = false
    getEvents(start, end).then((evs) => !stale && setRaw(evs || [])) // cache first, then Google
    refresh()
    return () => { stale = true }
  }, [start, end, visibleKey, refresh]) // eslint-disable-line react-hooks/exhaustive-deps

  const events = useMemo(() => raw.map(withDates), [raw])
  const byId = useMemo(() => Object.fromEntries(calendars.map((c) => [c.id, c])), [calendars])
  const colorOf = useCallback((ev) => colorFor(ev, byId), [byId])
  const writable = calendars.filter((c) => c.accessRole === 'owner' || c.accessRole === 'writer')
  const canDrag = useCallback((ev) => canEdit(ev, byId[ev.calendarId]), [byId])

  // A drop moves the event on screen at once; if Google refuses, it goes back.
  const commitDrop = async ({ event, s, e }, scope) => {
    setDropping(null)
    const at = (d) => (event.allDay ? isoDate(d) : d.toISOString())
    const before = raw
    setRaw((list) => list.map((x) => (x.id === event.id && x.calendarId === event.calendarId ? { ...x, start: at(s), end: at(e) } : x)))
    try {
      await updateEvent({
        calendarId: event.calendarId, eventId: event.id, onlyTimes: true, allDay: event.allDay,
        start: at(s), end: at(e), timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      }, scope)
      refresh() // a series change moves other occurrences too
    } catch (err) {
      setRaw(before)
      setToast(`Couldn't move “${event.summary || 'event'}”: ${errorText(err)}`)
    }
  }
  const dropRequest = useMemo(() => dropping && { title: 'Move repeating event', action: 'Move' }, [dropping])
  const onDrop = (event, s, e) => {
    if (isRepeating(event)) setDropping({ event, s, e })
    else commitDrop({ event, s, e }, 'this')
  }

  const toggle = async (cal, visible) => {
    await setCalendarVisible(cal.id, visible) // saved first: the refresh it triggers reads it
    setCalendars((cs) => cs.map((c) => (c.id === cal.id ? { ...c, visible } : c)))
  }

  // A saved, deleted or quick-added event shows at once, then the range reloads.
  const upsert = (ev) => {
    setRaw((list) => [...list.filter((e) => !(e.id === ev.id && e.calendarId === ev.calendarId)), ev])
    refresh()
  }
  const removed = (ev, scope) => {
    setRaw((list) => list.filter((e) => !(e.id === ev.id && e.calendarId === ev.calendarId)))
    if (scope && scope !== 'this') refresh() // other occurrences are gone too
  }

  const quickAdd = async () => {
    const primary = writable.find((c) => c.primary) || writable[0]
    if (!quick.trim() || !primary) return
    setQuickBusy(true)
    setError('')
    try {
      upsert(await quickAddEvent(primary.id, quick.trim()))
      setQuick('')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setQuickBusy(false)
    }
  }

  // T today, arrows move, N new event; not while typing or a dialog is open.
  useEffect(() => {
    const onKey = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey || typing() || document.querySelector('[role="dialog"]')) return
      if (e.key === 't' || e.key === 'T') setAnchor(new Date())
      else if (e.key === 'ArrowLeft') setAnchor((a) => shift(view, a, -1))
      else if (e.key === 'ArrowRight') setAnchor((a) => shift(view, a, 1))
      else if (e.key === 'n' || e.key === 'N') setEditing({ start: null })
      else return
      e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [view])

  const days = Array.from({ length: Math.round((end - start) / 86400000) }, (_, i) => addDays(start, i))
  const item = 'h-7 rounded px-2.5 text-[13px] text-muted data-[state=on]:bg-panel data-[state=on]:text-fg data-[state=on]:shadow-sm'

  return (
    <div className="flex h-full min-h-0 flex-col">
      <header className="flex items-center gap-3 border-b border-border px-6 py-3">
        <Button size="sm" onClick={() => setAnchor(new Date())}>Today</Button>
        <div className="flex">
          <Button size="sm" variant="ghost" aria-label="Previous" onClick={() => setAnchor((a) => shift(view, a, -1))}><ChevronLeft size={16} /></Button>
          <Button size="sm" variant="ghost" aria-label="Next" onClick={() => setAnchor((a) => shift(view, a, 1))}><ChevronRight size={16} /></Button>
        </div>
        <span className="min-w-0 flex-1 truncate text-[15px] font-medium">{rangeLabel(view, anchor, weekStart)}</span>
        {loading ? <Loader2 className="animate-spin text-muted" size={15} /> : (
          <Button size="sm" variant="ghost" aria-label="Refresh" onClick={refresh}><RefreshCw size={14} /></Button>
        )}
        <ToggleGroup.Root type="single" value={view} onValueChange={setView} className="flex rounded-md bg-raised p-0.5">
          <ToggleGroup.Item value="agenda" className={item}>Agenda</ToggleGroup.Item>
          <ToggleGroup.Item value="week" className={item}>Week</ToggleGroup.Item>
          <ToggleGroup.Item value="month" className={item}>Month</ToggleGroup.Item>
        </ToggleGroup.Root>
        <Button size="sm" variant="primary" onClick={() => setEditing({ start: null })} disabled={!writable.length}>
          <Plus size={14} /> New event
        </Button>
      </header>
      {(error || failed > 0) && (
        <p className="selectable border-b border-border px-6 py-2 text-[13px] text-danger">
          {error || `${failed} calendar${failed === 1 ? '' : 's'} could not be loaded.`}
        </p>
      )}
      <div className="flex min-h-0 flex-1">
        <aside className="w-56 shrink-0 space-y-5 overflow-y-auto border-r border-border p-3">
          <Input value={quick} onChange={(e) => setQuick(e.target.value)} disabled={quickBusy || !writable.length}
            onKeyDown={(e) => e.key === 'Enter' && quickAdd()} placeholder="Quick add: lunch Fri 1pm" className="h-8 text-[13px]" />
          <CalendarList calendars={calendars} onToggle={toggle} />
        </aside>
        <section className="min-w-0 flex-1">
          {view === 'week' && (
            <WeekView days={days} events={events} colorOf={colorOf} canDrag={canDrag} onSelect={setSelected}
              onCreate={(s) => setEditing({ start: s })} onDrop={onDrop} />
          )}
          {view === 'month' && (
            <MonthView days={days} month={anchor.getMonth()} events={events} colorOf={colorOf} canDrag={canDrag} onSelect={setSelected} onDrop={onDrop}
              onCreate={(s) => setEditing({ start: s })} onOpenDay={(d) => { setAnchor(d); setView('week') }} />
          )}
          {view === 'agenda' && <AgendaView start={start} events={events} colorOf={colorOf} onSelect={setSelected} />}
        </section>
      </div>
      <EventDetails
        event={selected}
        calendar={selected && byId[selected.calendarId]}
        color={selected ? colorOf(selected) : ''}
        onClose={() => setSelected(null)}
        onEdit={(ev) => { setSelected(null); setEditing({ event: ev }) }}
        onDeleted={removed}
      />
      <EventDialog
        open={!!editing}
        event={editing?.event}
        start={editing?.start}
        calendars={writable}
        onClose={() => setEditing(null)}
        onSaved={upsert}
      />
      <ScopeDialog
        request={dropRequest}
        onChoose={(scope) => commitDrop(dropping, scope)}
        onCancel={() => setDropping(null)}
      />
      {toast && (
        <div role="status" className="selectable fixed bottom-5 left-1/2 z-[80] max-w-[90vw] -translate-x-1/2 rounded-lg border border-border bg-panel px-4 py-2.5 text-[13px] text-danger shadow-2xl">
          {toast}
        </div>
      )}
    </div>
  )
}

export default function CalendarPage() {
  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Calendar" />
      <div className="min-h-0 flex-1">
        <FeatureGate service="calendar" apiName="Google Calendar API" why="Calendar needs its API switched on in your Cloud project.">
          <Calendar />
        </FeatureGate>
      </div>
    </div>
  )
}
