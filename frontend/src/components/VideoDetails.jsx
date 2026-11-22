import { useEffect, useState } from 'react'
import { Bell, BellOff, ChevronDown, ChevronUp, Loader2, ThumbsDown, ThumbsUp } from 'lucide-react'
import Button from './ui/Button.jsx'
import LinkedText from './LinkedText.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import { errorText, ytRate, ytSubscribe, ytSubscription, ytUnsubscribe, ytVideo } from '../lib/api.js'
import { compactNumber } from '../lib/format.js'

// Under the player: likes and your rating, Subscribe, and the description
// with links and timestamps that seek the player.
export default function VideoDetails({ video }) {
  const { command, openChannel } = usePlayer()
  const [info, setInfo] = useState(null)
  const [rating, setRating] = useState('none')
  const [sub, setSub] = useState(null) // subscription id, '' when not subscribed, null while loading
  const [more, setMore] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setInfo(null)
    setSub(null)
    setMore(false)
    setError('')
    ytVideo(video.id).then((v) => { setInfo(v); setRating(v.rating || 'none') }).catch((err) => setError(errorText(err)))
  }, [video.id])

  const channelId = info?.channelId || video.channelId
  useEffect(() => {
    if (channelId) ytSubscription(channelId).then(setSub).catch(() => setSub(''))
  }, [channelId])

  const rate = async (r) => {
    const next = rating === r ? 'none' : r
    const before = rating
    setRating(next)
    setInfo((i) => i && { ...i, likes: i.likes + (next === 'like' ? 1 : 0) - (before === 'like' ? 1 : 0) })
    try {
      await ytRate(video.id, next)
    } catch (err) {
      setRating(before)
      setError(errorText(err))
    }
  }

  const toggleSub = async () => {
    const before = sub
    setSub(before ? '' : 'pending')
    try {
      if (before) await ytUnsubscribe(channelId, before)
      else setSub(await ytSubscribe(channelId))
    } catch (err) {
      setSub(before)
      setError(errorText(err))
    }
  }

  const published = info?.publishedAt ? new Date(info.publishedAt).toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' }) : ''

  return (
    <div className="mt-3 space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        {channelId && (
          <button onClick={() => openChannel(channelId)} className="mr-1 font-medium hover:underline">{info?.channelTitle || video.channelTitle}</button>
        )}
        {channelId && (
          <Button size="sm" variant={sub ? 'secondary' : 'primary'} onClick={toggleSub} disabled={sub === null || sub === 'pending'}>
            {sub === null || sub === 'pending' ? <Loader2 className="animate-spin" size={14} /> : sub ? <BellOff size={14} /> : <Bell size={14} />}
            {sub && sub !== 'pending' ? 'Subscribed' : 'Subscribe'}
          </Button>
        )}
        <div className="flex-1" />
        <div className="flex overflow-hidden rounded-md border border-border">
          <button onClick={() => rate('like')} title="I like this" aria-pressed={rating === 'like'}
            className={`flex h-8 items-center gap-1.5 px-3 text-[13px] hover:bg-raised ${rating === 'like' ? 'text-accent' : ''}`}>
            <ThumbsUp size={15} className={rating === 'like' ? 'fill-accent/30' : ''} /> {info?.likes ? compactNumber(info.likes) : ''}
          </button>
          <button onClick={() => rate('dislike')} title="I dislike this" aria-pressed={rating === 'dislike'}
            className={`grid h-8 w-10 place-items-center border-l border-border hover:bg-raised ${rating === 'dislike' ? 'text-accent' : ''}`}>
            <ThumbsDown size={15} className={rating === 'dislike' ? 'fill-accent/30' : ''} />
          </button>
        </div>
      </div>
      {error && <p className="selectable text-[13px] text-danger">{error}</p>}
      {info && (
        <div className="selectable rounded-lg bg-raised/60 p-3 text-[13px]">
          <div className="font-medium">{[info.views && `${compactNumber(info.views)} views`, published].filter(Boolean).join(' · ')}</div>
          {info.description && (
            <div className={`mt-1 ${more ? '' : 'line-clamp-3'}`}>
              <LinkedText text={info.description} onSeek={(s) => command('seekTo', s)} />
            </div>
          )}
          {info.description?.length > 200 && (
            <button onClick={() => setMore((m) => !m)} className="mt-1 flex items-center gap-1 text-[12px] font-medium text-muted hover:text-fg">
              {more ? <>Show less <ChevronUp size={12} /></> : <>Show more <ChevronDown size={12} /></>}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
