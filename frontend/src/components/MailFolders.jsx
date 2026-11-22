import { useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { AlertOctagon, Archive, ChevronDown, ChevronRight, File, Inbox, MoreHorizontal, PenSquare, Plus, Send, Star, Tag, Trash2, TriangleAlert } from 'lucide-react'
import Button from './ui/Button.jsx'
import { folders } from '../lib/mail.js'

const icons = { INBOX: Inbox, STARRED: Star, IMPORTANT: TriangleAlert, SENT: Send, DRAFTS: File, ALL: Archive, SPAM: AlertOctagon, TRASH: Trash2 }
const item = 'flex cursor-default select-none items-center rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised'

function Row({ active, onClick, icon: Icon, color, name, count, depth = 0, menu, open, onToggle }) {
  return (
    <div className={`group flex items-center rounded-md ${active ? 'bg-accent/15 text-fg' : 'text-muted hover:bg-raised/60 hover:text-fg'}`} style={{ paddingLeft: depth * 12 }}>
      {onToggle ? (
        <button onClick={onToggle} className="grid h-6 w-4 place-items-center text-muted">{open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}</button>
      ) : depth > 0 && <span className="w-4" />}
      <button onClick={onClick} className="flex min-w-0 flex-1 items-center gap-2.5 px-2 py-1.5 text-left text-[13px]">
        {Icon ? <Icon size={15} className="shrink-0" /> : <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ backgroundColor: color || 'rgb(var(--muted))' }} />}
        <span className={`truncate ${count ? 'font-semibold text-fg' : ''}`}>{name}</span>
        {count > 0 && <span className="ml-auto text-[12px] tabular-nums">{count}</span>}
      </button>
      {menu}
    </div>
  )
}

function LabelNode({ node, depth, current, onPick, onRename, onDelete }) {
  const [open, setOpen] = useState(true)
  const l = node.label
  const kids = node.children || []
  const menu = l.id && (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button aria-label={`${l.name} options`} className="mr-1 grid h-6 w-6 place-items-center rounded text-muted opacity-0 hover:text-fg group-hover:opacity-100 data-[state=open]:opacity-100">
          <MoreHorizontal size={14} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content align="start" className="z-50 min-w-36 rounded-lg border border-border bg-panel p-1 shadow-xl">
          <DropdownMenu.Item className={item} onSelect={() => onRename(l)}>Rename</DropdownMenu.Item>
          <DropdownMenu.Item className={`${item} text-danger`} onSelect={() => onDelete(l)}>Delete label</DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
  return (
    <>
      <Row depth={depth} name={node.short} color={l.color?.backgroundColor} count={l.threadsUnread}
        active={current === l.id} onClick={() => l.id && onPick({ id: l.id, name: l.name })} menu={menu}
        onToggle={kids.length ? () => setOpen((o) => !o) : null} open={open} />
      {open && kids.map((k) => <LabelNode key={k.label.name} node={k} depth={depth + 1} current={current} onPick={onPick} onRename={onRename} onDelete={onDelete} />)}
    </>
  )
}

// The left pane: Compose, system folders with unread counts, and user labels nested by "/".
export default function MailFolders({ labels, tree, current, onPick, onCompose, onNewLabel, onRename, onDelete }) {
  const byId = Object.fromEntries((labels || []).map((l) => [l.id, l]))
  return (
    <nav className="flex w-56 shrink-0 flex-col overflow-y-auto border-r border-border p-3">
      <Button variant="primary" className="mb-3 justify-start" onClick={onCompose}><PenSquare size={15} /> Compose</Button>
      {folders.map((f) => (
        <Row key={f.id} icon={icons[f.id]} name={f.name} active={current === f.id}
          count={['INBOX', 'SPAM', 'DRAFTS'].includes(f.id) ? (f.id === 'DRAFTS' ? byId.DRAFT?.threadsTotal : byId[f.id]?.threadsUnread) : 0}
          onClick={() => onPick({ id: f.id, name: f.name })} />
      ))}
      <div className="mt-4 mb-1 flex items-center justify-between px-2">
        <span className="flex items-center gap-1.5 text-[12px] font-medium uppercase tracking-wide text-muted"><Tag size={12} /> Labels</span>
        <button aria-label="New label" title="New label" onClick={onNewLabel} className="grid h-6 w-6 place-items-center rounded text-muted hover:bg-raised hover:text-fg"><Plus size={14} /></button>
      </div>
      {(tree || []).map((n) => <LabelNode key={n.label.name} node={n} depth={0} current={current} onPick={onPick} onRename={onRename} onDelete={onDelete} />)}
      {(tree || []).length === 0 && <p className="px-2 text-[12px] text-muted">No labels yet.</p>}
    </nav>
  )
}
