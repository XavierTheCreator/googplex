// Small display helpers shared by pages.

export function shortDate(value) {
  const d = new Date(value)
  if (isNaN(d)) return ''
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  }
  const sameYear = d.getFullYear() === now.getFullYear()
  return d.toLocaleDateString([], { month: 'short', day: 'numeric', ...(sameYear ? {} : { year: 'numeric' }) })
}

export function longDate(value) {
  const d = new Date(value)
  return isNaN(d) ? '' : d.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
}

export function timeAgo(value) {
  const secs = (Date.now() - new Date(value).getTime()) / 1000
  if (isNaN(secs)) return ''
  const steps = [[60, 'second'], [60, 'minute'], [24, 'hour'], [7, 'day'], [4.35, 'week'], [12, 'month'], [Infinity, 'year']]
  let n = secs
  for (const [size, unit] of steps) {
    if (n < size) {
      const v = Math.max(1, Math.floor(n))
      return `${v} ${unit}${v === 1 ? '' : 's'} ago`
    }
    n /= size
  }
  return ''
}

export function compactNumber(n) {
  return Intl.NumberFormat([], { notation: 'compact' }).format(n || 0)
}

// 253 -> "4:13", 3723 -> "1:02:03"; "" when unknown.
export function formatDuration(seconds) {
  if (!seconds) return ''
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = String(seconds % 60).padStart(2, '0')
  return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`
}
