import { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, Loader2, Plus, RefreshCw } from 'lucide-react'
import Button from './ui/Button.jsx'
import TaskLists from './TaskLists.jsx'
import TaskRow from './TaskRow.jsx'
import TaskDetails from './TaskDetails.jsx'
import PromptDialog from './PromptDialog.jsx'
import Toast from './Toast.jsx'
import {
  errorText, tasksCached, tasksCachedOf, tasksClearCompleted, tasksCreate, tasksCreateList, tasksDelete, tasksDeleteList,
  tasksLists, tasksMove, tasksOf, tasksRenameList, tasksUpdate,
} from '../lib/api.js'
import { filterTasks, openCount, shapeTasks } from '../lib/tasks.js'

const filters = [['all', 'All'], ['today', 'Today'], ['upcoming', 'Upcoming'], ['overdue', 'Overdue']]
const UNDO_MS = 5000

const typing = () => {
  const el = document.activeElement
  return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}

// Google Tasks for the Home page. Every change shows at once and rolls back
// with a message if Google refuses it. Deleting waits 5 seconds for Undo.
export default function TasksPanel() {
  const [lists, setLists] = useState([])
  const [listId, setListId] = useState('')
  const [tasks, setTasks] = useState([])
  const [counts, setCounts] = useState({})
  const [filter, setFilter] = useState('all')
  const [loading, setLoading] = useState(false)
  const [quick, setQuick] = useState('')
  const [subFor, setSubFor] = useState('') // parent getting a new subtask
  const [subTitle, setSubTitle] = useState('')
  const [details, setDetails] = useState(null)
  const [prompt, setPrompt] = useState(null)
  const [showDone, setShowDone] = useState(false)
  const [toast, setToast] = useState(null) // { text, action?, error?, onAction? }
  const [drag, setDrag] = useState(null) // { id, over, where }
  const quickRef = useRef(null)
  const tasksRef = useRef(tasks)
  tasksRef.current = tasks
  const pending = useRef(new Map()) // task id -> { timer, listId } for deletes waiting on Undo
  const toastTimer = useRef(null)

  const showToast = useCallback((t) => {
    clearTimeout(toastTimer.current)
    setToast(t)
    toastTimer.current = setTimeout(() => setToast(null), t.ms || 5000)
  }, [])

  // ---- loading ----

  useEffect(() => {
    tasksCached().then((c) => {
      if (c.lists?.length) setLists(c.lists)
      if (c.selected) { setListId(c.selected); setTasks(c.tasks || []) }
    })
    tasksLists().then((ls) => {
      setLists(ls || [])
      setListId((cur) => (cur && ls.some((l) => l.id === cur) ? cur : ls[0]?.id || ''))
      ;(ls || []).forEach((l) => tasksCachedOf(l.id).then((ts) => setCounts((c) => ({ ...c, [l.id]: openCount(ts || []) }))))
    }).catch((err) => showToast({ text: errorText(err), error: true }))
  }, [showToast])

  const refresh = useCallback(async (id = listId) => {
    if (!id) return
    setLoading(true)
    try {
      const ts = (await tasksOf(id)) || []
      const hidden = new Set(pending.current.keys())
      setTasks(ts.filter((t) => !hidden.has(t.id) && !hidden.has(t.parent)))
    } catch (err) {
      showToast({ text: errorText(err), error: true })
    } finally {
      setLoading(false)
    }
  }, [listId, showToast])

  useEffect(() => {
    if (!listId) return
    tasksCachedOf(listId).then((ts) => ts?.length && setTasks(ts))
    refresh(listId)
  }, [listId]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => setCounts((c) => ({ ...c, [listId]: openCount(tasks) })), [tasks, listId])

  // ---- optimistic changes ----

  // apply(tasks) -> tasks shows the change now; call() makes it at Google.
  const change = async (apply, call, failText) => {
    const before = tasksRef.current
    setTasks(apply(before))
    try {
      return await call()
    } catch (err) {
      setTasks(before)
      showToast({ text: `${failText}: ${errorText(err)}`, error: true })
      return null
    }
  }
  const patchLocal = (id, fields) => (ts) => ts.map((t) => (t.id === id ? { ...t, ...fields } : t))
  const swapIn = (saved) => saved && setTasks((ts) => ts.map((t) => (t.id === saved.id ? saved : t)))

  const toggle = (t) => {
    const status = t.status === 'completed' ? 'needsAction' : 'completed'
    change(patchLocal(t.id, { status, completed: status === 'completed' ? new Date().toISOString() : '' }),
      () => tasksUpdate(listId, t.id, { status }), "Couldn't update the task").then(swapIn)
  }

  const rename = (t, title) => change(patchLocal(t.id, { title }), () => tasksUpdate(listId, t.id, { title }), "Couldn't rename the task").then(swapIn)

  const saveDetails = (t, patch) => change(patchLocal(t.id, patch), () => tasksUpdate(listId, t.id, patch), "Couldn't save the task").then(swapIn)

  const moveToList = (t, dest) => change((ts) => ts.filter((x) => x.id !== t.id && x.parent !== t.id),
    () => tasksMove(listId, t.id, '', '', dest), "Couldn't move the task").then((r) => {
    if (r) showToast({ text: `Moved to ${lists.find((l) => l.id === dest)?.title || 'another list'}` })
  })

  const addTask = async (title, parent = '') => {
    if (!title.trim()) return
    const siblings = tasksRef.current.filter((t) => (t.parent || '') === parent && t.status !== 'completed')
      .sort((a, b) => (a.position || '').localeCompare(b.position || ''))
    const previous = parent ? siblings[siblings.length - 1]?.id || '' : '' // subtasks go last, new tasks first
    const temp = { id: `tmp-${Date.now()}`, title: title.trim(), status: 'needsAction', parent, position: parent ? '~' : '' }
    const saved = await change((ts) => [temp, ...ts], () => tasksCreate(listId, { title, parent, previous }), "Couldn't add the task")
    if (saved) setTasks((ts) => ts.map((t) => (t.id === temp.id ? saved : t)))
  }

  // Delete hides the task now and deletes it after the Undo window.
  const remove = (t) => {
    const lid = listId
    const hide = (ts) => ts.filter((x) => x.id !== t.id && x.parent !== t.id)
    const before = tasksRef.current
    setTasks(hide)
    const timer = setTimeout(() => commitDelete(t.id), UNDO_MS)
    pending.current.set(t.id, { timer, listId: lid, before })
    showToast({ text: `Deleted “${t.title || 'task'}”`, action: 'Undo', ms: UNDO_MS, onAction: () => undoDelete(t.id) })
  }
  const commitDelete = async (id) => {
    const p = pending.current.get(id)
    if (!p) return
    pending.current.delete(id)
    try {
      await tasksDelete(p.listId, id)
    } catch (err) {
      if (p.listId === listId) setTasks(p.before)
      showToast({ text: `Couldn't delete the task: ${errorText(err)}`, error: true })
    }
  }
  const undoDelete = (id) => {
    const p = pending.current.get(id)
    if (!p) return
    clearTimeout(p.timer)
    pending.current.delete(id)
    if (p.listId === listId) setTasks(p.before)
    setToast(null)
  }

  // Deletes still waiting go through when leaving the page or closing the app.
  useEffect(() => {
    const flush = () => pending.current.forEach((p, id) => { clearTimeout(p.timer); tasksDelete(p.listId, id) })
    window.addEventListener('beforeunload', flush)
    return () => { window.removeEventListener('beforeunload', flush); flush(); pending.current.clear() }
  }, [])

  // ---- lists ----

  const newList = () => setPrompt({ title: 'New list', input: '', action: 'Create', run: async (title) => {
    const l = await tasksCreateList(title)
    setLists((ls) => [...ls, l])
    setListId(l.id)
  } })
  const renameList = (l) => setPrompt({ title: 'Rename list', input: l.title, action: 'Rename', run: async (title) => {
    const r = await tasksRenameList(l.id, title)
    setLists((ls) => ls.map((x) => (x.id === l.id ? { ...x, title: r.title } : x)))
  } })
  const deleteList = (l) => setPrompt({
    title: `Delete “${l.title}”?`, message: 'This deletes the list and all of its tasks in Google Tasks.', action: 'Delete list', danger: true,
    run: async () => {
      await tasksDeleteList(l.id)
      setLists((ls) => ls.filter((x) => x.id !== l.id))
      if (listId === l.id) setListId(lists.find((x) => x.default)?.id || '')
    },
  })

  const clearDone = () => change((ts) => ts.map((t) => (t.status === 'completed' ? { ...t, hidden: true } : t)),
    () => tasksClearCompleted(listId), "Couldn't clear completed tasks")

  // ---- drag to reorder, among tasks with the same parent ----

  const dragHandle = (task, siblings) => ({
    onPointerDown: (e) => {
      if (e.button !== 0) return
      e.preventDefault()
      e.currentTarget.setPointerCapture(e.pointerId)
      setDrag({ id: task.id, over: '', where: '' })
    },
    onPointerMove: (e) => {
      if (!drag || drag.id !== task.id) return
      let best = null
      for (const s of siblings) {
        const el = document.querySelector(`[data-task-id="${CSS.escape(s.id)}"]`)
        if (!el) continue
        const r = el.getBoundingClientRect()
        if (e.clientY >= r.top && e.clientY <= r.bottom) best = { over: s.id, where: e.clientY < r.top + r.height / 2 ? 'before' : 'after' }
      }
      if (best && (best.over !== drag.over || best.where !== drag.where)) setDrag({ ...drag, ...best })
    },
    onPointerUp: () => {
      const d = drag
      setDrag(null)
      if (!d || !d.over || d.over === task.id) return
      const order = siblings.map((s) => s.id).filter((id) => id !== task.id)
      let at = order.indexOf(d.over) + (d.where === 'after' ? 1 : 0)
      order.splice(at, 0, task.id)
      at = order.indexOf(task.id)
      if (siblings.map((s) => s.id).join() === order.join()) return
      const previous = at > 0 ? order[at - 1] : ''
      const pos = Object.fromEntries(order.map((id, i) => [id, String(i).padStart(20, '0')]))
      change((ts) => ts.map((t) => (pos[t.id] ? { ...t, position: pos[t.id] } : t)),
        () => tasksMove(listId, task.id, task.parent || '', previous), "Couldn't move the task").then((r) => r && refresh())
    },
    onPointerCancel: () => setDrag(null),
  })

  // ---- keyboard: N focuses quick add ----

  useEffect(() => {
    const onKey = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey || typing() || document.querySelector('[role="dialog"]')) return
      if (e.key === 'n' || e.key === 'N') { e.preventDefault(); quickRef.current?.focus() }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const { tree, completed } = shapeTasks(tasks)
  const flat = filter === 'all' ? null : filterTasks(tasks, filter)
  const rowProps = { onToggle: toggle, onRename: rename, onOpen: setDetails, onDelete: remove }
  const list = lists.find((l) => l.id === listId)

  return (
    <div className="flex min-h-0 flex-1">
      <aside className="w-56 shrink-0 overflow-y-auto border-r border-border p-3">
        <TaskLists lists={lists} counts={counts} selected={listId} onSelect={setListId} onNew={newList} onRename={renameList} onDelete={deleteList} />
      </aside>
      <section className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-3 px-6 pt-5">
          <h2 className="min-w-0 flex-1 truncate text-[17px] font-semibold">{list?.title || 'Tasks'}</h2>
          <div className="flex rounded-md bg-raised p-0.5">
            {filters.map(([id, label]) => (
              <button key={id} onClick={() => setFilter(id)}
                className={`h-7 rounded px-2.5 text-[13px] ${filter === id ? 'bg-panel text-fg shadow-sm' : 'text-muted hover:text-fg'}`}>{label}</button>
            ))}
          </div>
          <Button size="sm" variant="ghost" aria-label="Refresh" onClick={() => refresh()} disabled={loading}>
            {loading ? <Loader2 className="animate-spin" size={14} /> : <RefreshCw size={14} />}
          </Button>
        </div>
        <div className="px-6 pt-3">
          <div className="flex items-center gap-2 rounded-lg border border-border bg-panel px-3 focus-within:ring-2 focus-within:ring-accent/50">
            <Plus size={16} className="text-muted" />
            <input ref={quickRef} value={quick} onChange={(e) => setQuick(e.target.value)} disabled={!listId}
              onKeyDown={(e) => { if (e.key === 'Enter' && quick.trim()) { addTask(quick); setQuick('') } if (e.key === 'Escape') e.currentTarget.blur() }}
              placeholder="Add a task (press N)" className="h-10 flex-1 bg-transparent text-[14px] outline-none" />
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
          {flat ? (
            flat.length ? flat.map((t) => <TaskRow key={t.id} task={t} {...rowProps} />)
              : <p className="py-10 text-center text-muted">Nothing {filter === 'today' ? 'due today' : filter}.</p>
          ) : (
            <>
              {tree.map(({ task, children }) => (
                <div key={task.id}>
                  <TaskRow task={task} {...rowProps} onAddSubtask={(t) => { setSubFor(t.id); setSubTitle('') }}
                    dragHandle={dragHandle(task, tree.map((n) => n.task))} dragging={drag?.id === task.id}
                    dropLine={drag && drag.over === task.id && drag.id !== task.id ? drag.where : ''} />
                  {children.map((c) => (
                    <TaskRow key={c.id} task={c} depth={1} {...rowProps} dragHandle={dragHandle(c, children)} dragging={drag?.id === c.id}
                      dropLine={drag && drag.over === c.id && drag.id !== c.id ? drag.where : ''} />
                  ))}
                  {subFor === task.id && (
                    <div className="py-1 pl-[44px] pr-2">
                      <input autoFocus value={subTitle} onChange={(e) => setSubTitle(e.target.value)} placeholder="Subtask"
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' && subTitle.trim()) { addTask(subTitle, task.id); setSubTitle('') }
                          if (e.key === 'Escape') setSubFor('')
                        }}
                        onBlur={() => !subTitle.trim() && setSubFor('')}
                        className="h-8 w-full rounded-md border border-border bg-bg px-2 text-[14px] outline-none focus:ring-2 focus:ring-accent/50" />
                    </div>
                  )}
                </div>
              ))}
              {tree.length === 0 && !loading && <p className="py-10 text-center text-muted">No tasks. Add one above.</p>}
              {completed.length > 0 && (
                <div className="mt-4 border-t border-border pt-2">
                  <div className="flex items-center justify-between px-2">
                    <button onClick={() => setShowDone((s) => !s)} className="flex items-center gap-1 py-1 text-[13px] text-muted hover:text-fg">
                      {showDone ? <ChevronDown size={14} /> : <ChevronRight size={14} />} Completed ({completed.length})
                    </button>
                    {showDone && <button onClick={clearDone} className="text-[12px] text-muted hover:text-fg">Clear completed</button>}
                  </div>
                  {showDone && completed.map((t) => <TaskRow key={t.id} task={t} {...rowProps} />)}
                </div>
              )}
            </>
          )}
        </div>
      </section>
      <TaskDetails task={details} lists={lists} listId={listId} onClose={() => setDetails(null)}
        onSave={saveDetails} onMove={moveToList} onDelete={remove} />
      <PromptDialog request={prompt} onClose={() => setPrompt(null)} />
      <Toast toast={toast} onAction={() => { toast?.onAction?.(); setToast(null) }} onDismiss={() => setToast(null)} />
    </div>
  )
}
