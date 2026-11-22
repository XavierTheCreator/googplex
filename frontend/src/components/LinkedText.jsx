import { openUrl } from '../lib/api.js'

const pattern = /(https?:\/\/[^\s<>"]+)|(?<![\d:])((?:\d{1,2}:)?\d{1,2}:\d{2})(?![\d:])/g

export function toSeconds(stamp) {
  return stamp.split(':').map(Number).reduce((total, n) => total * 60 + n, 0)
}

// Plain text with web links that open in the browser and timestamps (1:23,
// 1:02:03) that seek the player. Never renders HTML from the text.
export default function LinkedText({ text, onSeek, className = '' }) {
  const parts = []
  let last = 0
  for (const m of (text || '').matchAll(pattern)) {
    if (m.index > last) parts.push(text.slice(last, m.index))
    if (m[1]) {
      const url = m[1].replace(/[.,)]+$/, '')
      parts.push(<a key={m.index} href={url} onClick={(e) => { e.preventDefault(); openUrl(url) }} className="text-accent hover:underline">{url}</a>)
      if (url.length < m[1].length) parts.push(m[1].slice(url.length))
    } else if (onSeek) {
      parts.push(<button key={m.index} onClick={() => onSeek(toSeconds(m[2]))} className="text-accent hover:underline">{m[2]}</button>)
    } else {
      parts.push(m[2])
    }
    last = m.index + m[0].length
  }
  if (last < (text || '').length) parts.push(text.slice(last))
  return <span className={`whitespace-pre-wrap break-words ${className}`}>{parts}</span>
}
