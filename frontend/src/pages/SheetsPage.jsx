import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowLeft, Loader2, Plus, RefreshCw, Undo2 } from 'lucide-react'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import Button from '../components/ui/Button.jsx'
import ExternalLink from '../components/ExternalLink.jsx'
import RecentFiles from '../components/RecentFiles.jsx'
import SheetGrid from '../components/SheetGrid.jsx'
import { docsApis } from './DocsPage.jsx'
import { driveGet, errorText, sheetsAppend, sheetsFormula, sheetsGet, sheetsUpdate, sheetsValues } from '../lib/api.js'

const PAGE = 200 // rows per load
const MAX_COLS = 26 // A to Z

// A spreadsheet tab as a grid. Double-click a cell to edit it (Enter saves,
// Escape cancels), add a row at the end, and undo the last edit.
function SheetView({ id, onBack }) {
  const [book, setBook] = useState(null)
  const [meta, setMeta] = useState(null)
  const [tab, setTab] = useState(0)
  const [rows, setRows] = useState([])
  const [done, setDone] = useState(false) // every row of the tab is loaded
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [undo, setUndo] = useState(null) // { tab, row, col, before }
  const [newRow, setNewRow] = useState([])
  const loadingRef = useRef(false)

  const sheet = book?.sheets?.[tab]
  const cols = Math.min(sheet?.cols || MAX_COLS, MAX_COLS)
  const canEdit = !!meta?.capabilities?.canEdit

  useEffect(() => {
    setError('')
    Promise.all([sheetsGet(id), driveGet(id)])
      .then(([b, m]) => { setBook(b); setMeta(m) })
      .catch((err) => setError(errorText(err)))
  }, [id])

  const loadRows = useCallback(async (from) => {
    if (!sheet || loadingRef.current) return
    loadingRef.current = true
    setLoading(true)
    try {
      const v = (await sheetsValues(id, sheet.title, from, PAGE, cols)) || []
      setRows((r) => (from === 0 ? v : [...r, ...v]))
      setDone(v.length < PAGE)
    } catch (err) {
      setError(errorText(err))
    } finally {
      loadingRef.current = false
      setLoading(false)
    }
  }, [id, sheet, cols])

  useEffect(() => {
    setRows([])
    setDone(false)
    setUndo(null)
    setNewRow([])
    loadRows(0)
  }, [tab, book]) // eslint-disable-line react-hooks/exhaustive-deps

  const setCell = (row, col, value) => setRows((rs) => {
    const next = rs.map((r) => [...r])
    while (next.length <= row) next.push([])
    while (next[row].length <= col) next[row].push('')
    next[row][col] = value
    return next
  })

  // The text to edit: the formula if the cell has one, else its value.
  const startEdit = (row, col) => sheetsFormula(id, sheet.title, row, col)

  const saveEdit = async (row, col, value, before) => {
    setError('')
    try {
      const shown = await sheetsUpdate(id, sheet.title, row, col, value)
      setCell(row, col, shown)
      setUndo({ tab, row, col, before })
    } catch (err) {
      setError(errorText(err))
    }
  }

  const undoLast = async () => {
    if (!undo || undo.tab !== tab) return
    setError('')
    try {
      const shown = await sheetsUpdate(id, sheet.title, undo.row, undo.col, undo.before)
      setCell(undo.row, undo.col, shown)
      setUndo(null)
    } catch (err) {
      setError(errorText(err))
    }
  }

  const addRow = async () => {
    setError('')
    try {
      await sheetsAppend(id, sheet.title, newRow.map((c) => c ?? ''))
      setNewRow([])
      setDone(false)
      loadRows(0)
    } catch (err) {
      setError(errorText(err))
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 border-b border-border px-5 py-2.5">
        <Button size="sm" variant="ghost" onClick={onBack}><ArrowLeft size={14} /> Recent</Button>
        <h2 className="min-w-0 flex-1 truncate font-medium">{book?.title || meta?.name || 'Spreadsheet'}</h2>
        {loading && <Loader2 className="animate-spin text-muted" size={15} />}
        {canEdit && undo?.tab === tab && <Button size="sm" variant="ghost" onClick={undoLast}><Undo2 size={14} /> Undo</Button>}
        <Button size="sm" variant="ghost" aria-label="Refresh" onClick={() => loadRows(0)}><RefreshCw size={14} /></Button>
        {meta?.webViewLink && <ExternalLink href={meta.webViewLink} size="sm">Open in Google Sheets</ExternalLink>}
      </div>
      {meta && !canEdit && <p className="border-b border-border px-5 py-2 text-[13px] text-muted">You can view this spreadsheet but not edit it.</p>}
      {error && <p className="selectable border-b border-border px-5 py-2 text-[13px] text-danger">{error}</p>}
      {sheet && (
        <SheetGrid
          rows={rows}
          cols={cols}
          frozen={Math.min(sheet.frozenRows || 0, 3)}
          canEdit={canEdit}
          onStartEdit={startEdit}
          onSave={saveEdit}
          onNearEnd={() => !done && loadRows(rows.length)}
        />
      )}
      {sheet && canEdit && (
        <div className="flex items-center gap-2 overflow-x-auto border-t border-border px-5 py-2">
          <span className="shrink-0 text-[12px] text-muted">New row</span>
          {Array.from({ length: Math.min(cols, 8) }, (_, c) => (
            <input key={c} value={newRow[c] || ''} placeholder={String.fromCharCode(65 + c)}
              onChange={(e) => setNewRow((r) => { const n = [...r]; n[c] = e.target.value; return n })}
              onKeyDown={(e) => e.key === 'Enter' && newRow.some(Boolean) && addRow()}
              className="h-7 w-28 shrink-0 rounded border border-border bg-bg px-2 text-[13px] focus:outline-none focus:ring-2 focus:ring-accent/50" />
          ))}
          <Button size="sm" onClick={addRow} disabled={!newRow.some(Boolean)}><Plus size={14} /> Add row</Button>
        </div>
      )}
      {book && book.sheets.length > 1 && (
        <div className="flex gap-1 overflow-x-auto border-t border-border bg-panel px-3 py-1.5">
          {book.sheets.map((s, i) => (
            <button key={s.id} onClick={() => setTab(i)}
              className={`shrink-0 rounded px-3 py-1 text-[13px] ${i === tab ? 'bg-raised font-medium text-fg' : 'text-muted hover:text-fg'}`}>{s.title}</button>
          ))}
        </div>
      )}
    </div>
  )
}

export default function SheetsPage({ sheetId, onOpen }) {
  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Sheets" />
      <div className="min-h-0 flex-1">
        <FeatureGate service="docs" apiName="Google Sheets API" apiNames={docsApis}
          why="Docs and Sheets need the Drive, Docs and Sheets APIs switched on in your Cloud project.">
          {sheetId ? <SheetView id={sheetId} onBack={() => onOpen('')} /> : <RecentFiles kind="sheets" onOpen={(f) => onOpen(f.id)} />}
        </FeatureGate>
      </div>
    </div>
  )
}
