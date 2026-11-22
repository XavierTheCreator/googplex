import clsx from 'clsx'
import { ListChecks, Mail, CalendarDays, Youtube, Settings, HardDrive, FileText, FileSpreadsheet } from 'lucide-react'
import Tooltip from './ui/Tooltip.jsx'
import { useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { Check, Monitor, Moon, Sun } from 'lucide-react'
import { nextTheme, themeLabels, useTheme } from '../lib/theme.js'
import { setTheme } from '../lib/api.js'

const items = [
  { id: 'home', label: 'Home', icon: ListChecks },
  { id: 'gmail', label: 'Gmail', icon: Mail },
  { id: 'calendar', label: 'Calendar', icon: CalendarDays },
  { id: 'youtube', label: 'YouTube', icon: Youtube },
  { id: 'drive', label: 'Drive', icon: HardDrive },
  { id: 'docs', label: 'Docs', icon: FileText },
  { id: 'sheets', label: 'Sheets', icon: FileSpreadsheet },
]

function NavButton({ item, active, onSelect }) {
  const Icon = item.icon
  return (
    <Tooltip label={item.label}>
      <button
        onClick={() => onSelect(item.id)}
        className={clsx(
          'grid h-10 w-10 place-items-center rounded-md transition-colors',
          active ? 'bg-raised text-fg' : 'text-muted hover:text-fg hover:bg-raised/60',
        )}
      >
        <Icon size={18} strokeWidth={1.8} />
      </button>
    </Tooltip>
  )
}

export default function Sidebar({ route, onSelect }) {
  return (
    <nav className="flex w-16 flex-col items-center gap-1 border-r border-border bg-panel pt-3 pb-3">
      {items.map((item) => (
        <NavButton key={item.id} item={item} active={route === item.id} onSelect={onSelect} />
      ))}
      <div className="flex-1" />
      <ThemeButton />
      <NavButton item={{ id: 'settings', label: 'Settings', icon: Settings }} active={route === 'settings'} onSelect={onSelect} />
    </nav>
  )
}

const themeIcons = { light: Sun, dark: Moon, system: Monitor }
const menuItem = 'flex cursor-default select-none items-center gap-2 rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised'

// Click cycles Light, Dark, System; right-click opens a menu to pick one.
// The icon shows the current choice.
function ThemeButton() {
  const { choice } = useTheme()
  const [open, setOpen] = useState(false)
  const Icon = themeIcons[choice]
  return (
    <DropdownMenu.Root open={open} onOpenChange={setOpen}>
      <Tooltip label={`Theme: ${themeLabels[choice]} (click to change, right-click to choose)`}>
        <DropdownMenu.Trigger asChild>
          <button aria-label={`Theme: ${themeLabels[choice]}`}
            onPointerDown={(e) => e.preventDefault()} // a plain click cycles instead of opening the menu
            onClick={() => setTheme(nextTheme(choice))}
            onContextMenu={(e) => { e.preventDefault(); setOpen(true) }}
            onKeyDown={(e) => { if (e.key === 'ArrowDown') setOpen(true) }}
            className="grid h-10 w-10 place-items-center rounded-md text-muted hover:bg-raised/60 hover:text-fg">
            <Icon size={18} strokeWidth={1.8} />
          </button>
        </DropdownMenu.Trigger>
      </Tooltip>
      <DropdownMenu.Portal>
        <DropdownMenu.Content side="right" align="end" sideOffset={6} className="z-50 min-w-36 rounded-lg border border-border bg-panel p-1 shadow-xl">
          {['system', 'light', 'dark'].map((t) => {
            const I = themeIcons[t]
            return (
              <DropdownMenu.Item key={t} className={menuItem} onSelect={() => setTheme(t)}>
                <I size={14} /> <span className="flex-1">{themeLabels[t]}</span> {choice === t && <Check size={13} />}
              </DropdownMenu.Item>
            )
          })}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}
