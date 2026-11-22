import { useEffect, useRef } from 'react'
import { Search } from 'lucide-react'
import Input from './ui/Input.jsx'

// Enter searches, Esc clears, Cmd/Ctrl+K focuses it from anywhere on the page.
export default function SearchBox({ value, onChange, onSubmit, placeholder }) {
  const ref = useRef(null)

  useEffect(() => {
    const onKey = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        ref.current?.focus()
        ref.current?.select()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const onKeyDown = (e) => {
    if (e.key === 'Enter' && value.trim()) onSubmit(value.trim())
    if (e.key === 'Escape') {
      onChange('')
      ref.current?.blur()
    }
  }

  return (
    <div className="relative w-72">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted" size={15} />
      <Input ref={ref} value={value} onChange={(e) => onChange(e.target.value)} onKeyDown={onKeyDown}
        placeholder={placeholder} className="h-8 pl-8 pr-12" />
      <kbd className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded border border-border px-1 text-[11px] text-muted">⌘K</kbd>
    </div>
  )
}
