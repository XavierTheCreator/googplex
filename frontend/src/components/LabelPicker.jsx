import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { Check, FolderInput, Tag } from 'lucide-react'
import Button from './ui/Button.jsx'

const item = 'flex cursor-default select-none items-center gap-2 rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised'

// "Label as" (toggle labels on the threads) or "Move to" (add one, leave the
// inbox), listing the user's labels.
export default function LabelPicker({ labels, applied, onToggle, onMove, compact }) {
  const user = (labels || []).filter((l) => l.type === 'user').sort((a, b) => a.name.localeCompare(b.name))
  const set = new Set(applied || [])
  const menu = (mode, Icon, title) => (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <Button size="sm" variant="ghost" title={title} aria-label={title}><Icon size={14} />{!compact && ` ${title}`}</Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content align="start" className="z-50 max-h-80 min-w-48 overflow-y-auto rounded-lg border border-border bg-panel p-1 shadow-xl">
          {user.length === 0 && <div className="px-2.5 py-1.5 text-[13px] text-muted">No labels yet</div>}
          {user.map((l) => (
            <DropdownMenu.Item key={l.id} className={item}
              onSelect={(e) => { if (mode === 'toggle') { e.preventDefault(); onToggle(l, !set.has(l.id)) } else onMove(l) }}>
              {mode === 'toggle' ? <span className="grid w-4 place-items-center">{set.has(l.id) && <Check size={13} />}</span>
                : <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: l.color?.backgroundColor || 'rgb(var(--muted))' }} />}
              {l.name}
            </DropdownMenu.Item>
          ))}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
  return (
    <>
      {onMove && menu('move', FolderInput, 'Move to')}
      {onToggle && menu('toggle', Tag, 'Label')}
    </>
  )
}
