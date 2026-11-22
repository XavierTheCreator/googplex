import { useState } from 'react'
import { Loader2 } from 'lucide-react'

const letter = (i) => {
  let s = ''
  for (i++; i > 0; i = Math.floor((i - 1) / 26)) s = String.fromCharCode(65 + ((i - 1) % 26)) + s
  return s
}
const numeric = (v) => /^[-+]?[$€£]?\(?[\d,]*\.?\d+\)?%?$/.test(String(v).trim())

// The cell grid: column letters and row numbers stay put, as do the sheet's
// frozen rows. Double-click edits a cell when allowed.
export default function SheetGrid({ rows, cols, frozen, canEdit, onStartEdit, onSave, onNearEnd }) {
  const [editing, setEditing] = useState(null) // { row, col, value, before }
  const [opening, setOpening] = useState(null)

  const begin = async (row, col) => {
    if (!canEdit) return
    setOpening({ row, col })
    try {
      const before = await onStartEdit(row, col)
      setEditing({ row, col, value: before, before })
    } finally {
      setOpening(null)
    }
  }

  const finish = (save) => {
    const e = editing
    setEditing(null)
    if (save && e && e.value !== e.before) onSave(e.row, e.col, e.value, e.before)
  }

  const onScroll = (e) => {
    const el = e.currentTarget
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 400) onNearEnd()
  }

  const shown = Math.max(rows.length, 30)
  const headH = 26 // px, the column letter row
  const rowH = 26

  return (
    <div className="selectable min-h-0 flex-1 overflow-auto bg-panel" onScroll={onScroll}>
      <table className="border-separate border-spacing-0 text-[13px]">
        <thead>
          <tr>
            <th className="sticky left-0 top-0 z-30 h-[26px] w-12 min-w-12 border-b border-r border-border bg-raised" />
            {Array.from({ length: cols }, (_, c) => (
              <th key={c} className="sticky top-0 z-20 h-[26px] min-w-28 border-b border-r border-border bg-raised px-2 text-center text-[12px] font-normal text-muted">
                {letter(c)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {Array.from({ length: shown }, (_, r) => {
            const sticky = r < frozen
            return (
              <tr key={r}>
                <td className={`sticky left-0 z-10 border-b border-r border-border bg-raised px-2 text-right text-[12px] text-muted ${sticky ? 'z-20' : ''}`}
                  style={sticky ? { top: headH + r * rowH } : undefined}>{r + 1}</td>
                {Array.from({ length: cols }, (_, c) => {
                  const v = rows[r]?.[c] ?? ''
                  const isEditing = editing && editing.row === r && editing.col === c
                  const isOpening = opening && opening.row === r && opening.col === c
                  return (
                    <td key={c} onDoubleClick={() => begin(r, c)}
                      className={`h-[26px] max-w-64 truncate border-b border-r border-border px-2 ${numeric(v) ? 'text-right tabular-nums' : ''} ${sticky ? 'sticky z-10 bg-raised/90 font-medium' : ''} ${isEditing ? 'p-0 ring-2 ring-inset ring-accent' : ''}`}
                      style={sticky ? { top: headH + r * rowH } : undefined} title={v.length > 30 ? v : undefined}>
                      {isEditing ? (
                        <input autoFocus value={editing.value} onChange={(e) => setEditing({ ...editing, value: e.target.value })}
                          onKeyDown={(e) => { if (e.key === 'Enter') finish(true); if (e.key === 'Escape') finish(false) }}
                          onBlur={() => finish(true)} className="h-full w-full bg-bg px-2 outline-none" />
                      ) : isOpening ? <Loader2 className="animate-spin text-muted" size={12} /> : v}
                    </td>
                  )
                })}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
