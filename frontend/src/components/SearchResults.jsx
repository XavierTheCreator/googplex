import { Loader2 } from 'lucide-react'
import Button from './ui/Button.jsx'
import VideoGrid from './VideoGrid.jsx'

// search: { query, videos, nextPageToken, source, searchesToday } from the last search.
export default function SearchResults({ search, loading, error, onMore, onPlay }) {
  return (
    <div>
      <div className="mb-4">
        <h2 className="font-semibold">Results for “{search.query}”</h2>
        {search.videos.length > 0 && (
          <p className="mt-0.5 text-[13px] text-muted">
            {search.source === 'ytdlp'
              ? "Google's daily search limit is used up, so these results come from yt-dlp."
              : `${search.searchesToday} search${search.searchesToday === 1 ? '' : 'es'} used today`}
          </p>
        )}
      </div>
      {error && <p className="selectable mb-4 text-danger">{error}</p>}
      {loading && search.videos.length === 0 && (
        <div className="grid h-48 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>
      )}
      <VideoGrid videos={search.videos} onPlay={onPlay} />
      {!loading && !error && search.videos.length === 0 && <p className="py-12 text-center text-muted">No videos found.</p>}
      {search.nextPageToken && search.videos.length > 0 && (
        <div className="mt-8 flex justify-center">
          <Button onClick={onMore} disabled={loading}>
            {loading && <Loader2 className="animate-spin" size={14} />} Load more
          </Button>
        </div>
      )}
    </div>
  )
}
