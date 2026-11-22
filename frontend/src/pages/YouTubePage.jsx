import { useEffect, useState } from 'react'
import * as Tabs from '@radix-ui/react-tabs'
import PageHeader from '../components/PageHeader.jsx'
import FeatureGate from '../components/FeatureGate.jsx'
import ChannelCard from '../components/ChannelCard.jsx'
import SubscriptionGrid from '../components/SubscriptionGrid.jsx'
import VideoFeed from '../components/VideoFeed.jsx'
import NowPlaying from '../components/NowPlaying.jsx'
import SearchBox from '../components/SearchBox.jsx'
import SearchResults from '../components/SearchResults.jsx'
import { usePlayer } from '../components/PlayerProvider.jsx'
import VideoActionsProvider from '../components/VideoActions.jsx'
import ChannelView from '../components/ChannelView.jsx'
import { LikedTab, LocalListTab, PlaylistsTab } from '../components/YouTubeLibrary.jsx'
import { X } from 'lucide-react'
import { errorText, getYouTubeOverview, searchYouTube, ytAddRecentSearch, ytClearRecentSearches, ytRecentSearches, ytRemoveRecentSearch } from '../lib/api.js'

const tab = 'px-1 pb-2.5 text-muted data-[state=active]:text-fg data-[state=active]:border-b-2 data-[state=active]:border-accent -mb-px'

function Connected() {
  const [overview, setOverview] = useState(null)
  const [error, setError] = useState('')
  const { youtubeTab, setYoutubeTab, play, channelId } = usePlayer() // the tab outlives this page
  const [recent, setRecent] = useState([])
  const [input, setInput] = useState('')
  const [search, setSearch] = useState(null) // last search: { query, videos, nextPageToken, source, searchesToday }
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState('')

  useEffect(() => {
    getYouTubeOverview().then(setOverview).catch((err) => setError(errorText(err)))
    ytRecentSearches().then((r) => setRecent(r || []))
  }, [])

  const runSearch = async (query, more = false) => {
    setSearching(true)
    setSearchError('')
    setYoutubeTab('search')
    if (!more) {
      setSearch({ query, videos: [], nextPageToken: '' })
      setInput(query)
      ytAddRecentSearch(query).then((r) => setRecent(r || []))
    }
    try {
      const r = await searchYouTube(query, more ? search.nextPageToken : '')
      setSearch((prev) => {
        const seen = new Set(more ? prev.videos.map((v) => v.id) : [])
        const videos = [...(more ? prev.videos : []), ...(r.videos || []).filter((v) => !seen.has(v.id))]
        return { ...r, query, videos }
      })
    } catch (err) {
      setSearchError(errorText(err))
    } finally {
      setSearching(false)
    }
  }

  // The Search tab only exists once something has been searched.
  const current = youtubeTab === 'search' && !search ? 'feed' : youtubeTab

  return (
    <Tabs.Root value={current} onValueChange={setYoutubeTab}>
      <NowPlaying />
      {channelId ? <ChannelView id={channelId} /> : <>
      <div className="flex items-end justify-between gap-6 border-b border-border px-8 pt-3">
        <Tabs.List className="flex gap-6">
          <Tabs.Trigger className={tab} value="feed">Feed</Tabs.Trigger>
          {search && <Tabs.Trigger className={tab} value="search">Search</Tabs.Trigger>}
          <Tabs.Trigger className={tab} value="subscriptions">
            Subscriptions{overview && ` (${overview.subscriptions?.length || 0})`}
          </Tabs.Trigger>
          <Tabs.Trigger className={tab} value="playlists">Playlists</Tabs.Trigger>
          <Tabs.Trigger className={tab} value="liked">Liked</Tabs.Trigger>
          <Tabs.Trigger className={tab} value="later">Watch later</Tabs.Trigger>
          <Tabs.Trigger className={tab} value="history">History</Tabs.Trigger>
          <Tabs.Trigger className={tab} value="channel">Your channel</Tabs.Trigger>
        </Tabs.List>
        <div className="pb-2">
          <SearchBox value={input} onChange={setInput} onSubmit={(q) => runSearch(q)} placeholder="Search YouTube" />
        </div>
      </div>
      {recent.length > 0 && (current === 'search' || !search) && (
        <div className="flex flex-wrap items-center gap-1.5 px-8 pt-3 text-[12px]">
          <span className="text-muted">Recent:</span>
          {recent.map((q) => (
            <span key={q} className="flex items-center rounded-full border border-border">
              <button onClick={() => runSearch(q)} className="py-0.5 pl-2.5 pr-1 hover:text-fg">{q}</button>
              <button aria-label={`Remove ${q}`} onClick={() => ytRemoveRecentSearch(q).then((r) => setRecent(r || []))} className="pr-1.5 text-muted hover:text-fg"><X size={11} /></button>
            </span>
          ))}
          <button onClick={() => ytClearRecentSearches().then((r) => setRecent(r || []))} className="ml-1 text-muted hover:text-fg">Clear</button>
        </div>
      )}
      {error && <p className="selectable px-8 pt-4 text-danger">{error}</p>}
      <Tabs.Content value="feed" className="px-8 py-6"><VideoFeed /></Tabs.Content>
      <Tabs.Content value="search" className="px-8 py-6">
        {search && (
          <SearchResults search={search} loading={searching} error={searchError}
            onMore={() => runSearch(search.query, true)} onPlay={play} />
        )}
      </Tabs.Content>
      <Tabs.Content value="subscriptions" className="px-8 py-6">
        {overview && <SubscriptionGrid subscriptions={overview.subscriptions} />}
      </Tabs.Content>
      <Tabs.Content value="playlists" className="px-8 py-6"><PlaylistsTab /></Tabs.Content>
      <Tabs.Content value="liked" className="px-8 py-6"><LikedTab /></Tabs.Content>
      <Tabs.Content value="later" className="px-8 py-6"><LocalListTab kind="later" /></Tabs.Content>
      <Tabs.Content value="history" className="px-8 py-6"><LocalListTab kind="history" /></Tabs.Content>
      <Tabs.Content value="channel" className="px-8 py-6">
        {overview && <ChannelCard channel={overview.channel} />}
      </Tabs.Content>
      </>}
    </Tabs.Root>
  )
}

export default function YouTubePage() {
  return (
    <>
      <PageHeader title="YouTube" />
      <FeatureGate service="youtube" apiName="YouTube Data API v3" why="Your subscriptions and playlists come from the YouTube Data API.">
        <VideoActionsProvider><Connected /></VideoActionsProvider>
      </FeatureGate>
    </>
  )
}
