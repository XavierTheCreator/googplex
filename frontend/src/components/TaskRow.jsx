import { useEffect, useRef, useState } from 'react'
import { Check, GripVertical, Plus, StickyNote } from 'lucide-react'
import { dueInfo } from '../lib/tasks.js'

const tones = { danger: 'bg-danger/15 text-danger', accent: 'bg-accent/15 text-accent', muted: 'bg-raised text-muted' }

// One task: a round checkbox, the title (click to edit), its due badge, and
// hover actions. Space completes, Delete removes, E opens details when focused.
export default function TaskRow({ task, depth = 0, dragging, dropLine, onToggle, onRename, onOpen, onDelete, onAddSubtask, dragHandle }) {
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState(task.title)
  const input = useRef(null)
  const done = task.status === 'completed'
  const due = dueInfo(task)

  useEffect(() => setTitle(task.title), [task.title])
  useEffect(() => { if (editing) input.current?.select() }, [editing])

  const save = () => {
    setEditing(false)
    if (title.trim() && title.trim() !== task.title) onRename(task, title.trim())
    else setTitle(task.title)
  }

  const onKeyDown = (e) => {
    if (editing || e.target !== e.currentTarget) return
    if (e.key === ' ') { e.preventDefault(); onToggle(task) }
    else if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); onDelete(task) }
    else if (e.key === 'e' || e.key === 'E') { e.preventDefault(); onOpen(task) }
    else if (e.key === 'Enter') { e.preventDefault(); setEditing(true) }
  }

  return (
    <div
      data-task-id={task.id}
      tabIndex={0}
      onKeyDown={onKeyDown}
      className={`group relative flex items-center gap-2 rounded-md py-1.5 pr-2 outline-none focus-visible:bg-raised/70 hover:bg-raised/50 ${dragging ? 'opacity-40' : ''}`}
      style={{ paddingLeft: 8 + depth * 28 }}
    >
      {dropLine && <div className={`pointer-events-none absolute inset-x-2 h-0.5 rounded bg-accent ${dropLine === 'before' ? '-top-px' : '-bottom-px'}`} />}
      {dragHandle ? (
        <span {...dragHandle} className="-ml-1 cursor-grab text-muted/0 group-hover:text-muted active:cursor-grabbing" style={{ touchAction: 'none' }}>
          <GripVertical size={14} />
        </span>
      ) : <span className="w-[10px]" />}
      <button
        aria-label={done ? 'Mark not done' : 'Mark done'}
        onClick={() => onToggle(task)}
        className={`grid h-[18px] w-[18px] shrink-0 place-items-center rounded-full border-2 transition-colors ${done ? 'border-accent bg-accent text-on-accent' : 'border-muted/60 text-transparent hover:border-accent hover:text-accent/60'}`}
      >
        <Check size={11} strokeWidth={3} />
      </button>
      <div className="min-w-0 flex-1">
        {editing ? (
          <input ref={input} value={title} onChange={(e) => setTitle(e.target.value)} onBlur={save}
            onKeyDown={(e) => { if (e.key === 'Enter') save(); if (e.key === 'Escape') { setTitle(task.title); setEditing(false) } }}
            className="w-full rounded bg-bg px-1 py-0.5 text-[14px] outline-none ring-2 ring-accent/50" />
        ) : (
          <button onClick={() => (done ? onOpen(task) : setEditing(true))}
            className={`block w-full truncate text-left text-[14px] ${done ? 'text-muted line-through' : ''}`}>
            {task.title || '(untitled)'}
          </button>
        )}
        {task.notes && !editing && <div className="truncate text-[12px] text-muted">{task.notes.split('\n')[0]}</div>}
      </div>
      {task.notes && <StickyNote size={12} className="shrink-0 text-muted" />}
      {due && <span className={`shrink-0 rounded px-1.5 py-0.5 text-[11px] font-medium ${tones[due.tone]}`}>{due.label}</span>}
      {!done && depth === 0 && onAddSubtask && (
        <button aria-label="Add subtask" title="Add subtask" onClick={() => onAddSubtask(task)}
          className="grid h-6 w-6 place-items-center rounded text-muted opacity-0 hover:bg-raised hover:text-fg group-hover:opacity-100"><Plus size={14} /></button>
      )}
      <button onClick={() => onOpen(task)} className="rounded px-1.5 text-[12px] text-muted opacity-0 hover:bg-raised hover:text-fg group-hover:opacity-100">Details</button>
    </div>
  )
}
