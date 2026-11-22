import { useCallback, useEffect, useRef, useState } from 'react'
import { Archive, Loader2, MailOpen, RefreshCw, Star, Trash2, Undo2 } from 'lucide-react'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import Button from '../components/ui/Button.jsx'
import SearchBox from '../components/SearchBox.jsx'
import MailFolders from '../components/MailFolders.jsx'
import ThreadRow from '../components/ThreadRow.jsx'
import MessageView from '../components/MessageView.jsx'
import ComposeDialog from '../components/ComposeDialog.jsx'
import LabelPicker from '../components/LabelPicker.jsx'
import PromptDialog from '../components/PromptDialog.jsx'
import Toast from '../components/Toast.jsx'
import {
  checkService, errorText, getInbox, gmailCreateLabel, gmailDeleteLabel, gmailDrafts, gmailGetDraft, gmailLabels, gmailList,
  gmailModify, gmailRenameLabel, gmailReplyDraft, gmailTrash,
} from '../lib/api.js'
import { shortDate } from '../lib/format.js'
import { categories, chips, folderLabels } from '../lib/mail.js'

const typing = () => {
  const el = document.activeElement
  return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}

// What each action does to labels, and what undoes it.
const actions = {
  archive: { remove: ['INBOX'], leaves: (f) => f === 'INBOX', undo: 'inbox', done: 'Archived' },
  inbox: { add: ['INBOX'], leaves: () => false, done: 'Moved to Inbox' },
  spam: { add: ['SPAM'], remove: ['INBOX'], leaves: () => true, undo: 'notspam', done: 'Reported as spam' },
  notspam: { add: ['INBOX'], remove: ['SPAM'], leaves: (f) => f === 'SPAM', undo: 'spam', done: 'Moved to Inbox' },
  read: { remove: ['UNREAD'], leaves: () => false },
  unread: { add: ['UNREAD'], leaves: () => false },
}

function Mail() {
  const [labels, setLabels] = useState({ labels: [], tree: [] })
  const [folder, setFolder] = useState({ id: 'INBOX', name: 'Inbox' })
  const [category, setCategory] = useState('')
  const [query, setQuery] = useState('') // the search box
  const [search, setSearch] = useState('') // the search being shown
  const [activeChips, setActiveChips] = useState([])
  const [threads, setThreads] = useState([])
  const [drafts, setDrafts] = useState([])
  const [next, setNext] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [open, setOpen] = useState(null) // open thread id
  const [openMessages, setOpenMessages] = useState([])
  const [checked, setChecked] = useState(new Set())
  const [cursor, setCursor] = useState(0)
  const [lastCheck, setLastCheck] = useState(-1)
  const [compose, setCompose] = useState(null) // initial draft for the compose window, or null
  const [prompt, setPrompt] = useState(null)
  const [toast, setToast] = useState(null)
  const [driveReady, setDriveReady] = useState(false)
  const searchRef = useRef(null)
  const listRef = useRef(null)
  const toastTimer = useRef(null)
  const threadsRef = useRef(threads)
  threadsRef.current = threads

  const isDrafts = folder.id === 'DRAFTS'
  const q = [search, ...chips.filter((c) => activeChips.includes(c.id)).map((c) => c.q)].filter(Boolean).join(' ')
  const labelIds = folderLabels(folder.id, folder.id === 'INBOX' ? category : '')
  const byId = Object.fromEntries((labels.labels || []).map((l) => [l.id, l]))

  const showToast = useCallback((t) => {
    clearTimeout(toastTimer.current)
    setToast(t)
    toastTimer.current = setTimeout(() => setToast(null), t.ms || 5000)
  }, [])

  const loadLabels = useCallback(() => gmailLabels().then((r) => setLabels({ labels: r.labels || [], tree: r.tree || [] })).catch(() => {}), [])

  useEffect(() => {
    loadLabels()
    checkService('drive').then((s) => setDriveReady(s.status === 'ready')).catch(() => {})
  }, [loadLabels])

  const load = useCallback(async (more = false) => {
    setLoading(true)
    setError('')
    try {
      if (isDrafts) {
        const r = await gmailDrafts(more ? next : '')
        setDrafts((d) => (more ? [...d, ...(r.drafts || [])] : r.drafts || []))
        setNext(r.nextPageToken || '')
      } else {
        const r = await gmailList(q, labelIds, more ? next : '')
        setThreads((t) => (more ? [...t, ...r.threads.filter((x) => !t.some((y) => y.id === x.id))] : r.threads))
        setNext(r.nextPageToken || '')
      }
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [isDrafts, q, labelIds.join(','), next]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setChecked(new Set())
    setCursor(0)
    setNext('')
    if (folder.id === 'INBOX' && !q && !category) getInbox().then((t) => t?.length && setThreads(t)) // instant, then fresh
    else setThreads([])
    load()
  }, [folder.id, category, q]) // eslint-disable-line react-hooks/exhaustive-deps

  // Load more when scrolled near the end.
  const onScroll = (e) => {
    const el = e.currentTarget
    if (next && !loading && el.scrollHeight - el.scrollTop - el.clientHeight < 300) load(true)
  }

  // ---- actions, all optimistic ----

  const targets = (explicit) => {
    if (explicit) return explicit
    if (checked.size) return threadsRef.current.filter((t) => checked.has(t.id))
    const t = threadsRef.current.find((x) => x.id === open) || threadsRef.current[cursor]
    return t ? [t] : []
  }

  const applyLocal = (ids, add = [], remove = [], leaves = false) => setThreads((ts) => (leaves
    ? ts.filter((t) => !ids.includes(t.id))
    : ts.map((t) => {
      if (!ids.includes(t.id)) return t
      const set = new Set(t.labelIds || [])
      add.forEach((l) => set.add(l))
      remove.forEach((l) => set.delete(l))
      return { ...t, labelIds: [...set], unread: set.has('UNREAD'), starred: set.has('STARRED'), important: set.has('IMPORTANT') }
    })))

  const run = async (kind, opt, explicit) => {
    const list = targets(explicit)
    if (!list.length) return
    const ids = list.map((t) => t.id)
    const msgIds = list.flatMap((t) => t.messageIds || [])
    const before = threadsRef.current
    let add = []
    let remove = []
    let leaves = false
    let undo = null
    let done = ''

    if (kind === 'trash' || kind === 'untrash') {
      leaves = true
      done = kind === 'trash' ? (ids.length > 1 ? `${ids.length} conversations moved to trash` : 'Moved to trash') : 'Restored from trash'
      undo = () => gmailTrash(ids, kind === 'trash').then(() => { setThreads(before); load() })
    } else if (kind === 'star' || kind === 'important') {
      const label = kind === 'star' ? 'STARRED' : 'IMPORTANT'
      const allOn = list.every((t) => (t.labelIds || []).includes(label))
      ;(allOn ? remove : add).push(label)
    } else if (kind === 'addLabel' || kind === 'removeLabel') {
      ;(kind === 'addLabel' ? add : remove).push(opt.id)
      leaves = kind === 'removeLabel' && folder.id === opt.id
    } else if (kind === 'move') {
      add = [opt.id]
      remove = ['INBOX']
      leaves = folder.id === 'INBOX'
      done = `Moved to ${opt.name}`
      undo = () => gmailModify(ids, msgIds, ['INBOX'], [opt.id]).then(() => setThreads(before))
    } else {
      const a = actions[kind]
      add = a.add || []
      remove = a.remove || []
      leaves = a.leaves(folder.id)
      done = a.done
      if (a.undo) undo = () => gmailModify(ids, msgIds, remove, add).then(() => setThreads(before))
    }

    applyLocal(ids, add, remove, leaves)
    if (leaves && ids.includes(open)) setOpen(null)
    setChecked(new Set())
    try {
      if (kind === 'trash' || kind === 'untrash') await gmailTrash(ids, kind === 'untrash')
      else await gmailModify(ids, msgIds, add, remove)
      if (done) showToast({ text: done, action: undo ? 'Undo' : '', onAction: undo })
      if (add.concat(remove).some((l) => l === 'UNREAD' || l === 'INBOX' || l === 'SPAM')) loadLabels() // unread counts
    } catch (err) {
      setThreads(before)
      showToast({ text: `That didn't work: ${errorText(err)}`, error: true })
    }
  }

  const openThreadId = (id) => {
    setOpen(id)
    const i = threadsRef.current.findIndex((t) => t.id === id)
    if (i >= 0) setCursor(i)
    applyLocal([id], [], ['UNREAD']) // opening marks it read
  }

  const check = (id, shift) => {
    const i = threadsRef.current.findIndex((t) => t.id === id)
    setChecked((prev) => {
      const nextSet = new Set(prev)
      if (shift && lastCheck >= 0) {
        const [a, b] = [Math.min(lastCheck, i), Math.max(lastCheck, i)]
        threadsRef.current.slice(a, b + 1).forEach((t) => nextSet.add(t.id))
      } else if (nextSet.has(id)) nextSet.delete(id)
      else nextSet.add(id)
      return nextSet
    })
    setLastCheck(i)
  }

  const reply = async (message, mode) => setCompose(await gmailReplyDraft(message, mode))
  const replyLatest = (mode) => {
    const m = openMessages[openMessages.length - 1]
    if (m) reply(m, mode)
  }

  // ---- labels ----

  const newLabel = () => setPrompt({ title: 'New label', input: '', message: 'Use a slash to nest it, like Work/Clients.', action: 'Create',
    run: async (name) => { await gmailCreateLabel(name); await loadLabels() } })
  const renameLabel = (l) => setPrompt({ title: 'Rename label', input: l.name, action: 'Rename',
    run: async (name) => { await gmailRenameLabel(l.id, name); await loadLabels() } })
  const deleteLabel = (l) => setPrompt({ title: `Delete the label “${l.name}”?`, message: 'The emails that have it stay where they are.', action: 'Delete label', danger: true,
    run: async () => { await gmailDeleteLabel(l.id); if (folder.id === l.id) setFolder({ id: 'INBOX', name: 'Inbox' }); await loadLabels() } })

  // ---- keyboard, like Gmail ----

  useEffect(() => {
    const onKey = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey || typing() || document.querySelector('[role="dialog"]')) return
      const list = threadsRef.current
      const k = e.key
      const go = (i) => {
        const n = Math.max(0, Math.min(list.length - 1, i))
        setCursor(n)
        listRef.current?.querySelector(`[data-thread-id="${CSS.escape(list[n]?.id || '')}"]`)?.scrollIntoView({ block: 'nearest' })
        if (open && list[n]) openThreadId(list[n].id)
      }
      const handled = {
        j: () => go(cursor + 1),
        k: () => go(cursor - 1),
        o: () => list[cursor] && openThreadId(list[cursor].id),
        Enter: () => list[cursor] && openThreadId(list[cursor].id),
        x: () => list[cursor] && check(list[cursor].id, false),
        e: () => run(folder.id === 'TRASH' ? 'untrash' : 'archive'),
        '#': () => run('trash'),
        s: () => run('star'),
        U: () => run('unread'),
        r: () => replyLatest('reply'),
        a: () => replyLatest('all'),
        f: () => replyLatest('forward'),
        c: () => setCompose({}),
        '/': () => searchRef.current?.querySelector('input')?.focus(),
      }[k]
      if (handled) {
        e.preventDefault()
        handled()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const frameProps = () => ({ onMailto: (draft) => setCompose(draft), onBlockedForm: () => showToast({ text: 'Open this email in Gmail to use this form.' }) })
  const openThread = threads.find((t) => t.id === open)
  const anyChecked = checked.size > 0
  const checkedList = threads.filter((t) => checked.has(t.id))

  return (
    <div className="flex h-full min-h-0">
      <MailFolders labels={labels.labels} tree={labels.tree} current={folder.id}
        onPick={(f) => { setFolder(f); setOpen(null); setCategory('') }} onCompose={() => setCompose({})}
        onNewLabel={newLabel} onRename={renameLabel} onDelete={deleteLabel} />
      <section className="flex w-[420px] shrink-0 flex-col border-r border-border">
        <div ref={searchRef} className="space-y-2 border-b border-border px-3 py-2.5">
          <div className="flex items-center gap-2">
            <SearchBox value={query} onChange={(v) => { setQuery(v); if (!v) setSearch('') }} onSubmit={setSearch}
              placeholder={`Search ${folder.id === 'ALL' ? 'all mail' : folder.name}`} />
            <Button size="sm" variant="ghost" aria-label="Refresh" onClick={() => { load(); loadLabels() }} disabled={loading}>
              {loading ? <Loader2 className="animate-spin" size={14} /> : <RefreshCw size={14} />}
            </Button>
          </div>
          {!isDrafts && (
            <div className="flex flex-wrap gap-1.5">
              {chips.map((c) => {
                const on = activeChips.includes(c.id)
                return (
                  <button key={c.id} onClick={() => setActiveChips((a) => (on ? a.filter((x) => x !== c.id) : [...a, c.id]))}
                    className={`rounded-full border px-2.5 py-0.5 text-[12px] ${on ? 'border-accent bg-accent/15 text-fg' : 'border-border text-muted hover:text-fg'}`}>
                    {c.label}
                  </button>
                )
              })}
            </div>
          )}
        </div>
        {folder.id === 'INBOX' && !q && (
          <div className="flex gap-3 overflow-x-auto border-b border-border px-3 pt-2 text-[13px]">
            {categories.map((c) => (
              <button key={c.id} onClick={() => setCategory(c.id)}
                className={`-mb-px shrink-0 border-b-2 pb-2 ${category === c.id ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg'}`}>{c.name}</button>
            ))}
          </div>
        )}
        {anyChecked && !isDrafts && (
          <div className="flex items-center gap-1 border-b border-border bg-raised/40 px-3 py-1.5 text-[13px]">
            <input type="checkbox" checked={checked.size === threads.length} aria-label="Select all"
              onChange={(e) => setChecked(e.target.checked ? new Set(threads.map((t) => t.id)) : new Set())} className="mr-1 accent-[rgb(var(--accent))]" />
            <span className="mr-2 text-muted">{checked.size} selected</span>
            {folder.id !== 'TRASH' && <BulkTool title="Archive" onClick={() => run('archive')}><Archive size={15} /></BulkTool>}
            {folder.id === 'TRASH' ? <BulkTool title="Restore" onClick={() => run('untrash')}><Undo2 size={15} /></BulkTool>
              : <BulkTool title="Move to trash" onClick={() => run('trash')}><Trash2 size={15} /></BulkTool>}
            <BulkTool title="Mark as read" onClick={() => run('read')}><MailOpen size={15} /></BulkTool>
            <BulkTool title="Mark as unread" onClick={() => run('unread')}><MailOpen size={15} className="opacity-60" /></BulkTool>
            <BulkTool title="Star" onClick={() => run('star')}><Star size={15} /></BulkTool>
            <LabelPicker compact labels={labels.labels} applied={checkedList.length ? checkedList.map((t) => t.labelIds || []).reduce((a, b) => a.filter((x) => b.includes(x))) : []}
              onToggle={(l, on) => run(on ? 'addLabel' : 'removeLabel', l)} onMove={folder.id === 'INBOX' ? (l) => run('move', l) : null} />
          </div>
        )}
        {error && <p className="selectable border-b border-border p-3 text-[13px] text-danger">{error}</p>}
        <div ref={listRef} onScroll={onScroll} className="min-h-0 flex-1 overflow-y-auto">
          {isDrafts ? drafts.map((d) => (
            <button key={d.draftId} onClick={async () => { try { setCompose(await gmailGetDraft(d.draftId)) } catch (err) { setError(errorText(err)) } }}
              className="block w-full border-b border-border px-4 py-2.5 text-left hover:bg-raised/60">
              <div className="flex items-baseline gap-2">
                <span className="min-w-0 flex-1 truncate text-danger">Draft{d.to ? <span className="text-fg"> · {d.to}</span> : ''}</span>
                <span className="shrink-0 text-[12px] text-muted">{shortDate(d.date)}</span>
              </div>
              <div className="mt-0.5 truncate text-[13px]">{d.subject || '(no subject)'}</div>
              <div className="mt-0.5 truncate text-[13px] text-muted">{d.snippet}</div>
            </button>
          )) : threads.map((t, i) => (
            <ThreadRow key={t.id} thread={t} selected={t.id === open} checked={checked.has(t.id)} focused={i === cursor}
              labelsById={byId} onOpen={openThreadId} onCheck={check} onStar={(th) => run('star', null, [th])} />
          ))}
          {!loading && !error && (isDrafts ? drafts.length === 0 : threads.length === 0) && (
            <p className="p-8 text-center text-muted">{q ? 'No emails match.' : isDrafts ? 'No drafts.' : 'Nothing here.'}</p>
          )}
          {loading && (threads.length > 0 || drafts.length > 0) && <div className="grid h-12 place-items-center text-muted"><Loader2 className="animate-spin" size={15} /></div>}
        </div>
      </section>
      <section className="min-w-0 flex-1 overflow-y-auto">
        <MessageView threadId={isDrafts ? null : open} thread={openThread} folder={folder.id} labels={labels.labels} driveReady={driveReady}
          onAction={(kind, opt) => run(kind, opt, openThread ? [openThread] : undefined)} onReply={reply} onMessages={setOpenMessages}
          frameProps={frameProps} />
      </section>
      <ComposeDialog open={!!compose} initial={compose} onClose={() => setCompose(null)} onSent={() => { showToast({ text: 'Sent' }); load() }} />
      <PromptDialog request={prompt} onClose={() => setPrompt(null)} />
      <Toast toast={toast} onAction={() => { toast?.onAction?.(); setToast(null) }} onDismiss={() => setToast(null)} />
    </div>
  )
}

function BulkTool({ title, onClick, children }) {
  return (
    <button title={title} aria-label={title} onClick={onClick} className="grid h-7 w-7 place-items-center rounded text-muted hover:bg-raised hover:text-fg">
      {children}
    </button>
  )
}

export default function GmailPage() {
  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Gmail" />
      <div className="min-h-0 flex-1">
        <FeatureGate service="gmail" apiName="Gmail API" why="Gmail needs its API switched on in your Cloud project.">
          <Mail />
        </FeatureGate>
      </div>
    </div>
  )
}
