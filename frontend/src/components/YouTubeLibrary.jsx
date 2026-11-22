import { useCallback, useEffect, useState } from 'react'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { ListVideo, Loader2, MoreVertical, Play, Plus } from 'lucide-react'
import Button from './ui/Button.jsx'
import VideoGrid from './VideoGrid.jsx'
import PlaylistView from './PlaylistView.jsx'
import PromptDialog from './PromptDialog.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import {
  errorText, ytClearHistory, ytCreatePlaylist, ytDeletePlaylist, ytHistory, ytLiked, ytPlaylists, ytRemoveWatchLater, ytRenamePlaylist, ytWatchLater,
} from '../lib/api.js'

const item = 'flex cursor-default select-none items-center rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised'
const Spinner = () => <div className="grid h-40 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>

export function PlaylistGrid({ playlists, onOpen, onRename, onDelete }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-x-4 gap-y-6">
      {playlists.map((p) => (
        <div key={p.id} className="group">
          <button onClick={() => onOpen(p)} className="relative block w-full overflow-hidden rounded-lg border border-border bg-raised">
            {p.thumbnail ? <img src={p.thumbnail} alt="" referrerPolicy="no-referrer" className="aspect-video w-full object-cover" />
              : <div className="grid aspect-video place-items-center text-muted"><ListVideo size={28} /></div>}
            <span className="absolute inset-y-0 right-0 flex w-2/5 flex-col items-center justify-center bg-black/70 text-[13px] text-white">
              <ListVideo size={18} /> {p.itemCount}
            </span>
          </button>
          <div className="mt-2 flex gap-1">
            <button onClick={() => onOpen(p)} className="min-w-0 flex-1 text-left">
              <div className="truncate text-[13px] font-medium">{p.title}</div>
              <div className="text-[12px] capitalize text-muted">{p.privacy}</div>
            </button>
            {onRename && (
              <DropdownMenu.Root>
                <DropdownMenu.Trigger asChild>
                  <button aria-label="Playlist options" className="grid h-7 w-7 place-items-center rounded text-muted opacity-0 hover:bg-raised group-hover:opacity-100 data-[state=open]:opacity-100"><MoreVertical size={15} /></button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content align="end" className="z-50 min-w-36 rounded-lg border border-border bg-panel p-1 shadow-xl">
                    <DropdownMenu.Item className={item} onSelect={() => onRename(p)}>Rename</DropdownMenu.Item>
                    <DropdownMenu.Item className={`${item} text-danger`} onSelect={() => onDelete(p)}>Delete playlist</DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
            )}
          </div>
        </div>
      ))}
    </div>
  )
}

// Your playlists: create, rename, delete, and open one to play or rearrange it.
export function PlaylistsTab() {
  const [lists, setLists] = useState(null)
  const [open, setOpen] = useState(null)
  const [prompt, setPrompt] = useState(null)
  const [error, setError] = useState('')

  const load = useCallback(() => ytPlaylists().then((r) => setLists(r.playlists || [])).catch((err) => setError(errorText(err))), [])
  useEffect(() => { load() }, [load])

  if (open) return <PlaylistView playlist={open} editable onBack={() => { setOpen(null); load() }} />

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <span className="text-[13px] text-muted">Your playlists</span>
        <Button size="sm" onClick={() => setPrompt({ title: 'New playlist', input: '', message: 'New playlists are private.', action: 'Create',
          run: async (t) => { await ytCreatePlaylist(t); await load() } })}><Plus size={14} /> New playlist</Button>
      </div>
      {error && <p className="selectable mb-3 text-danger">{error}</p>}
      {!lists ? <Spinner /> : lists.length === 0 ? <p className="py-12 text-center text-muted">No playlists yet.</p> : (
        <PlaylistGrid playlists={lists} onOpen={setOpen}
          onRename={(p) => setPrompt({ title: 'Rename playlist', input: p.title, action: 'Rename', run: async (t) => { await ytRenamePlaylist(p.id, t, p.description); await load() } })}
          onDelete={(p) => setPrompt({ title: `Delete “${p.title}”?`, message: 'This deletes the playlist on YouTube. The videos themselves stay.', action: 'Delete playlist', danger: true,
            run: async () => { await ytDeletePlaylist(p.id); await load() } })} />
      )}
      <PromptDialog request={prompt} onClose={() => setPrompt(null)} />
    </div>
  )
}

export function LikedTab() {
  const { playList } = usePlayer()
  const [videos, setVideos] = useState(null)
  const [next, setNext] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async (token = '') => {
    setLoading(true)
    try {
      const r = await ytLiked(token)
      setVideos((v) => (token ? [...(v || []), ...r.videos] : r.videos))
      setNext(r.nextPageToken || '')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { load() }, [])

  if (error) return <p className="selectable text-danger">{error}</p>
  if (!videos) return <Spinner />
  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <span className="text-[13px] text-muted">Videos you liked</span>
        <Button size="sm" onClick={() => playList(videos, 0)} disabled={!videos.length}><Play size={14} /> Play all</Button>
      </div>
      <VideoGrid videos={videos} onPlay={(v, i) => playList(videos, i)} />
      {next && <div className="mt-8 flex justify-center"><Button onClick={() => load(next)} disabled={loading}>{loading && <Loader2 className="animate-spin" size={14} />} Load more</Button></div>}
    </div>
  )
}

// Watch later and History are kept by the app: YouTube's API doesn't offer them.
export function LocalListTab({ kind }) {
  const { playList } = usePlayer()
  const [videos, setVideos] = useState(null)
  const load = useCallback(() => (kind === 'later' ? ytWatchLater() : ytHistory()).then((v) => setVideos(v || [])), [kind])
  useEffect(() => { load() }, [load])

  if (!videos) return <Spinner />
  const later = kind === 'later'
  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <p className="min-w-0 flex-1 text-[13px] text-muted">
          {later ? "YouTube doesn't let apps read your Watch later list, so Googleplex keeps its own here."
            : "YouTube doesn't let apps read your watch history, so this lists what you've played in Googleplex."}
        </p>
        {later && videos.length > 0 && <Button size="sm" onClick={() => playList(videos, 0)}><Play size={14} /> Play all</Button>}
        {!later && videos.length > 0 && <Button size="sm" variant="ghost" onClick={async () => { await ytClearHistory(); load() }}>Clear history</Button>}
      </div>
      {videos.length === 0 ? <p className="py-12 text-center text-muted">{later ? 'Use a video’s ⋯ menu to save it here.' : 'Nothing played yet.'}</p> : (
        <VideoGrid videos={videos} onPlay={(v, i) => playList(videos, i)}
          extra={later ? (v) => [['Remove from Watch later', async () => { await ytRemoveWatchLater(v.id); load() }]] : undefined} />
      )}
    </div>
  )
}
