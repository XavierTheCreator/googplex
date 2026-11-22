import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import {
  bringBack as apiBringBack, closePopOut as apiClosePopOut, errorText, onPipClosed, onPipDocked, onPipEnded, popOut as apiPopOut, ytAddHistory,
} from '../lib/api.js'
import { advance, enqueue, move, playFrom, removeAt } from '../lib/queue.js'

const PlayerContext = createContext(null)

export const usePlayer = () => useContext(PlayerContext)

const AUTOPLAY_KEY = 'gp.autoplay'
const savedAutoplay = () => {
  try { return localStorage.getItem(AUTOPLAY_KEY) !== 'off' } catch { return true }
}
const typing = () => {
  const el = document.activeElement
  return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}
const playerFrame = () => document.querySelector('#youtube-player-slot iframe')

// Holds what is playing and where (docked in the YouTube page, or in the
// pop-out window), the Up next queue with autoplay, and the channel page
// being shown. Every play is added to the app's own watch history.
export default function PlayerProvider({ onShowYouTube, children }) {
  const [video, setVideo] = useState(null)
  const [mode, setMode] = useState('docked') // docked | popout
  const [startAt, setStartAt] = useState(0) // seconds the docked player starts from
  const [youtubeTab, setYoutubeTab] = useState('feed')
  const [channelId, setChannelId] = useState('') // a channel page open in the YouTube section
  const [queue, setQueue] = useState([])
  const [autoplay, setAutoplayState] = useState(savedAutoplay)
  const [error, setError] = useState('')
  // Videos that block embedding, for this session: id -> yt-dlp stream page.
  const [streams, setStreams] = useState({})
  const streamsRef = useRef({})
  streamsRef.current = streams

  // Refs so the event handlers below always see the latest values.
  const timeRef = useRef(0)
  const videoRef = useRef(null)
  const modeRef = useRef('docked')
  const queueRef = useRef([])
  const autoplayRef = useRef(autoplay)
  videoRef.current = video
  modeRef.current = mode
  queueRef.current = queue
  autoplayRef.current = autoplay

  const setAutoplay = (on) => {
    setAutoplayState(on)
    try { localStorage.setItem(AUTOPLAY_KEY, on ? 'on' : 'off') } catch { /* not saved; fine */ }
  }

  const remember = (v) => ytAddHistory({ ...v, views: v.views || 0, duration: v.duration || v.durationSeconds || 0 }).catch(() => {})

  const play = useCallback((v) => {
    if (modeRef.current === 'popout') apiClosePopOut()
    timeRef.current = 0
    setError('')
    setStartAt(0)
    setVideo(v)
    setMode('docked')
    remember(v)
  }, [])

  const popOut = useCallback(async (v = videoRef.current, start = Math.floor(timeRef.current)) => {
    if (!v) return
    setError('')
    try {
      await apiPopOut(v.id, v.title, start, streamsRef.current[v.id] || '')
      setMode('popout')
    } catch (err) {
      setError(errorText(err))
    }
  }, [])

  // Plays the next queued video, where the current one is playing.
  const next = useCallback(() => {
    const { next: n, queue: rest } = advance(queueRef.current)
    if (!n) return false
    setQueue(rest)
    if (modeRef.current === 'popout') {
      setVideo(n)
      remember(n)
      popOut(n, 0) // replaces the pop-out with the next video, in the same spot
    } else play(n)
    return true
  }, [play, popOut])

  // The docked player reports its time every 2 seconds, and when it ends.
  useEffect(() => {
    const onMessage = (e) => {
      if (e.data?.source !== 'gp-player') return
      if (typeof e.data.time === 'number') timeRef.current = e.data.time
      if (e.data.msg === 'ended' && autoplayRef.current && modeRef.current === 'docked') next()
    }
    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, [next])

  const dockAt = useCallback((t) => {
    timeRef.current = t
    setStartAt(Math.floor(t))
    setMode('docked')
    onShowYouTube()
  }, [onShowYouTube])

  useEffect(() => {
    const offDocked = onPipDocked(({ videoId, time }) => {
      if (videoRef.current?.id === videoId) dockAt(time)
    })
    const offClosed = onPipClosed(() => {
      setVideo(null)
      setMode('docked')
    })
    const offEnded = onPipEnded(() => { if (autoplayRef.current) next() })
    return () => {
      offDocked()
      offClosed()
      offEnded()
    }
  }, [dockAt, next])

  // Sends a command to the docked player page (play/pause, seek).
  const command = (cmd, seconds = 0) => playerFrame()?.contentWindow?.postMessage({ gp: 'cmd', cmd, seconds }, '*')

  // Keyboard, while the docked player is on screen: k or space play/pause,
  // j and l seek 10 s, f fullscreen, shift-N next.
  useEffect(() => {
    const onKey = (e) => {
      const frame = playerFrame()
      if (!frame || e.metaKey || e.ctrlKey || e.altKey || typing() || document.querySelector('[role="dialog"]')) return
      const k = e.key
      if (k === 'k' || k === ' ') command('toggle')
      else if (k === 'j') command('seekBy', -10)
      else if (k === 'l') command('seekBy', 10)
      else if (k === 'f') (frame.requestFullscreen || frame.webkitRequestFullscreen)?.call(frame)
      else if (k === 'N' && e.shiftKey) next()
      else return
      e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [next])

  const value = {
    video,
    mode,
    startAt,
    error,
    streams,
    setStream: (id, url) => setStreams((m) => ({ ...m, [id]: url })),
    currentTime: () => timeRef.current,
    youtubeTab,
    setYoutubeTab,
    channelId,
    openChannel: (id) => {
      setChannelId(id)
      onShowYouTube()
    },
    closeChannel: () => setChannelId(''),
    popOut: () => popOut(),
    play,
    command,
    // Queue
    queue,
    autoplay,
    setAutoplay,
    next,
    playNext: (v) => setQueue((q) => enqueue(q, v, 'next')),
    addToQueue: (v) => setQueue((q) => enqueue(q, v, 'end')),
    removeFromQueue: (i) => setQueue((q) => removeAt(q, i)),
    moveInQueue: (from, to) => setQueue((q) => move(q, from, to)),
    clearQueue: () => setQueue([]),
    // Play all / shuffle a list (a playlist, liked videos…), starting at index i.
    playList: (list, i = 0, shuffled = false) => {
      const { now, queue: rest } = playFrom(list, i, shuffled)
      if (!now) return
      setQueue(rest)
      play(now)
    },
    // Called when Now Playing leaves the screen (another page): keep playing in the pop-out.
    leftFeed: () => {
      if (videoRef.current && modeRef.current === 'docked') popOut()
    },
    bringBack: async () => dockAt(await apiBringBack()),
    closePopOut: () => {
      apiClosePopOut()
      setVideo(null)
      setMode('docked')
    },
    close: () => setVideo(null),
  }

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>
}
