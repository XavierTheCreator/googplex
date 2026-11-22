import { useEffect, useState } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Check, Loader2, Plus, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import Input from './ui/Input.jsx'
import { errorText, ytAddToPlaylist, ytCreatePlaylist, ytMembership, ytPlaylists, ytRemoveFromPlaylist } from '../lib/api.js'

// "Save to…" like YouTube: a checkbox per playlist, ticked where the video
// already is. Ticking adds it, unticking removes it; or make a new playlist.
export default function SaveToPlaylistDialog({ video, onClose, notify }) {
  const [lists, setLists] = useState(null)
  const [member, setMember] = useState({}) // playlist id -> item id
  const [busy, setBusy] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!video) return
    setLists(null)
    setError('')
    setName('')
    Promise.all([ytPlaylists(), ytMembership(video.id)])
      .then(([p, m]) => { setLists(p.playlists || []); setMember(m || {}) })
      .catch((err) => setError(errorText(err)))
  }, [video])

  const toggle = async (p) => {
    setBusy(p.id)
    setError('')
    try {
      if (member[p.id]) {
        await ytRemoveFromPlaylist(p.id, member[p.id])
        setMember((m) => { const n = { ...m }; delete n[p.id]; return n })
        notify?.(`Removed from ${p.title}`)
      } else {
        const item = await ytAddToPlaylist(p.id, video.id)
        setMember((m) => ({ ...m, [p.id]: item }))
        notify?.(`Saved to ${p.title}`)
      }
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy('')
    }
  }

  const create = async () => {
    if (!name.trim()) return
    setBusy('new')
    setError('')
    try {
      const p = await ytCreatePlaylist(name.trim())
      const item = await ytAddToPlaylist(p.id, video.id)
      setLists((l) => [p, ...(l || [])])
      setMember((m) => ({ ...m, [p.id]: item }))
      setName('')
      notify?.(`Saved to ${p.title}`)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy('')
    }
  }

  return (
    <Dialog.Root open={!!video} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-[60] bg-black/40" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-[70] w-[min(380px,92vw)] -translate-x-1/2 -translate-y-1/2 rounded-xl border border-border bg-panel shadow-2xl focus:outline-none">
          <div className="flex items-center justify-between border-b border-border px-4 py-3">
            <Dialog.Title className="font-semibold">Save to…</Dialog.Title>
            <Dialog.Description className="sr-only">Choose playlists for this video.</Dialog.Description>
            <Dialog.Close className="grid h-7 w-7 place-items-center rounded-md text-muted hover:bg-raised hover:text-fg"><X size={16} /></Dialog.Close>
          </div>
          <div className="max-h-72 overflow-y-auto p-2">
            {!lists && !error && <div className="grid h-20 place-items-center text-muted"><Loader2 className="animate-spin" size={16} /></div>}
            {lists?.length === 0 && <p className="px-2 py-3 text-[13px] text-muted">No playlists yet.</p>}
            {lists?.map((p) => (
              <button key={p.id} onClick={() => toggle(p)} disabled={!!busy}
                className="flex w-full items-center gap-3 rounded-md px-2 py-2 text-left text-[14px] hover:bg-raised">
                <span className={`grid h-4 w-4 place-items-center rounded border ${member[p.id] ? 'border-accent bg-accent text-on-accent' : 'border-muted'}`}>
                  {busy === p.id ? <Loader2 className="animate-spin" size={10} /> : member[p.id] && <Check size={11} strokeWidth={3} />}
                </span>
                <span className="min-w-0 flex-1 truncate">{p.title}</span>
                <span className="text-[12px] text-muted">{p.privacy}</span>
              </button>
            ))}
          </div>
          <div className="flex gap-2 border-t border-border p-3">
            <Input value={name} onChange={(e) => setName(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && create()} placeholder="New playlist (private)" className="h-8" />
            <Button size="sm" onClick={create} disabled={!name.trim() || !!busy}><Plus size={14} /> Create</Button>
          </div>
          {error && <p className="selectable px-4 pb-3 text-[13px] text-danger">{error}</p>}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
