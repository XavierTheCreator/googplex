import { useCallback, useEffect, useRef, useState } from 'react'
import { Loader2, Power, ShieldCheck } from 'lucide-react'
import Card from './ui/Card.jsx'
import Button from './ui/Button.jsx'
import ExternalLink from './ExternalLink.jsx'
import { checkService, connectService, consoleLinks, errorText } from '../lib/api.js'

const POLL_MS = 3000
const POLL_LIMIT = 40 // about 2 minutes; enabling an API can take a minute to apply

// Shows a feature's own setup the first time it is opened, then renders children.
export default function FeatureGate({ service, apiName, apiNames, why, children }) {
  const [status, setStatus] = useState(null)
  const [links, setLinks] = useState({})
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [polling, setPolling] = useState(false)
  const polls = useRef(0)

  const refresh = useCallback(async () => {
    const s = await checkService(service)
    setStatus(s)
    return s
  }, [service])

  useEffect(() => {
    refresh()
    consoleLinks().then(setLinks)
  }, [refresh])

  // After the user opens the Console, keep checking until the API is on.
  useEffect(() => {
    if (!polling) return
    const t = setInterval(async () => {
      polls.current += 1
      const s = await refresh()
      if (s.status !== 'api_disabled' || polls.current >= POLL_LIMIT) setPolling(false)
    }, POLL_MS)
    return () => clearInterval(t)
  }, [polling, refresh])

  const grant = async () => {
    setBusy(true)
    setError('')
    try {
      await connectService(service)
      await refresh()
    } catch (err) {
      setError(errorText(err))
    } finally {
      setBusy(false)
    }
  }

  if (!status) {
    return <div className="grid h-64 place-items-center text-muted"><Loader2 className="animate-spin" size={18} /></div>
  }
  if (status.status === 'ready') return children

  return (
    <div className="mx-auto max-w-xl px-8 py-12">
      <Card className="p-6">
        {status.status === 'api_disabled' && (
          <>
            <Power className="text-accent" size={22} />
            <h2 className="mt-3 text-[16px] font-semibold">Turn on the {apiNames?.[status.api] || apiName}</h2>
            <p className="mt-1.5 text-muted">
              {why} Open your Cloud project, press <span className="text-fg">Enable</span>, then come back. This page
              notices on its own once it is on.
            </p>
            <div className="mt-5 flex items-center gap-3">
              <span onClick={() => { polls.current = 0; setPolling(true) }}>
                <ExternalLink href={links[status.api || service]} variant="primary">Open {apiNames?.[status.api] || apiName}</ExternalLink>
              </span>
              {polling && <span className="flex items-center gap-2 text-muted"><Loader2 className="animate-spin" size={14} /> Waiting for Google…</span>}
            </div>
          </>
        )}
        {status.status === 'needs_consent' && (
          <>
            <ShieldCheck className="text-accent" size={22} />
            <h2 className="mt-3 text-[16px] font-semibold">Allow Googleplex to use {apiName.replace(/ API.*$/, '')}</h2>
            <p className="mt-1.5 text-muted">{why} Your browser will open Google's permission page for just this feature.</p>
            <Button className="mt-5" variant="primary" onClick={grant} disabled={busy}>
              {busy && <Loader2 className="animate-spin" size={14} />} Allow access
            </Button>
          </>
        )}
        {status.status === 'needs_sign_in' && (
          <>
            <h2 className="text-[16px] font-semibold">Sign in again</h2>
            <p className="mt-1.5 text-muted">{status.detail}</p>
            <Button className="mt-5" variant="primary" onClick={grant} disabled={busy}>Sign in</Button>
          </>
        )}
        {status.status === 'error' && (
          <>
            <h2 className="text-[16px] font-semibold">Google returned an error</h2>
            <p className="selectable mt-1.5 break-words font-mono text-[12px] text-muted">{status.detail}</p>
            <Button className="mt-5" onClick={refresh}>Try again</Button>
          </>
        )}
        {error && <p className="mt-3 text-danger">{error}</p>}
      </Card>
    </div>
  )
}
