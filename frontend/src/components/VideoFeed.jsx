import { useCallback, useEffect, useState } from 'react'
import { Loader2, RefreshCw } from 'lucide-react'
import VideoGrid from './VideoGrid.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import Button from './ui/Button.jsx'
import { errorText, getYouTubeFeed, refreshYouTubeFeed } from '../lib/api.js'

export default function VideoFeed() {
  const [videos, setVideos] = useState([])
  const [failed, setFailed] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const { play } = usePlayer()

  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const r = await refreshYouTubeFeed()
      setVideos(r.videos || [])
      setFailed(r.failed)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    getYouTubeFeed().then((r) => setVideos(r.videos || [])) // cached feed first
    refresh()
  }, [refresh])

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <span className="text-[13px] text-muted">
          Latest from your subscriptions{failed > 0 && ` · ${failed} channel${failed === 1 ? '' : 's'} could not be loaded`}
        </span>
        <Button size="sm" variant="ghost" onClick={refresh} disabled={loading}>
          {loading ? <Loader2 className="animate-spin" size={14} /> : <RefreshCw size={14} />} Refresh
        </Button>
      </div>
      {error && <p className="selectable mb-4 text-danger">{error}</p>}
      <VideoGrid videos={videos} onPlay={play} />
      {!loading && videos.length === 0 && !error && <p className="py-12 text-center text-muted">No videos yet.</p>}
    </div>
  )
}
