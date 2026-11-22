import { useEffect, useMemo, useRef, useState } from 'react'
import { openUrl } from '../lib/api.js'
import { attachEmailHandlers, EMAIL_CSP, sanitizeEmail } from '../lib/emailLinks.js'
import { useTheme } from '../lib/theme.js'

// HTML emails are designed for white, so they always get a white card. Plain
// text emails follow the app's theme. (Colors match the app's tokens.)
const baseStyle = (dark) => `
  body { margin: 0; padding: 20px; font: 14px/1.5 -apple-system, "Segoe UI", Inter, sans-serif; word-wrap: break-word;
    color: ${dark ? '#e6e8eb' : '#111317'}; background: ${dark ? '#14161a' : '#ffffff'}; }
  img { max-width: 100%; height: auto; }
  pre { white-space: pre-wrap; font: inherit; margin: 0; }
  a { color: ${dark ? '#608cff' : '#2f62e6'}; }
`

// "Dark email": invert the whole email, then invert pictures back so they
// look right. A gentle hue-rotate keeps colors close to the originals.
const DARK_FILTER = `
  html { filter: invert(0.9) hue-rotate(180deg); background: #fff; }
  img, video, picture, svg image, [style*="background-image"] { filter: invert(1) hue-rotate(180deg); }
`

function escapeHtml(s) {
  return s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c])
}

// Renders an email body in an iframe. The email's own scripts are stripped
// and blocked by a content policy, and it can't submit forms or open windows;
// the app handles its links (see lib/emailLinks.js). The frame grows to fit.
//
// The sandbox allows scripts only because WebKit won't run the app's click
// listeners otherwise; see the note in lib/emailLinks.js.
export default function EmailFrame({ html, text, darkEmail = false, onMailto, onBlockedForm, onHover }) {
  const { dark } = useTheme()
  const textDark = !html && dark // plain text follows the app theme
  const ref = useRef(null)
  const cleanup = useRef(() => {})
  const handlers = useRef({})
  handlers.current = { onMailto, onBlockedForm, onHover }
  const [height, setHeight] = useState(200)

  const body = useMemo(() => (html ? sanitizeEmail(html) : `<pre>${escapeHtml(text || '(This message has no text.)')}</pre>`), [html, text])
  const srcDoc = `<!doctype html><html><head><meta charset="utf-8">
    <meta http-equiv="Content-Security-Policy" content="${EMAIL_CSP}">
    <style>${baseStyle(textDark)}</style></head><body>${body}</body></html>`

  // Wired from the iframe's own onLoad, which React attaches before the
  // document loads, so a fast load can't slip past it.
  const onLoad = () => {
    cleanup.current()
    const doc = ref.current?.contentDocument
    if (!doc?.body) return
    setFilter()
    const fit = () => setHeight(doc.documentElement.scrollHeight)
    fit()
    const observer = new ResizeObserver(fit)
    observer.observe(doc.body)
    const unwire = attachEmailHandlers(doc, {
      onWeb: (url) => openUrl(url),
      onMailto: (draft) => handlers.current.onMailto?.(draft),
      onBlockedForm: () => handlers.current.onBlockedForm?.(),
      onHover: (url) => handlers.current.onHover?.(url),
    })
    cleanup.current = () => {
      observer.disconnect()
      unwire()
      handlers.current.onHover?.('')
    }
  }

  useEffect(() => () => cleanup.current(), [])

  // The Dark email filter goes in and out without reloading the email.
  const setFilter = () => {
    const doc = ref.current?.contentDocument
    if (!doc?.head) return
    let st = doc.getElementById('gp-dark-email')
    if (darkEmail && html) {
      if (!st) {
        st = doc.createElement('style')
        st.id = 'gp-dark-email'
        doc.head.appendChild(st)
      }
      st.textContent = DARK_FILTER
    } else st?.remove()
  }
  useEffect(setFilter, [darkEmail, html]) // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <iframe
      ref={ref}
      title="email"
      sandbox="allow-same-origin allow-scripts"
      srcDoc={srcDoc}
      onLoad={onLoad}
      style={{ height }}
      className={`w-full rounded-md border border-border ${textDark ? 'bg-panel' : 'bg-white'}`}
    />
  )
}
