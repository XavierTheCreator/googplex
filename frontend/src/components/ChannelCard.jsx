import Card from './ui/Card.jsx'

const fmt = (n) => (n ? Number(n).toLocaleString() : '0')

export default function ChannelCard({ channel }) {
  if (!channel?.id) {
    return <Card className="p-5 text-muted">This Google account has no YouTube channel yet.</Card>
  }
  return (
    <Card className="flex items-center gap-4 p-5">
      <img src={channel.thumbnail} alt="" className="h-14 w-14 rounded-full" referrerPolicy="no-referrer" />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[16px] font-semibold">{channel.title}</div>
        {channel.handle && <div className="text-muted">{channel.handle}</div>}
      </div>
      <dl className="flex gap-8 text-right">
        <div><dt className="text-[12px] text-muted">Subscribers</dt><dd className="font-medium">{fmt(channel.subscribers)}</dd></div>
        <div><dt className="text-[12px] text-muted">Videos</dt><dd className="font-medium">{fmt(channel.videos)}</dd></div>
        <div><dt className="text-[12px] text-muted">Views</dt><dd className="font-medium">{fmt(channel.views)}</dd></div>
      </dl>
    </Card>
  )
}
