import VideoCard from './VideoCard.jsx'

// onPlay(video, index); extra(video, index) may return more menu items per card.
export default function VideoGrid({ videos, onPlay, extra }) {
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-x-4 gap-y-6">
      {videos.map((v, i) => <VideoCard key={`${v.id}-${i}`} video={v} onPlay={(x) => onPlay(x, i)} extra={extra ? extra(v, i) : []} />)}
    </div>
  )
}
