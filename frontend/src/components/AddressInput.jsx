import { useMemo, useRef, useState } from 'react'
import { X } from 'lucide-react'

const split = (s) => (s || '').split(/,(?=(?:[^"]*"[^"]*")*[^"]*$)/).map((x) => x.trim()).filter(Boolean)
const label = (addr) => {
  const m = addr.match(/^"?([^"<]*?)"?\s*<([^>]+)>$/)
  return m ? (m[1].trim() || m[2]) : addr
}
const format = (c) => (c.name ? `"${c.name.replace(/"/g, '')}" <${c.email}>` : c.email)

// Recipients as chips. Type to see suggestions from people you've mailed; Enter,
// comma or Tab turns what's typed into a chip; Backspace removes the last one.
// value and onChange use a comma-separated string, as the Go side expects.
export default function AddressInput({ value, onChange, contacts, placeholder, autoFocus }) {
  const [text, setText] = useState('')
  const [active, setActive] = useState(0)
  const input = useRef(null)
  const chips = split(value)

  const suggestions = useMemo(() => {
    const q = text.trim().toLowerCase()
    if (!q) return []
    const taken = new Set(chips.map((c) => (c.match(/<([^>]+)>/)?.[1] || c).toLowerCase()))
    return (contacts || []).filter((c) => !taken.has(c.email.toLowerCase()) &&
      (c.email.toLowerCase().includes(q) || (c.name || '').toLowerCase().includes(q))).slice(0, 6)
  }, [text, contacts, value]) // eslint-disable-line react-hooks/exhaustive-deps

  const add = (addr) => {
    const a = addr.trim().replace(/[,;]$/, '')
    if (!a) return
    onChange([...chips, a].join(', '))
    setText('')
    setActive(0)
  }
  const remove = (i) => onChange(chips.filter((_, k) => k !== i).join(', '))

  const onKeyDown = (e) => {
    if (suggestions.length && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault()
      setActive((a) => (a + (e.key === 'ArrowDown' ? 1 : -1) + suggestions.length) % suggestions.length)
    } else if ((e.key === 'Enter' || e.key === 'Tab') && suggestions.length) {
      e.preventDefault()
      add(format(suggestions[active]))
    } else if ((e.key === 'Enter' || e.key === ',' || e.key === ';' || (e.key === 'Tab' && text)) && text.trim()) {
      e.preventDefault()
      add(text)
    } else if (e.key === 'Backspace' && !text && chips.length) {
      remove(chips.length - 1)
    }
  }

  return (
    <div className="relative flex-1">
      <div onClick={() => input.current?.focus()}
        className="flex min-h-9 flex-wrap items-center gap-1 rounded-md border border-border bg-bg px-1.5 py-1 focus-within:ring-2 focus-within:ring-accent/50">
        {chips.map((c, i) => (
          <span key={`${c}-${i}`} title={c} className="flex items-center gap-1 rounded-full bg-raised py-0.5 pl-2 pr-1 text-[13px]">
            {label(c)}
            <button aria-label={`Remove ${c}`} onClick={(e) => { e.stopPropagation(); remove(i) }} className="grid h-4 w-4 place-items-center rounded-full text-muted hover:bg-border hover:text-fg"><X size={11} /></button>
          </span>
        ))}
        <input ref={input} value={text} autoFocus={autoFocus} placeholder={chips.length ? '' : placeholder}
          onChange={(e) => setText(e.target.value)} onKeyDown={onKeyDown} onBlur={() => text.trim() && add(text)}
          className="h-7 min-w-24 flex-1 bg-transparent px-1 text-[14px] outline-none" />
      </div>
      {suggestions.length > 0 && (
        <ul className="absolute inset-x-0 top-full z-10 mt-1 overflow-hidden rounded-lg border border-border bg-panel py-1 shadow-xl">
          {suggestions.map((c, i) => (
            <li key={c.email}>
              <button onMouseDown={(e) => { e.preventDefault(); add(format(c)) }}
                className={`flex w-full items-baseline gap-2 px-3 py-1.5 text-left text-[13px] ${i === active ? 'bg-raised' : ''}`}>
                <span className="font-medium">{c.name || c.email}</span>
                {c.name && <span className="truncate text-muted">{c.email}</span>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
