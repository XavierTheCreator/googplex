import { useEffect, useState } from 'react'
import { ArrowLeft, Bell, BellOff, Loader2 } from 'lucide-react'
import Button from './ui/Button.jsx'
import LinkedText from './LinkedText.jsx'
import PlaylistView from './PlaylistView.jsx'
import { PlaylistGrid } from './YouTubeLibrary.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import { errorText, ytChannel, ytPlaylists, ytSubscribe, ytUnsubscribe } from '../lib/api.js'
import { compactNumber } from '../lib/format.js'

// A channel's page: banner, avatar, subscribers, Subscribe, its uploads (from
// the uploads playlist, 1 quota unit a page, never search) and its playlists.
export default function ChannelView({ id }) {
  const { closeChannel } = usePlayer()
  const [page, setPage] = useState(null)
  const [sub, setSub] = useState('')
  const [tab, setTab] = useState('uploads')
  const [playlists, setPlaylists] = useState(null)
  const [openList, setOpenList] = useState(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    setPage(null)
    setTab('uploads')
    setPlaylists(null)
    setOpenList(null)
    setError('')
    ytChannel(id).then((p) => { setPage(p); setSub(p.subscriptionId || '') }).catch((err) => setError(errorText(err)))
  }, [id])

  useEffect(() => {
    if (tab === 'playlists' && !playlists) ytPlaylists(id).then((r) => setPlaylists(r.playlists || [])).catch((err) => setError(errorText(err)))
  }, [tab, id, playlists])

  const toggle = async () => {
    const before = sub
    setBusy(true)
    try {
      if (before) { setSub(''); await ytUnsubscribe(id, before) } else setSub(await ytSubscribe(id))
    } catch (err) {
      setSub(before)
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  if (error && !page) return <div className="px-8 py-6"><Button size="sm" variant="ghost" onClick={closeChannel}><ArrowLeft size={14} /> Back</Button><p className="selectable mt-4 text-danger">{error}</p></div>
  if (!page) return <div className="grid h-60 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>
  const c = page.channel
  const tabCls = (t) => `-mb-px border-b-2 pb-2.5 ${tab === t ? 'border-accent text-fg' : 'border-transparent text-muted hover:text-fg'}`

  return (
    <div>
      {c.banner && <img src={c.banner} alt="" referrerPolicy="no-referrer" className="h-36 w-full object-cover" />}
      <div className="px-8 pt-4">
        <Button size="sm" variant="ghost" onClick={closeChannel}><ArrowLeft size={14} /> Back</Button>
        <div className="mt-3 flex items-center gap-4">
          {c.avatar && <img src={c.avatar} alt="" referrerPolicy="no-referrer" className="h-20 w-20 rounded-full" />}
          <div className="min-w-0 flex-1">
            <h2 className="text-[22px] font-semibold">{c.title}</h2>
            <p className="text-[13px] text-muted">
              {[c.handle, !c.hiddenSubscriberCount && `${compactNumber(c.subscribers)} subscribers`, `${compactNumber(c.videos)} videos`].filter(Boolean).join(' · ')}
            </p>
            {c.description && <p className="selectable mt-1 line-clamp-2 max-w-3xl text-[13px] text-muted"><LinkedText text={c.description} /></p>}
          </div>
          <Button variant={sub ? 'secondary' : 'primary'} onClick={toggle} disabled={busy}>
            {busy ? <Loader2 className="animate-spin" size={14} /> : sub ? <BellOff size={14} /> : <Bell size={14} />} {sub ? 'Subscribed' : 'Subscribe'}
          </Button>
        </div>
        {error && <p className="selectable mt-2 text-[13px] text-danger">{error}</p>}
        <div className="mt-5 flex gap-6 border-b border-border text-[14px]">
          <button className={tabCls('uploads')} onClick={() => { setTab('uploads'); setOpenList(null) }}>Videos</button>
          <button className={tabCls('playlists')} onClick={() => { setTab('playlists'); setOpenList(null) }}>Playlists</button>
        </div>
      </div>
      <div className="px-8 py-6">
        {tab === 'uploads' && c.uploads && <PlaylistView playlist={{ id: c.uploads, title: 'Uploads', itemCount: c.videos }} />}
        {tab === 'playlists' && (openList ? <PlaylistView playlist={openList} onBack={() => setOpenList(null)} />
          : !playlists ? <div className="grid h-40 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>
            : playlists.length === 0 ? <p className="py-10 text-center text-muted">No public playlists.</p>
              : <PlaylistGrid playlists={playlists} onOpen={setOpenList} />)}
      </div>
    </div>
  )
}
