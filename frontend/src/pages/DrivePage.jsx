import { useCallback, useEffect, useRef, useState } from 'react'
import { ArrowDown, ArrowUp, ChevronRight, Clock, FolderPlus, HardDrive, LayoutGrid, List, Loader2, RefreshCw, Star, Trash2, Upload, Users } from 'lucide-react'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import Button from '../components/ui/Button.jsx'
import SearchBox from '../components/SearchBox.jsx'
import FileRow, { rowGrid } from '../components/FileRow.jsx'
import FileTile from '../components/FileTile.jsx'
import UploadDropzone from '../components/UploadDropzone.jsx'
import PreviewDialog from '../components/PreviewDialog.jsx'
import PromptDialog from '../components/PromptDialog.jsx'
import {
  driveCached, driveCreateFolder, driveDownload, driveExport, driveList, driveRename, driveSearch, driveStar, driveTrash,
  driveUpload, driveUploadPaths, errorText, onDriveProgress, openUrl,
} from '../lib/api.js'
import { canPreview, fileKind, sortFiles } from '../lib/files.js'

const views = [
  { id: 'folder', label: 'My Drive', icon: HardDrive },
  { id: 'shared', label: 'Shared with me', icon: Users },
  { id: 'recent', label: 'Recent', icon: Clock },
  { id: 'starred', label: 'Starred', icon: Star },
  { id: 'trash', label: 'Trash', icon: Trash2 },
]

const LAYOUT_KEY = 'gp.driveLayout'
const savedLayout = () => {
  try { return localStorage.getItem(LAYOUT_KEY) || 'list' } catch { return 'list' }
}

function Drive({ onOpenDoc }) {
  // crumbs: where we are. The first is a view; later ones are folders opened from it.
  const [crumbs, setCrumbs] = useState([{ label: 'My Drive', view: 'folder', folderId: '' }])
  const [query, setQuery] = useState('') // the search box
  const [searching, setSearching] = useState('') // the search being shown, if any
  const [files, setFiles] = useState([])
  const [next, setNext] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [layout, setLayoutState] = useState(savedLayout)
  const [sort, setSort] = useState({ by: 'name', desc: false })
  const [selected, setSelected] = useState('')
  const [preview, setPreview] = useState(null)
  const [prompt, setPrompt] = useState(null)

  const here = crumbs[crumbs.length - 1]
  const inTrash = here.view === 'trash'
  const canUpload = !searching && here.view === 'folder'
  const hereRef = useRef(here)
  hereRef.current = here

  const setLayout = (l) => {
    setLayoutState(l)
    try { localStorage.setItem(LAYOUT_KEY, l) } catch { /* not saved; fine */ }
  }

  const load = useCallback(async (more = false) => {
    setLoading(true)
    setError('')
    try {
      const page = searching
        ? await driveSearch(searching, more ? next : '')
        : await driveList(here.view, here.folderId, '', more ? next : '')
      setFiles((fs) => (more ? [...fs, ...page.files] : page.files))
      setNext(page.nextPageToken || '')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [here.view, here.folderId, searching, next])

  useEffect(() => {
    setSelected('')
    setFiles([])
    setNext('')
    if (!searching) driveCached(here.view, here.folderId).then((fs) => fs?.length && setFiles(fs)) // instant, then fresh
    load()
  }, [here.view, here.folderId, searching]) // eslint-disable-line react-hooks/exhaustive-deps

  // A finished upload into the folder on screen shows up right away.
  useEffect(() => onDriveProgress((p) => {
    if (p.kind === 'upload' && p.status === 'done' && hereRef.current.view === 'folder') load()
  }), [load])

  const goView = (v) => {
    setSearching('')
    setQuery('')
    setCrumbs([{ label: views.find((x) => x.id === v).label, view: v, folderId: '' }])
  }

  const open = (f) => {
    const kind = fileKind(f.mimeType)
    if (kind === 'folder') {
      setSearching('')
      // From search results, the folder opens under the view the search started from.
      setCrumbs((c) => [...(searching ? c.slice(0, 1) : c), { label: f.name, view: 'folder', folderId: f.id }])
    } else if (kind === 'doc' || kind === 'sheet') onOpenDoc(kind === 'doc' ? 'docs' : 'sheets', f)
    else if (canPreview(f)) setPreview(f)
    else if (f.webViewLink) openUrl(f.webViewLink)
  }

  const replace = (f) => setFiles((fs) => fs.map((x) => (x.id === f.id ? { ...x, ...f } : x)))
  const remove = (f) => setFiles((fs) => fs.filter((x) => x.id !== f.id))
  const run = async (fn) => {
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(errorText(err))
    }
  }

  const on = (action, f, format) => {
    if (action === 'open') return open(f)
    if (action === 'browser') return openUrl(f.webViewLink)
    if (action === 'download') return run(() => driveDownload(f.id))
    if (action === 'export') return run(() => driveExport(f.id, format))
    if (action === 'star') return run(async () => replace(await driveStar(f.id, !f.starred)))
    if (action === 'rename') {
      return setPrompt({ title: 'Rename', input: f.name, action: 'Rename', run: async (name) => replace(await driveRename(f.id, name)) })
    }
    if (action === 'trash') {
      return setPrompt({
        title: `Move “${f.name}” to the trash?`, message: 'You can restore it from the trash in Google Drive for 30 days.',
        action: 'Move to trash', danger: true, run: async () => { await driveTrash(f.id); remove(f) },
      })
    }
  }

  const newFolder = () => setPrompt({
    title: 'New folder', input: 'Untitled folder', action: 'Create',
    run: async (name) => { const f = await driveCreateFolder(here.folderId, name); setFiles((fs) => [f, ...fs]) },
  })

  const sorted = sortFiles(files, sort.by, sort.desc)
  const SortHead = ({ by, children, className = '' }) => (
    <button onClick={() => setSort((s) => ({ by, desc: s.by === by ? !s.desc : false }))} className={`flex items-center gap-1 text-left hover:text-fg ${className}`}>
      {children}{sort.by === by && (sort.desc ? <ArrowDown size={12} /> : <ArrowUp size={12} />)}
    </button>
  )

  return (
    <div className="flex h-full min-h-0">
      <nav className="w-52 shrink-0 space-y-0.5 border-r border-border p-3">
        {views.map((v) => {
          const Icon = v.icon
          const active = !searching && crumbs[0].view === v.id
          return (
            <button key={v.id} onClick={() => goView(v.id)}
              className={`flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[13px] ${active ? 'bg-raised text-fg' : 'text-muted hover:bg-raised/60 hover:text-fg'}`}>
              <Icon size={15} /> {v.label}
            </button>
          )
        })}
      </nav>
      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-border px-5 py-2.5">
          <div className="flex min-w-0 flex-1 items-center gap-1 text-[14px]">
            {searching ? (
              <span className="truncate font-medium">Results for “{searching}”</span>
            ) : crumbs.map((c, i) => (
              <span key={i} className="flex min-w-0 items-center gap-1">
                {i > 0 && <ChevronRight size={14} className="shrink-0 text-muted" />}
                <button onClick={() => setCrumbs((cs) => cs.slice(0, i + 1))}
                  className={`truncate rounded px-1 ${i === crumbs.length - 1 ? 'font-medium' : 'text-muted hover:text-fg'}`}>{c.label}</button>
              </span>
            ))}
          </div>
          <SearchBox value={query} onChange={(v) => { setQuery(v); if (!v) setSearching('') }} onSubmit={setSearching} placeholder="Search Drive" />
          <div className="flex rounded-md bg-raised p-0.5">
            <button aria-label="List" onClick={() => setLayout('list')} className={`grid h-7 w-7 place-items-center rounded ${layout === 'list' ? 'bg-panel text-fg shadow-sm' : 'text-muted'}`}><List size={15} /></button>
            <button aria-label="Grid" onClick={() => setLayout('grid')} className={`grid h-7 w-7 place-items-center rounded ${layout === 'grid' ? 'bg-panel text-fg shadow-sm' : 'text-muted'}`}><LayoutGrid size={15} /></button>
          </div>
          <Button size="sm" variant="ghost" aria-label="Refresh" onClick={() => load()} disabled={loading}>
            {loading ? <Loader2 className="animate-spin" size={14} /> : <RefreshCw size={14} />}
          </Button>
          {canUpload && (
            <>
              <Button size="sm" onClick={newFolder}><FolderPlus size={14} /> New folder</Button>
              <Button size="sm" variant="primary" onClick={() => run(() => driveUpload(here.folderId))}><Upload size={14} /> Upload</Button>
            </>
          )}
        </div>
        {error && <p className="selectable border-b border-border px-5 py-2 text-[13px] text-danger">{error}</p>}
        {inTrash && <p className="border-b border-border px-5 py-2 text-[13px] text-muted">Files in the trash can be restored or deleted for good in Google Drive.</p>}
        <UploadDropzone disabled={!canUpload} onDrop={(paths) => run(() => driveUploadPaths(here.folderId, paths))} className="min-h-0 flex-1 overflow-y-auto">
          {layout === 'list' ? (
            <div role="table">
              <div className={`${rowGrid} sticky top-0 z-10 border-b border-border bg-bg px-4 py-2 text-[12px] text-muted`}>
                <SortHead by="name">Name</SortHead>
                <SortHead by="owner">Owner</SortHead>
                <SortHead by="modified">Modified</SortHead>
                <SortHead by="size">Size</SortHead>
                <span />
              </div>
              {sorted.map((f) => (
                <FileRow key={f.id} file={f} selected={selected === f.id} onSelect={(x) => setSelected(x.id)} onOpen={open} on={on} inTrash={inTrash} />
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3 p-5">
              {sorted.map((f) => (
                <FileTile key={f.id} file={f} selected={selected === f.id} onSelect={(x) => setSelected(x.id)} onOpen={open} on={on} inTrash={inTrash} />
              ))}
            </div>
          )}
          {!loading && files.length === 0 && !error && (
            <p className="py-16 text-center text-muted">
              {searching ? 'No files match.' : canUpload ? 'This folder is empty. Drop files here to upload them.' : 'Nothing here.'}
            </p>
          )}
          {next && (
            <div className="flex justify-center py-6">
              <Button onClick={() => load(true)} disabled={loading}>{loading && <Loader2 className="animate-spin" size={14} />} Load more</Button>
            </div>
          )}
        </UploadDropzone>
      </div>
      <PreviewDialog file={preview} onClose={() => setPreview(null)} onDownload={(f) => run(() => driveDownload(f.id))} />
      <PromptDialog request={prompt} onClose={() => setPrompt(null)} />
    </div>
  )
}

export default function DrivePage({ onOpenDoc }) {
  return (
    <div className="flex h-full flex-col">
      <PageHeader title="Drive" />
      <div className="min-h-0 flex-1">
        <FeatureGate service="drive" apiName="Google Drive API" why="Drive needs its API switched on in your Cloud project.">
          <Drive onOpenDoc={onOpenDoc} />
        </FeatureGate>
      </div>
    </div>
  )
}
