// Gmail folders, search chips and small helpers shared by the Gmail screens.
import { sanitizeEmail } from './emailLinks.js'

// System folders, in Gmail's order. Snoozed isn't in the API, so it's absent.
export const folders = [
  { id: 'INBOX', name: 'Inbox' },
  { id: 'STARRED', name: 'Starred' },
  { id: 'IMPORTANT', name: 'Important' },
  { id: 'SENT', name: 'Sent' },
  { id: 'DRAFTS', name: 'Drafts' },
  { id: 'ALL', name: 'All Mail' },
  { id: 'SPAM', name: 'Spam' },
  { id: 'TRASH', name: 'Trash' },
]

export const categories = [
  { id: '', name: 'All' },
  { id: 'CATEGORY_PERSONAL', name: 'Primary' },
  { id: 'CATEGORY_SOCIAL', name: 'Social' },
  { id: 'CATEGORY_PROMOTIONS', name: 'Promotions' },
  { id: 'CATEGORY_UPDATES', name: 'Updates' },
  { id: 'CATEGORY_FORUMS', name: 'Forums' },
]

export const chips = [
  { id: 'unread', label: 'Unread', q: 'is:unread' },
  { id: 'attach', label: 'Has attachment', q: 'has:attachment' },
  { id: 'fromme', label: 'From me', q: 'from:me' },
  { id: 'week', label: 'Last 7 days', q: 'newer_than:7d' },
]

// The labels to list for a folder (and inbox category).
export function folderLabels(folderId, category) {
  if (folderId === 'ALL') return []
  if (folderId === 'INBOX' && category) return ['INBOX', category]
  return [folderId]
}

export function formatBytes(n) {
  if (!n) return ''
  if (n < 1024) return `${n} B`
  if (n < 1 << 20) return `${Math.round(n / 1024)} KB`
  return `${(n / (1 << 20)).toFixed(1)} MB`
}

export const MAX_ATTACH = 25 * 1024 * 1024

// Gmail's own links for things the app sends to the browser.
const hexToDec = (hex) => {
  try { return BigInt(`0x${hex}`).toString() } catch { return '' }
}
export const gmailWebLinks = {
  thread: (threadId) => `https://mail.google.com/mail/u/0/#all/${threadId}`,
  print: (threadId) => `https://mail.google.com/mail/u/0/?ui=2&view=pt&search=all&th=${threadId}`,
  original: (messageId) => `https://mail.google.com/mail/u/0/?ui=2&view=om&permmsgid=msg-f:${hexToDec(messageId)}`,
}

// HTML that may go into the compose editor, which sits in the app's own page:
// no scripts or handlers (sanitizeEmail), and no styles or links that would
// restyle the app.
export function sanitizeForEditor(html) {
  const clean = sanitizeEmail(html || '')
  const doc = new DOMParser().parseFromString(clean, 'text/html')
  doc.querySelectorAll('style, link, title').forEach((el) => el.remove())
  doc.querySelectorAll('[class]').forEach((el) => el.removeAttribute('class'))
  return doc.body.innerHTML
}

export const escapeHtml = (s) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c])

// Text to HTML for a plain draft body (keeps line breaks).
export const textToHtml = (t) => escapeHtml(t || '').replace(/\n/g, '<br>')
