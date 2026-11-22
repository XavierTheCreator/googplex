// Shaping Google Tasks for the Home page. Due dates are plain YYYY-MM-DD
// strings (Google keeps no time of day), compared with today's local date.
import { addDays, isoDate } from './calendar.js'

export const today = () => isoDate(new Date())

// { label, tone } for a due badge, or null when there's no due date.
export function dueInfo(task) {
  if (!task.due) return null
  const t = today()
  const tomorrow = isoDate(addDays(new Date(), 1))
  if (task.status !== 'completed' && task.due < t) return { label: `Overdue · ${shortDate(task.due)}`, tone: 'danger' }
  if (task.due === t) return { label: 'Today', tone: 'accent' }
  if (task.due === tomorrow) return { label: 'Tomorrow', tone: 'muted' }
  return { label: shortDate(task.due), tone: 'muted' }
}

export function shortDate(due) {
  const [y, m, d] = due.split('-').map(Number)
  const date = new Date(y, m - 1, d)
  const sameYear = y === new Date().getFullYear()
  return date.toLocaleDateString([], { weekday: sameYear ? 'short' : undefined, month: 'short', day: 'numeric', year: sameYear ? undefined : 'numeric' })
}

const byPosition = (a, b) => (a.position || '').localeCompare(b.position || '')

// Open tasks as a tree (parents with their subtasks, in Google's order) and
// completed tasks newest first. Hidden tasks (cleared) are left out.
export function shapeTasks(tasks) {
  const visible = tasks.filter((t) => !t.hidden)
  const open = visible.filter((t) => t.status !== 'completed')
  const ids = new Set(open.map((t) => t.id))
  const children = {}
  for (const t of open) if (t.parent && ids.has(t.parent)) (children[t.parent] ||= []).push(t)
  const tree = open.filter((t) => !t.parent || !ids.has(t.parent)).sort(byPosition)
    .map((t) => ({ task: t, children: (children[t.id] || []).sort(byPosition) }))
  const completed = visible.filter((t) => t.status === 'completed')
    .sort((a, b) => (b.completed || '').localeCompare(a.completed || ''))
  return { tree, completed }
}

// Open tasks for a filter other than All, as a flat list ordered by due date.
export function filterTasks(tasks, filter) {
  const t = today()
  const open = tasks.filter((x) => !x.hidden && x.status !== 'completed')
  const pick = {
    today: (x) => x.due === t,
    upcoming: (x) => x.due && x.due > t,
    overdue: (x) => x.due && x.due < t,
  }[filter]
  return open.filter(pick).sort((a, b) => a.due.localeCompare(b.due) || byPosition(a, b))
}

export const openCount = (tasks) => tasks.filter((t) => !t.hidden && t.status !== 'completed').length

export const dueShortcuts = () => [
  ['Today', today()],
  ['Tomorrow', isoDate(addDays(new Date(), 1))],
  ['Next week', isoDate(addDays(new Date(), 7))],
]
