import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { ListTodo, MoreHorizontal, Plus } from 'lucide-react'

const item = 'flex cursor-default select-none items-center rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised data-[disabled]:opacity-40'

// The left column: task lists with their open counts, a menu per list, and New list.
export default function TaskLists({ lists, counts, selected, onSelect, onNew, onRename, onDelete }) {
  return (
    <div className="space-y-0.5">
      <h3 className="mb-2 px-2 text-[12px] font-medium uppercase tracking-wide text-muted">Lists</h3>
      {lists.map((l) => (
        <div key={l.id} className={`group flex items-center rounded-md ${l.id === selected ? 'bg-raised' : 'hover:bg-raised/60'}`}>
          <button onClick={() => onSelect(l.id)} className={`flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5 text-left text-[13px] ${l.id === selected ? 'text-fg' : 'text-muted'}`}>
            <ListTodo size={14} className="shrink-0" />
            <span className="truncate">{l.title}</span>
            {counts[l.id] > 0 && <span className="ml-auto text-[12px] tabular-nums text-muted">{counts[l.id]}</span>}
          </button>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger asChild>
              <button aria-label={`${l.title} options`} className="mr-1 grid h-6 w-6 place-items-center rounded text-muted opacity-0 hover:text-fg group-hover:opacity-100 data-[state=open]:opacity-100">
                <MoreHorizontal size={14} />
              </button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content align="start" className="z-50 min-w-40 rounded-lg border border-border bg-panel p-1 shadow-xl">
                <DropdownMenu.Item className={item} onSelect={() => onRename(l)}>Rename</DropdownMenu.Item>
                <DropdownMenu.Item className={`${item} text-danger`} disabled={l.default} onSelect={() => onDelete(l)}>
                  {l.default ? 'Default list (can’t delete)' : 'Delete list'}
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
        </div>
      ))}
      <button onClick={onNew} className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] text-muted hover:bg-raised/60 hover:text-fg">
        <Plus size={14} /> New list
      </button>
    </div>
  )
}
