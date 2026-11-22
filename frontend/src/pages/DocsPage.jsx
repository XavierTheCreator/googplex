import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, Loader2, Pencil, Plus, RefreshCw, Replace } from 'lucide-react'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import Button from '../components/ui/Button.jsx'
import Textarea from '../components/ui/Textarea.jsx'
import Input from '../components/ui/Input.jsx'
import ExternalLink from '../components/ExternalLink.jsx'
import PromptDialog from '../components/PromptDialog.jsx'
import RecentFiles from '../components/RecentFiles.jsx'
import { docsAppend, docsGet, docsReplace, driveGet, driveRename, errorText, openUrl } from '../lib/api.js'

export const docsApis = { drive: 'Google Drive API', docs: 'Google Docs API', sheets: 'Google Sheets API' }

// A Doc rendered read-only, with two light edits: add text at the end, and
// find and replace. Only offered when Drive says the user can edit the file.
function DocView({ id, onBack }) {
  const [doc, setDoc] = useState(null)
  const [meta, setMeta] = useState(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [panel, setPanel] = useState('') // append | replace
  const [text, setText] = useState('')
  const [find, setFind] = useState('')
  const [repl, setRepl] = useState('')
  const [matchCase, setMatchCase] = useState(false)
  const [busy, setBusy] = useState(false)
  const [note, setNote] = useState('')
  const [prompt, setPrompt] = useState(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const [d, m] = await Promise.all([docsGet(id), driveGet(id)])
      setDoc(d)
      setMeta(m)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => { load() }, [load])

  const canEdit = !!meta?.capabilities?.canEdit

  const act = async (fn) => {
    setBusy(true)
    setError('')
    setNote('')
    try {
      await fn()
      await load()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  // Links inside the document open in the browser.
  const onClick = (e) => {
    const a = e.target.closest('a[href]')
    if (!a) return
    e.preventDefault()
    if (/^https?:/i.test(a.getAttribute('href'))) openUrl(a.href)
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex items-center gap-2 border-b border-border px-5 py-2.5">
        <Button size="sm" variant="ghost" onClick={onBack}><ArrowLeft size={14} /> Recent</Button>
        <h2 className="min-w-0 flex-1 truncate font-medium">{doc?.title || meta?.name || 'Document'}</h2>
        {loading && <Loader2 className="animate-spin text-muted" size={15} />}
        {canEdit && (
          <>
            <Button size="sm" variant="ghost" onClick={() => setPrompt({ title: 'Edit title', input: meta.name, action: 'Save', run: async (name) => { await driveRename(id, name); await load() } })}>
              <Pencil size={14} /> Title
            </Button>
            <Button size="sm" onClick={() => setPanel(panel === 'append' ? '' : 'append')}><Plus size={14} /> Append</Button>
            <Button size="sm" onClick={() => setPanel(panel === 'replace' ? '' : 'replace')}><Replace size={14} /> Find and replace</Button>
          </>
        )}
        <Button size="sm" variant="ghost" aria-label="Refresh" onClick={load} disabled={loading}><RefreshCw size={14} /></Button>
        {meta?.webViewLink && <ExternalLink href={meta.webViewLink} size="sm">Open in Google Docs</ExternalLink>}
      </div>
      {meta && !canEdit && <p className="border-b border-border px-5 py-2 text-[13px] text-muted">You can view this document but not edit it.</p>}
      {panel === 'append' && (
        <div className="flex items-start gap-2 border-b border-border px-5 py-3">
          <Textarea value={text} onChange={(e) => setText(e.target.value)} placeholder="Text to add at the end of the document" className="h-20 flex-1" />
          <Button variant="primary" disabled={busy || !text.trim()} onClick={() => act(async () => { await docsAppend(id, text); setText(''); setNote('Added to the end.') })}>
            {busy && <Loader2 className="animate-spin" size={14} />} Add
          </Button>
        </div>
      )}
      {panel === 'replace' && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border px-5 py-3">
          <Input value={find} onChange={(e) => setFind(e.target.value)} placeholder="Find" className="h-8 w-56" />
          <Input value={repl} onChange={(e) => setRepl(e.target.value)} placeholder="Replace with" className="h-8 w-56" />
          <label className="flex items-center gap-1.5 text-[13px] text-muted">
            <input type="checkbox" checked={matchCase} onChange={(e) => setMatchCase(e.target.checked)} /> Match case
          </label>
          <Button size="sm" variant="primary" disabled={busy || !find}
            onClick={() => act(async () => { const n = await docsReplace(id, find, repl, matchCase); setNote(n === 1 ? 'Replaced 1 match.' : `Replaced ${n} matches.`) })}>
            {busy && <Loader2 className="animate-spin" size={14} />} Replace all
          </Button>
        </div>
      )}
      {(error || note) && <p className={`selectable border-b border-border px-5 py-2 text-[13px] ${error ? 'text-danger' : 'text-success'}`}>{error || note}</p>}
      <div className="min-h-0 flex-1 overflow-y-auto bg-raised/50 px-6 py-8">
        {doc && (
          // Built by the Go side from the document's structure, with all text escaped.
          <article onClick={onClick} className="doc-page selectable mx-auto max-w-[816px] rounded-sm bg-white px-16 py-14 text-[#202124] shadow"
            dangerouslySetInnerHTML={{ __html: doc.html }} />
        )}
      </div>
      <PromptDialog request={prompt} onClose={() => setPrompt(null)} />
    </div>
  )
}

export default function DocsPage({ docId, onOpen }) {
  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Docs" />
      <div className="min-h-0 flex-1">
        <FeatureGate service="docs" apiName="Google Docs API" apiNames={docsApis}
          why="Docs and Sheets need the Drive, Docs and Sheets APIs switched on in your Cloud project.">
          {docId ? <DocView id={docId} onBack={() => onOpen('')} /> : <RecentFiles kind="docs" onOpen={(f) => onOpen(f.id)} />}
        </FeatureGate>
      </div>
    </div>
  )
}
