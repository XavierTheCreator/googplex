import { useEffect, useRef, useState } from 'react'
import { Bold, Italic, Link, List, ListOrdered, Quote, Underline } from 'lucide-react'

const tools = [
  ['bold', Bold, 'Bold (⌘B)'],
  ['italic', Italic, 'Italic (⌘I)'],
  ['underline', Underline, 'Underline (⌘U)'],
  ['link', Link, 'Link'],
  ['insertUnorderedList', List, 'Bulleted list'],
  ['insertOrderedList', ListOrdered, 'Numbered list'],
  ['quote', Quote, 'Quote'],
]

// A small rich-text box for email: bold, italic, underline, links, lists and
// quotes, using the browser's editing commands. initialHTML must already be
// safe (see sanitizeForEditor); onChange gets { html, text } as the user types.
export default function RichEditor({ initialHTML, onChange, className = '' }) {
  const ref = useRef(null)
  const saved = useRef(null) // the selection while the link field has focus
  const [linking, setLinking] = useState(false)
  const [url, setUrl] = useState('https://')

  useEffect(() => {
    if (ref.current) ref.current.innerHTML = initialHTML || ''
  }, [initialHTML])

  const emit = () => {
    const el = ref.current
    if (el) onChange({ html: el.innerHTML, text: el.innerText })
  }

  const run = (cmd) => {
    ref.current?.focus()
    if (cmd === 'link') {
      const sel = window.getSelection()
      saved.current = sel.rangeCount ? sel.getRangeAt(0).cloneRange() : null
      setUrl('https://')
      setLinking(true)
      return
    } else if (cmd === 'quote') {
      document.execCommand('formatBlock', false, 'blockquote')
    } else {
      document.execCommand(cmd)
    }
    emit()
  }

  // The webview has no prompt(), so links get a small field in the toolbar.
  const applyLink = () => {
    setLinking(false)
    const u = url.trim()
    if (!/^(https?:\/\/.+|mailto:.+)/i.test(u) || !saved.current) return
    ref.current?.focus()
    const sel = window.getSelection()
    sel.removeAllRanges()
    sel.addRange(saved.current)
    if (sel.isCollapsed) document.execCommand('insertText', false, u) // no text chosen: the address becomes the text
    if (sel.isCollapsed) {
      const r = sel.getRangeAt(0)
      r.setStart(r.startContainer, Math.max(0, r.startOffset - u.length))
      sel.removeAllRanges()
      sel.addRange(r)
    }
    document.execCommand('createLink', false, u)
    emit()
  }

  return (
    <div className={`flex flex-col overflow-hidden rounded-md border border-border bg-bg focus-within:ring-2 focus-within:ring-accent/50 ${className}`}>
      <div className="flex gap-0.5 border-b border-border px-1.5 py-1">
        {tools.map(([cmd, Icon, title]) => (
          <button key={cmd} type="button" title={title} aria-label={title}
            onMouseDown={(e) => { e.preventDefault(); run(cmd) }}
            className="grid h-7 w-7 place-items-center rounded text-muted hover:bg-raised hover:text-fg">
            <Icon size={14} />
          </button>
        ))}
        {linking && (
          <input autoFocus value={url} onChange={(e) => setUrl(e.target.value)} onBlur={() => setLinking(false)}
            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); applyLink() } if (e.key === 'Escape') setLinking(false) }}
            placeholder="https://" aria-label="Link address"
            className="ml-1 h-7 flex-1 rounded border border-border bg-panel px-2 text-[13px] outline-none focus:ring-2 focus:ring-accent/50" />
        )}
      </div>
      <div
        ref={ref}
        contentEditable
        suppressContentEditableWarning
        onInput={emit}
        onPaste={(e) => { // paste as plain text, so no outside markup or styles come in
          e.preventDefault()
          document.execCommand('insertText', false, e.clipboardData.getData('text/plain'))
        }}
        className="rich-editor min-h-0 flex-1 overflow-y-auto px-3 py-2 text-[14px] leading-relaxed outline-none"
      />
    </div>
  )
}
