import { useEffect, useRef, useState } from 'react'
import { Undo2, X } from 'lucide-react'
import { pipClose, pipDock, pipEnded, pipPlayerUrl, pipReportTime } from '../lib/api.js'

const noDrag = { '--wails-draggable': 'no-drag' }

// The whole screen of the pop-out window: the video, plus a bar along the top
// that drags the window and holds Dock back and Close. The bar shows whenever
// the pointer is over the window (and for a few seconds after it opens).
export default function PipPage({ config }) {
  const [src, setSrc] = useState('')
  const [hint, setHint] = useState(true)
  const [over, setOver] = useState(false)
  const [embedError, setEmbedError] = useState('')
  const timeRef = useRef(config.start)

  useEffect(() => {
    if (config.stream) setSrc(`${config.stream}&autoplay=1&start=${config.start}`) // plays through yt-dlp
    else pipPlayerUrl(config.videoId).then((u) => setSrc(`${u}&autoplay=1&start=${config.start}`))
    const t = setTimeout(() => setHint(false), 3000)
    return () => clearTimeout(t)
  }, [config])

  useEffect(() => {
    const onMessage = (e) => {
      if (e.data?.source !== 'gp-player') return
      if (typeof e.data.time === 'number') {
        timeRef.current = e.data.time
        pipReportTime(e.data.time) // lets the main window's Bring back resume here
      } else if (String(e.data.msg).startsWith('error')) {
        setEmbedError(e.data.msg)
      } else if (e.data.msg === 'ended') {
        pipEnded() // the app's queue may send the next video here
      } else if (e.data.msg === 'embed_blocked') {
        setEmbedError(`embedding blocked, error ${e.data.code}`) // dock back: the main window switches to yt-dlp
      }
    }
    const onKey = (e) => { if (e.key === 'Escape') pipClose() }
    window.addEventListener('message', onMessage)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('message', onMessage)
      window.removeEventListener('keydown', onKey)
    }
  }, [])

  const showBar = hint || over || !!embedError

  return (
    // The iframe is a child, so moving onto the video doesn't count as leaving.
    <div className="relative h-full overflow-hidden bg-black" onMouseEnter={() => setOver(true)} onMouseLeave={() => setOver(false)}>
      {src && <iframe className="block h-full w-full border-0" src={src} title={config.title}
        allow="autoplay; encrypted-media; fullscreen" allowFullScreen />}
      {embedError && (
        <div className="absolute inset-0 grid place-items-center p-4 text-center text-[13px] text-white/70">
          YouTube won't play this video in a pop-out ({embedError}). Dock it back to play it through yt-dlp.
        </div>
      )}
      <div
        style={{ '--wails-draggable': 'drag' }}
        className={`absolute inset-x-0 top-0 flex h-10 cursor-grab items-center gap-1 bg-gradient-to-b from-black/90 to-black/0 pl-3 pr-1.5 text-white transition-opacity duration-200 ${showBar ? 'opacity-100' : 'opacity-0'}`}
      >
        <span className="min-w-0 flex-1 truncate text-[12px] font-medium">{config.title}</span>
        <button onClick={() => pipDock(timeRef.current)} style={noDrag} title="Back to the app"
          className="flex h-7 items-center gap-1.5 rounded px-2 text-[12px] text-white/85 hover:bg-white/15 hover:text-white">
          <Undo2 size={14} /> Dock back
        </button>
        <button onClick={pipClose} style={noDrag} title="Close" aria-label="Close"
          className="grid h-7 w-7 place-items-center rounded text-white/85 hover:bg-red-600 hover:text-white">
          <X size={16} />
        </button>
      </div>
    </div>
  )
}
