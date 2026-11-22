import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { MoreVertical } from 'lucide-react'
import { usePlayer } from './PlayerProvider.jsx'
import { useVideoActions } from './VideoActions.jsx'
import { compactNumber, formatDuration, timeAgo } from '../lib/format.js'

const item = 'flex cursor-default select-none items-center rounded px-2.5 py-1.5 text-[13px] outline-none data-[highlighted]:bg-raised'

// Bottom-right corner of the thumbnail, like YouTube: length, LIVE or UPCOMING.
function LengthBadge({ video }) {
  const base = 'absolute bottom-1.5 right-1.5 rounded px-1 py-px text-[12px] font-medium text-white'
  if (video.live === 'live') return <span className={`${base} bg-red-700`}>LIVE</span>
  if (video.live === 'upcoming') return <span className={`${base} bg-black/80`}>UPCOMING</span>
  const length = formatDuration(video.durationSeconds)
  return length ? <span className={`${base} bg-black/80 tabular-nums`}>{length}</span> : null
}

// extra: more menu items for where the card is shown, e.g. [['Remove from playlist', fn]].
export default function VideoCard({ video, onPlay, extra = [] }) {
  const player = usePlayer()
  const actions = useVideoActions()
  return (
    <div className="group text-left">
      <button onClick={() => onPlay(video)} className="block w-full">
        <div className="relative overflow-hidden rounded-lg border border-border bg-raised">
          <img src={video.thumbnail} alt="" loading="lazy" referrerPolicy="no-referrer"
            className="aspect-video w-full object-cover transition-transform duration-200 group-hover:scale-[1.03]" />
          <LengthBadge video={video} />
        </div>
      </button>
      <div className="mt-2 flex gap-1">
        <div className="min-w-0 flex-1">
          <button onClick={() => onPlay(video)} className="line-clamp-2 text-left text-[13px] font-medium leading-snug">{video.title}</button>
          {video.channelId && player?.openChannel ? (
            <button onClick={() => player.openChannel(video.channelId)} className="mt-1 block truncate text-left text-[12px] text-muted hover:text-fg">{video.channelTitle}</button>
          ) : <div className="mt-1 truncate text-[12px] text-muted">{video.channelTitle}</div>}
          <div className="text-[12px] text-muted">
            {[video.views > 0 && `${compactNumber(video.views)} views`, timeAgo(video.publishedAt)].filter(Boolean).join(' · ')}
          </div>
        </div>
        <DropdownMenu.Root>
          <DropdownMenu.Trigger asChild>
            <button aria-label="More" className="grid h-7 w-7 shrink-0 place-items-center rounded text-muted opacity-0 hover:bg-raised hover:text-fg group-hover:opacity-100 data-[state=open]:opacity-100">
              <MoreVertical size={15} />
            </button>
          </DropdownMenu.Trigger>
          <DropdownMenu.Portal>
            <DropdownMenu.Content align="end" className="z-50 min-w-48 rounded-lg border border-border bg-panel p-1 shadow-xl">
              {player && <DropdownMenu.Item className={item} onSelect={() => { player.playNext(video); actions?.notify('Plays next') }}>Play next</DropdownMenu.Item>}
              {player && <DropdownMenu.Item className={item} onSelect={() => { player.addToQueue(video); actions?.notify('Added to queue') }}>Add to queue</DropdownMenu.Item>}
              {actions && <DropdownMenu.Item className={item} onSelect={() => actions.saveToPlaylist(video)}>Save to playlist…</DropdownMenu.Item>}
              {actions && (
                <DropdownMenu.Item className={item} onSelect={() => actions.toggleWatchLater(video)}>
                  {actions.inWatchLater(video.id) ? 'Remove from Watch later' : 'Save to Watch later'}
                </DropdownMenu.Item>
              )}
              {video.channelId && player && <DropdownMenu.Item className={item} onSelect={() => player.openChannel(video.channelId)}>Go to channel</DropdownMenu.Item>}
              {extra.length > 0 && <DropdownMenu.Separator className="my-1 h-px bg-border" />}
              {extra.map(([label, fn]) => <DropdownMenu.Item key={label} className={item} onSelect={fn}>{label}</DropdownMenu.Item>)}
            </DropdownMenu.Content>
          </DropdownMenu.Portal>
        </DropdownMenu.Root>
      </div>
    </div>
  )
}
