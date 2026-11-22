import { useEffect, useRef, useState } from 'react'
import { Download, Loader2, PictureInPicture2, Undo2, X } from 'lucide-react'
import Button from './ui/Button.jsx'
import Badge from './ui/Badge.jsx'
import Tooltip from './ui/Tooltip.jsx'
import ExternalLink from './ExternalLink.jsx'
import VideoDetails from './VideoDetails.jsx'
import Comments from './Comments.jsx'
import UpNext from './UpNext.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import { downloadVideo, errorText, playerUrl, streamUrl } from '../lib/api.js'
import { timeAgo } from '../lib/format.js'

const playerErrors = {
  'error 2': 'YouTube says this video link is invalid.',
  'error 100': 'This video was removed or made private.',
  'error stream': "The yt-dlp copy of this video wouldn't play.",
}

// The player at the top of the YouTube page (above its tabs), or a note while
// the video is playing in the pop-out window. Videos whose owners block
// embedding (YouTube errors 101 and 150) play through yt-dlp instead.
export default function NowPlaying() {
  const { video, mode, startAt, error, streams, setStream, currentTime, popOut, leftFeed, bringBack, closePopOut, close } = usePlayer()
  const ref = useRef(null)
  const [src, setSrc] = useState('')
  const [embedError, setEmbedError] = useState('')
  const [resolving, setResolving] = useState(false)
  const [streamError, setStreamError] = useState('')
  const [showComments, setShowComments] = useState(false)
  const [audioTrouble, setAudioTrouble] = useState(false) // the separate audio kept stalling

  // Leaving the YouTube page moves the video to the pop-out.
  useEffect(() => () => leftFeed(), []) // eslint-disable-line react-hooks/exhaustive-deps

  const docked = !!video && mode === 'docked'
  const stream = video ? streams[video.id] : ''

  useEffect(() => {
    setEmbedError('')
    setStreamError('')
    setAudioTrouble(false)
    if (!docked) return setSrc('')
    if (stream) return setSrc(`${stream}&autoplay=1&start=${startAt}`) // already known to block embedding
    setSrc('')
    playerUrl(video.id).then((u) => setSrc(`${u}&autoplay=1&start=${startAt}`))
  }, [docked, video?.id, startAt]) // eslint-disable-line react-hooks/exhaustive-deps -- `stream` is read, not watched: switching to it mid-play sets src itself

  useEffect(() => {
    if (docked) ref.current?.scrollIntoView({ block: 'start', behavior: 'smooth' })
  }, [docked, video?.id])

  useEffect(() => {
    const onMessage = async (e) => {
      if (e.data?.source !== 'gp-player') return
      const msg = String(e.data.msg)
      if (msg === 'audio_trouble') {
        setAudioTrouble(true)
      } else if (msg === 'embed_blocked' && e.data.videoId) {
        const id = e.data.videoId
        setResolving(true)
        setStreamError('')
        try {
          const url = await streamUrl(id)
          setSrc(`${url}&autoplay=1&start=${Math.floor(currentTime())}`)
          setStream(id, url)
        } catch (err) {
          setStreamError(errorText(err))
        } finally {
          setResolving(false)
        }
      } else if (msg.startsWith('error')) {
        setEmbedError(msg)
      }
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  if (!video) return null

  const watchUrl = `https://www.youtube.com/watch?v=${video.id}`

  if (mode === 'popout') {
    return (
      <section className="mx-8 mt-6 flex max-w-[960px] items-center gap-4 rounded-lg border border-border bg-panel p-4">
        <PictureInPicture2 className="shrink-0 text-accent" size={20} />
        <div className="min-w-0 flex-1">
          <div className="text-[13px] text-muted">Playing in a pop-out window</div>
          <div className="truncate font-medium">{video.title}</div>
        </div>
        <Button size="sm" onClick={bringBack}><Undo2 size={14} /> Bring back</Button>
        <Button size="sm" variant="ghost" onClick={closePopOut}><X size={14} /> Close pop-out</Button>
      </section>
    )
  }

  const overlay = (children) => (
    <div className="absolute inset-0 grid place-items-center bg-black p-6 text-center text-[14px] text-white/75">{children}</div>
  )

  return (
    <section ref={ref} className="mx-8 mt-6 grid max-w-[1320px] scroll-mt-6 grid-cols-1 gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
     <div className="min-w-0 max-w-[960px]">
      <div id="youtube-player-slot" className="relative aspect-video w-full overflow-hidden rounded-lg bg-black">
        {src && !embedError && !streamError && (
          <iframe key={src} className="h-full w-full" src={src} title={video.title}
            allow="autoplay; encrypted-media; picture-in-picture; fullscreen" allowFullScreen />
        )}
        {resolving && !stream && overlay(
          <div className="flex items-center gap-2.5">
            <Loader2 className="animate-spin" size={16} /> This video blocks embedding, loading it through yt-dlp…
          </div>,
        )}
        {streamError && overlay(
          <div className="max-w-md">
            <p>This video's owner doesn't allow it to play outside YouTube, and yt-dlp couldn't get a copy either.</p>
            <p className="selectable mt-2 text-[12px] text-white/50">{streamError}</p>
            <div className="mt-4 flex justify-center gap-2">
              <ExternalLink href={watchUrl} variant="primary">Open on YouTube</ExternalLink>
              <Button onClick={() => downloadVideo(video.id, video.title)}><Download size={14} /> Download</Button>
            </div>
          </div>,
        )}
        {embedError && overlay(
          <div>
            <p>{playerErrors[embedError] || `YouTube won't play this video inside the app (${embedError}).`}</p>
            <div className="mt-3"><ExternalLink href={watchUrl} variant="primary">Open on YouTube</ExternalLink></div>
          </div>,
        )}
      </div>
      <div className="mt-3 flex items-start gap-4">
        <div className="min-w-0 flex-1">
          <h2 className="font-semibold leading-snug">{video.title}</h2>
          <p className="mt-1 flex items-center gap-2 text-[13px] text-muted">
            {timeAgo(video.publishedAt)}
            {stream && (
              <Tooltip label="The owner blocks this video outside YouTube, so Googleplex plays it through yt-dlp instead (up to 1080p).">
                <span><Badge>yt-dlp playback</Badge></span>
              </Tooltip>
            )}
          </p>
          {audioTrouble && (
            <p className="mt-1 text-[12px] text-warning">
              The audio keeps stalling on this connection. Open on YouTube or Download should play it smoothly.
            </p>
          )}
        </div>
        <div className="flex shrink-0 gap-2">
          <Button size="sm" onClick={popOut} disabled={resolving}><PictureInPicture2 size={14} /> Pop out</Button>
          <Button size="sm" onClick={() => downloadVideo(video.id, video.title)}><Download size={14} /> Download</Button>
          <ExternalLink href={watchUrl} size="sm">Open on YouTube</ExternalLink>
          <Button size="sm" variant="ghost" onClick={close}><X size={14} /> Close</Button>
        </div>
      </div>
      {error && <p className="selectable mt-2 text-[13px] text-danger">{error}</p>}
      <VideoDetails video={video} />
      <button onClick={() => setShowComments((c) => !c)} className="mt-3 text-[13px] font-medium text-muted hover:text-fg">
        {showComments ? 'Hide comments' : 'Show comments'}
      </button>
      {showComments && <Comments videoId={video.id} />}
     </div>
      <UpNext />
    </section>
  )
}
