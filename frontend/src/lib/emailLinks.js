// What clicking things inside an email does. The email is shown in an iframe
// that can't run scripts; the app listens on its document instead:
//   http(s) links    -> the system browser
//   mailto: links    -> the app's own compose window, prefilled
//   anything else    -> nothing (javascript:, data:, file:, cid:, …)
//   forms            -> GET forms to http(s) open in the browser with their
//                       fields; others can't work here, so the app says so

// Why the frame allows scripts at all: WebKit (the app's webview) never runs
// event listeners on a document whose sandbox forbids scripts, even
// listeners added from outside it, so links could not be intercepted. The
// frame therefore allows scripts, and two layers make sure none of them can
// come from the email: sanitizeEmail strips every way an email could carry
// script, and EMAIL_CSP forbids scripts, frames, plugins and form posts.
export const EMAIL_CSP = [
  "default-src 'none'",
  "script-src 'none'",
  "object-src 'none'",
  "frame-src 'none'",
  "form-action 'none'",
  "base-uri 'none'",
  "img-src * data: blob: cid:",
  "media-src * data: blob:",
  "style-src * 'unsafe-inline'",
  "font-src * data:",
].join('; ')

const DROP = 'script, noscript, iframe, frame, frameset, object, embed, applet, base, link[rel=import], meta, portal, template'
const URL_ATTRS = ['href', 'src', 'action', 'formaction', 'xlink:href', 'background', 'poster', 'data']

// Removes anything in an email that could run script, keeping its layout.
export function sanitizeEmail(html) {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll(DROP).forEach((el) => el.remove())
  doc.querySelectorAll('*').forEach((el) => {
    for (const attr of [...el.attributes]) {
      const name = attr.name.toLowerCase()
      const value = attr.value.replace(/[\u0000-\u0020]/g, '').toLowerCase()
      if (name.startsWith('on') || name === 'srcdoc') el.removeAttribute(attr.name)
      else if (URL_ATTRS.includes(name) && (value.startsWith('javascript:') || value.startsWith('vbscript:') ||
        (value.startsWith('data:') && !/^data:image\/(png|gif|jpe?g|webp|bmp)[;,]/.test(value)))) el.removeAttribute(attr.name)
    }
  })
  // Styles stay, but not the old ways of running script from CSS.
  doc.querySelectorAll('style').forEach((st) => { st.textContent = st.textContent.replace(/expression\s*\(|behavior\s*:|-moz-binding/gi, '') })
  return doc.body.innerHTML
}

// "mailto:a@x.com,b@y.com?subject=Hi&cc=c@z.com&body=Hello" -> a draft.
export function parseMailto(href) {
  const rest = href.replace(/^mailto:/i, '')
  const [to, query = ''] = rest.split('?')
  const params = new URLSearchParams(query)
  const get = (k) => {
    for (const [key, v] of params) if (key.toLowerCase() === k) return v
    return ''
  }
  const decode = (s) => {
    try { return decodeURIComponent(s) } catch { return s }
  }
  const extraTo = get('to')
  return {
    to: [decode(to), extraTo].filter(Boolean).join(', '),
    cc: get('cc'),
    subject: get('subject'),
    body: get('body'),
    threadId: '',
    inReplyTo: '',
  }
}

// { kind: 'web', url } | { kind: 'mailto', draft } | { kind: 'ignore' }
export function linkAction(href) {
  if (!href) return { kind: 'ignore' }
  if (/^https?:\/\//i.test(href)) return { kind: 'web', url: href }
  if (/^mailto:/i.test(href)) return { kind: 'mailto', draft: parseMailto(href) }
  return { kind: 'ignore' }
}

// { kind: 'web', url } for a GET form to http(s), else { kind: 'blocked' }.
export function formAction(form, submitter) {
  const action = submitter?.getAttribute('formaction') || form.getAttribute('action') || ''
  const method = (submitter?.getAttribute('formmethod') || form.getAttribute('method') || 'get').toLowerCase()
  let url
  try {
    url = new URL(action, 'about:blank')
  } catch {
    return { kind: 'blocked' }
  }
  if (method !== 'get' || !/^https?:$/.test(url.protocol)) return { kind: 'blocked' }
  const data = new FormData(form)
  if (submitter?.name) data.append(submitter.name, submitter.value || '')
  for (const [k, v] of data) if (typeof v === 'string') url.searchParams.append(k, v)
  return { kind: 'web', url: url.toString() }
}

const linkOf = (el) => el?.closest?.('a[href], area[href]')
const submitterOf = (el) => el?.closest?.('button:not([type]), button[type=submit], input[type=submit], input[type=image]')

// Wires an email's document. handlers: onWeb(url), onMailto(draft),
// onBlockedForm(), onHover(url or ''). Returns a function that unwires it.
export function attachEmailHandlers(doc, { onWeb, onMailto, onBlockedForm, onHover }) {
  const follow = (href) => {
    const act = linkAction(href)
    if (act.kind === 'web') onWeb(act.url)
    else if (act.kind === 'mailto') onMailto(act.draft)
  }
  const submit = (form, submitter) => {
    const act = formAction(form, submitter)
    if (act.kind === 'web') onWeb(act.url)
    else onBlockedForm()
  }

  const onClick = (e) => {
    const a = linkOf(e.target)
    if (a) {
      e.preventDefault()
      e.stopPropagation()
      if (e.button === 0 || e.button === 1) follow(a.href)
      return
    }
    const btn = submitterOf(e.target)
    if (btn?.form) {
      e.preventDefault()
      e.stopPropagation()
      submit(btn.form, btn)
    }
  }
  const onSubmit = (e) => {
    e.preventDefault()
    e.stopPropagation()
    submit(e.target, e.submitter)
  }
  const onOver = (e) => onHover(linkOf(e.target)?.href || '')
  const onLeave = () => onHover('')

  doc.addEventListener('click', onClick, true)
  doc.addEventListener('auxclick', onClick, true) // middle-click
  doc.addEventListener('submit', onSubmit, true)
  doc.addEventListener('mouseover', onOver, true)
  doc.documentElement.addEventListener('mouseleave', onLeave)
  return () => {
    doc.removeEventListener('click', onClick, true)
    doc.removeEventListener('auxclick', onClick, true)
    doc.removeEventListener('submit', onSubmit, true)
    doc.removeEventListener('mouseover', onOver, true)
    doc.documentElement.removeEventListener('mouseleave', onLeave)
  }
}
