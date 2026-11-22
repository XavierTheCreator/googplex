import { useCallback, useEffect, useState } from 'react'
import { Loader2, ThumbsUp, Trash2 } from 'lucide-react'
import Button from './ui/Button.jsx'
import Textarea from './ui/Textarea.jsx'
import LinkedText from './LinkedText.jsx'
import { usePlayer } from './PlayerProvider.jsx'
import { errorText, ytComments, ytDeleteComment, ytMyChannelId, ytPostComment, ytReplies, ytReply } from '../lib/api.js'
import { compactNumber, timeAgo } from '../lib/format.js'

function CommentBody({ c, mine, onDelete, onReply, onSeek }) {
  return (
    <div className="flex gap-3">
      {c.authorAvatar ? <img src={c.authorAvatar} alt="" referrerPolicy="no-referrer" className="h-8 w-8 shrink-0 rounded-full" />
        : <span className="h-8 w-8 shrink-0 rounded-full bg-raised" />}
      <div className="min-w-0 flex-1">
        <div className="text-[12px]"><span className="font-medium">{c.author}</span> <span className="text-muted">{timeAgo(c.publishedAt)}{c.edited && ' (edited)'}</span></div>
        <div className="selectable mt-0.5 text-[13px]"><LinkedText text={c.text} onSeek={onSeek} /></div>
        <div className="mt-1 flex items-center gap-3 text-[12px] text-muted">
          {c.likes > 0 && <span className="flex items-center gap-1"><ThumbsUp size={12} /> {compactNumber(c.likes)}</span>}
          {onReply && <button onClick={onReply} className="hover:text-fg">Reply</button>}
          {mine && <button onClick={onDelete} className="flex items-center gap-1 hover:text-danger"><Trash2 size={12} /> Delete</button>}
        </div>
      </div>
    </div>
  )
}

function Thread({ thread, me, onDeleted, onSeek }) {
  const [replies, setReplies] = useState(thread.replies || [])
  const [next, setNext] = useState(null) // null: not loaded through comments.list yet
  const [open, setOpen] = useState(false)
  const [replying, setReplying] = useState(false)
  const [text, setText] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const load = async (token = '') => {
    setBusy(true)
    try {
      const r = await ytReplies(thread.top.id, token)
      setReplies((x) => (token ? [...x, ...r.replies] : r.replies))
      setNext(r.nextPageToken || '')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const send = async () => {
    setBusy(true)
    setError('')
    try {
      const c = await ytReply(thread.top.id, text)
      setReplies((x) => [...x, c])
      setOpen(true)
      setText('')
      setReplying(false)
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  const remove = async (c, isTop) => {
    try {
      await ytDeleteComment(c.id)
      if (isTop) onDeleted(thread.id)
      else setReplies((x) => x.filter((r) => r.id !== c.id))
    } catch (err) {
      setError(errorText(err))
    }
  }

  const count = Math.max(thread.replyCount, replies.length)
  return (
    <li className="space-y-2">
      <CommentBody c={thread.top} mine={me && thread.top.authorChannelId === me} onDelete={() => remove(thread.top, true)}
        onReply={() => setReplying((r) => !r)} onSeek={onSeek} />
      <div className="space-y-2 pl-11">
        {replying && (
          <div className="flex gap-2">
            <Textarea value={text} onChange={(e) => setText(e.target.value)} placeholder="Add a reply" className="h-16 text-[13px]" autoFocus />
            <Button size="sm" variant="primary" onClick={send} disabled={busy || !text.trim()}>Reply</Button>
          </div>
        )}
        {count > 0 && (
          <button onClick={() => { setOpen((o) => !o); if (!open && next === null) load() }} className="text-[12px] font-medium text-accent">
            {open ? 'Hide replies' : `${count} ${count === 1 ? 'reply' : 'replies'}`}
          </button>
        )}
        {open && replies.map((r) => (
          <CommentBody key={r.id} c={r} mine={me && r.authorChannelId === me} onDelete={() => remove(r, false)} onSeek={onSeek} />
        ))}
        {open && next && <button onClick={() => load(next)} className="text-[12px] text-muted hover:text-fg">{busy ? 'Loading…' : 'More replies'}</button>}
        {error && <p className="selectable text-[12px] text-danger">{error}</p>}
      </div>
    </li>
  )
}

// Comments under the player: sort, load more, post, reply and delete your own.
// Text is shown as plain text; links open in the browser, timestamps seek.
export default function Comments({ videoId }) {
  const { command } = usePlayer()
  const [order, setOrder] = useState('relevance')
  const [threads, setThreads] = useState([])
  const [next, setNext] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [text, setText] = useState('')
  const [posting, setPosting] = useState(false)
  const [me, setMe] = useState('')

  useEffect(() => { ytMyChannelId().then(setMe) }, [])

  const load = useCallback(async (token = '') => {
    setLoading(true)
    setError('')
    try {
      const r = await ytComments(videoId, order, token)
      setThreads((t) => (token ? [...t, ...r.threads] : r.threads))
      setNext(r.nextPageToken || '')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setLoading(false)
    }
  }, [videoId, order])

  useEffect(() => {
    setThreads([])
    load()
  }, [load])

  const post = async () => {
    setPosting(true)
    setError('')
    try {
      const t = await ytPostComment(videoId, text)
      setThreads((x) => [t, ...x])
      setText('')
    } catch (err) {
      setError(errorText(err))
    } finally {
      setPosting(false)
    }
  }

  return (
    <section className="mt-6">
      <div className="mb-3 flex items-center gap-4">
        <h3 className="font-semibold">Comments</h3>
        <div className="flex rounded-md bg-raised p-0.5 text-[12px]">
          {[['relevance', 'Top'], ['time', 'Newest']].map(([id, label]) => (
            <button key={id} onClick={() => setOrder(id)} className={`h-6 rounded px-2 ${order === id ? 'bg-panel text-fg shadow-sm' : 'text-muted'}`}>{label}</button>
          ))}
        </div>
      </div>
      <div className="mb-4 flex gap-2">
        <Textarea value={text} onChange={(e) => setText(e.target.value)} placeholder="Add a comment" className="h-16 text-[13px]" />
        <Button size="sm" variant="primary" onClick={post} disabled={posting || !text.trim()}>
          {posting && <Loader2 className="animate-spin" size={14} />} Comment
        </Button>
      </div>
      {error && <p className="selectable mb-3 text-[13px] text-muted">{error}</p>}
      <ul className="space-y-4">
        {threads.map((t) => (
          <Thread key={t.id} thread={t} me={me} onSeek={(s) => command('seekTo', s)} onDeleted={(id) => setThreads((x) => x.filter((y) => y.id !== id))} />
        ))}
      </ul>
      {loading && <div className="grid h-12 place-items-center text-muted"><Loader2 className="animate-spin" size={15} /></div>}
      {next && !loading && <div className="mt-4"><Button size="sm" onClick={() => load(next)}>More comments</Button></div>}
    </section>
  )
}
