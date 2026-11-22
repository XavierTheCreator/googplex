import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, GripVertical, Loader2, Play, Shuffle, Trash2 } from 'lucide-react'
import Button from './ui/Button.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import { useVideoActions } from './VideoActions.jsx'
import { errorText, ytMovePlaylistItem, ytPlaylistItems, ytRemoveFromPlaylist } from '../lib/api.js'
import { formatDuration } from '../lib/format.js'

// One playlist's videos: Play all, Shuffle, and (for your own playlists)
// drag to reorder and remove. Loads 50 at a time.
export default function PlaylistView({ playlist, editable, onBack, header }) {
  const { playList } = usePlayer()
  const actions = useVideoActions()
  const [items, setItems] = useState([])
  const [next, setNext] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [drag, setDrag] = useState(null) // { from, to }

  const load = useCallback(async (token = '') => {
    setLoading(true)
    setError('')
    try {
      const r = await ytPlaylistItems(playlist.id, token)
      setItems((x) => (token ? [...x, ...r.items] : r.items))
      setNext(r.nextPageToken || '')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [playlist.id])

  useEffect(() => { load() }, [load])

  const videos = items.map((i) => i.video)

  const remove = async (item) => {
    const before = items
    setItems((x) => x.filter((i) => i.itemId !== item.itemId))
    try {
      await ytRemoveFromPlaylist(playlist.id, item.itemId)
      actions?.notify('Removed from playlist')
    } catch (err) {
      setItems(before)
      setError(errorText(err))
    }
  }

  const drop = async (from, to) => {
    if (from === to) return
    const before = items
    const out = [...items]
    const [it] = out.splice(from, 1)
    out.splice(to, 0, it)
    setItems(out)
    try {
      await ytMovePlaylistItem(it.itemId, playlist.id, it.video.id, to)
    } catch (err) {
      setItems(before)
      setError(errorText(err))
    }
  }

  const handle = (i) => ({
    onPointerDown: (e) => { e.preventDefault(); e.currentTarget.setPointerCapture(e.pointerId); setDrag({ from: i, to: i }) },
    onPointerMove: (e) => {
      if (!drag) return
      const rows = [...document.querySelectorAll('[data-playlist-row]')]
      const to = rows.findIndex((r) => { const b = r.getBoundingClientRect(); return e.clientY >= b.top && e.clientY <= b.bottom })
      if (to >= 0 && to !== drag.to) setDrag({ ...drag, to })
    },
    onPointerUp: () => { if (drag) drop(drag.from, drag.to); setDrag(null) },
    onPointerCancel: () => setDrag(null),
  })

  return (
    <div>
      <div className="mb-4 flex flex-wrap items-center gap-3">
        {onBack && <Button size="sm" variant="ghost" onClick={onBack}><ArrowLeft size={14} /> Back</Button>}
        <div className="min-w-0 flex-1">
          <h2 className="truncate text-[17px] font-semibold">{playlist.title}</h2>
          <p className="text-[12px] text-muted">{[playlist.itemCount && `${playlist.itemCount} videos`, playlist.privacy].filter(Boolean).join(' · ')}</p>
        </div>
        {header}
        <Button size="sm" variant="primary" onClick={() => playList(videos, 0)} disabled={!videos.length}><Play size={14} /> Play all</Button>
        <Button size="sm" onClick={() => playList(videos, 0, true)} disabled={videos.length < 2}><Shuffle size={14} /> Shuffle</Button>
      </div>
      {error && <p className="selectable mb-3 text-[13px] text-danger">{error}</p>}
      <ul className="divide-y divide-border rounded-lg border border-border bg-panel">
        {items.map((it, i) => (
          <li key={it.itemId} data-playlist-row
            className={`group flex items-center gap-3 px-3 py-2 ${drag?.to === i && drag.from !== i ? 'border-t-2 border-t-accent' : ''} ${drag?.from === i ? 'opacity-40' : ''}`}>
            {editable ? <span {...handle(i)} className="cursor-grab text-muted/40 group-hover:text-muted" style={{ touchAction: 'none' }}><GripVertical size={15} /></span>
              : <span className="w-6 text-right text-[12px] text-muted tabular-nums">{i + 1}</span>}
            <button onClick={() => playList(videos, i)} className="flex min-w-0 flex-1 items-center gap-3 text-left">
              <span className="relative shrink-0">
                <img src={it.video.thumbnail} alt="" referrerPolicy="no-referrer" className="aspect-video w-32 rounded object-cover" />
                {formatDuration(it.video.durationSeconds) && (
                  <span className="absolute bottom-1 right-1 rounded bg-black/80 px-1 text-[11px] text-white tabular-nums">{formatDuration(it.video.durationSeconds)}</span>
                )}
              </span>
              <span className="min-w-0">
                <span className="line-clamp-2 text-[13px] font-medium">{it.video.title}</span>
                <span className="block truncate text-[12px] text-muted">{it.video.channelTitle}</span>
              </span>
            </button>
            {editable && (
              <button onClick={() => remove(it)} title="Remove from playlist" aria-label="Remove from playlist"
                className="grid h-7 w-7 place-items-center rounded text-muted opacity-0 hover:bg-raised hover:text-danger group-hover:opacity-100"><Trash2 size={14} /></button>
            )}
          </li>
        ))}
      </ul>
      {!loading && items.length === 0 && !error && <p className="py-10 text-center text-muted">This playlist is empty.</p>}
      {loading && <div className="grid h-14 place-items-center text-muted"><Loader2 className="animate-spin" size={16} /></div>}
      {next && !loading && <div className="mt-4 flex justify-center"><Button onClick={() => load(next)}>Load more</Button></div>}
    </div>
  )
}
