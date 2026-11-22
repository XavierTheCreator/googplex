// What kind of file a Drive mime type is, and how to show sizes and dates.

const google = 'application/vnd.google-apps.'

export function fileKind(mimeType = '') {
  if (mimeType === `${google}folder`) return 'folder'
  if (mimeType === `${google}document`) return 'doc'
  if (mimeType === `${google}spreadsheet`) return 'sheet'
  if (mimeType === `${google}presentation`) return 'slides'
  if (mimeType.startsWith(google)) return 'google'
  if (mimeType === 'application/pdf') return 'pdf'
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  if (/zip|compressed|tar|rar|7z/.test(mimeType)) return 'archive'
  if (mimeType.startsWith('text/')) return 'text'
  return 'other'
}

export const isGoogleFile = (mimeType = '') => mimeType.startsWith(google) && mimeType !== `${google}folder`

// Images and PDFs can be shown in the app; everything else opens elsewhere.
export const canPreview = (f) => ['image', 'pdf'].includes(fileKind(f.mimeType))

export function formatSize(bytes) {
  const n = Number(bytes)
  if (!bytes || !Number.isFinite(n)) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v < 10 && i > 0 ? v.toFixed(1) : Math.round(v)} ${units[i]}`
}

export function modifiedLabel(iso) {
  const d = new Date(iso)
  if (isNaN(d)) return ''
  const now = new Date()
  if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
  return d.toLocaleDateString([], { month: 'short', day: 'numeric', ...(d.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' }) })
}

export const ownerOf = (f) => f.owners?.[0]?.displayName || ''

// Sorts folders first, then by the chosen column.
export function sortFiles(files, by, desc) {
  const key = {
    name: (f) => f.name.toLowerCase(),
    modified: (f) => f.modifiedTime || '',
    size: (f) => Number(f.size) || 0,
    owner: (f) => ownerOf(f).toLowerCase(),
  }[by] || ((f) => f.name.toLowerCase())
  return [...files].sort((a, b) => {
    const fa = fileKind(a.mimeType) === 'folder'
    const fb = fileKind(b.mimeType) === 'folder'
    if (fa !== fb) return fa ? -1 : 1
    const x = key(a)
    const y = key(b)
    const c = x < y ? -1 : x > y ? 1 : 0
    return desc ? -c : c
  })
}
