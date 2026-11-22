import { usePlayer } from './PlayerProvider.jsx'

// Your subscriptions; each opens the channel's page in the app.
export default function SubscriptionGrid({ subscriptions }) {
  const { openChannel } = usePlayer()
  if (!subscriptions?.length) return <p className="text-muted">No subscriptions yet.</p>
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-3">
      {subscriptions.map((s) => (
        <button
          key={s.channelId}
          onClick={() => openChannel(s.channelId)}
          className="flex items-center gap-2.5 rounded-md border border-border bg-panel p-2.5 text-left transition-colors hover:border-accent/40"
        >
          <img src={s.thumbnail} alt="" className="h-8 w-8 shrink-0 rounded-full" referrerPolicy="no-referrer" loading="lazy" />
          <span className="truncate text-[13px]">{s.title}</span>
        </button>
      ))}
    </div>
  )
}
