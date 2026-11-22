import { useEffect, useState } from 'react'
import * as ToggleGroup from '@radix-ui/react-toggle-group'
import { FolderOpen, Loader2, LogOut, Monitor, Moon, RefreshCw, Sun } from 'lucide-react'
import { themeLabels, useTheme } from '../lib/theme.js'
import PageHeader from '../components/PageHeader.jsx'
import Card from '../components/ui/Card.jsx'
import Button from '../components/ui/Button.jsx'
import { chooseDownloadDir, errorText, getDownloadSettings, getWeekStart, gmailLocalSignature, gmailSetLocalSignature, setTheme, setWeekStart, signOut, updateYtdlp } from '../lib/api.js'
import Textarea from '../components/ui/Textarea.jsx'

export default function SettingsPage({ state, onChange }) {
  const acc = state.account
  return (
    <>
      <PageHeader title="Settings" />
      <div className="max-w-2xl space-y-4 px-8 py-6">
        <Card className="flex items-center gap-4 p-5">
          {acc?.picture && <img src={acc.picture} alt="" className="h-10 w-10 rounded-full" referrerPolicy="no-referrer" />}
          <div className="flex-1">
            <div className="font-medium">{acc?.name}</div>
            <div className="selectable text-muted">{acc?.email}</div>
          </div>
          <Button variant="danger" onClick={async () => onChange(await signOut())}>
            <LogOut size={14} /> Sign out
          </Button>
        </Card>
        <Card className="p-5 text-[13px] text-muted">
          <div>Cloud project: <span className="selectable text-fg">{state.projectId || 'not set'}</span></div>
          <div className="mt-1">OAuth client from: <span className="text-fg">{state.clientSource === 'env' ? '.env (dev build)' : 'downloaded JSON'}</span></div>
        </Card>
        <AppearanceCard />
        <GmailCard />
        <CalendarCard />
        <DownloadsCard />
      </div>
    </>
  )
}

const ua = navigator.userAgent
const ffmpegHint = ua.includes('Mac') ? 'brew install ffmpeg' : ua.includes('Windows') ? 'winget install ffmpeg' : 'sudo apt install ffmpeg'

function DownloadsCard() {
  const [settings, setSettings] = useState(null)
  const [updating, setUpdating] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    getDownloadSettings().then(setSettings).catch((err) => setError(errorText(err)))
  }, [])

  const run = async (fn) => {
    setError('')
    try {
      await fn()
    } catch (err) {
      setError(errorText(err))
    }
  }

  const changeDir = () => run(async () => setSettings(await chooseDownloadDir()))
  const update = () => run(async () => {
    setUpdating(true)
    try {
      const version = await updateYtdlp()
      setSettings((s) => ({ ...s, ytdlpVersion: version }))
    } finally {
      setUpdating(false)
    }
  })

  return (
    <Card className="space-y-3 p-5">
      <h2 className="font-medium">Downloads</h2>
      <Row label="Folder">
        <span className="selectable min-w-0 flex-1 truncate text-fg">{settings?.dir || '…'}</span>
        <Button size="sm" onClick={changeDir}><FolderOpen size={14} /> Change</Button>
      </Row>
      <Row label="yt-dlp">
        <span className="min-w-0 flex-1 text-fg">
          {settings ? settings.ytdlpVersion || 'Not installed yet. It downloads on your first video download.' : '…'}
        </span>
        <Button size="sm" onClick={update} disabled={updating}>
          {updating ? <Loader2 className="animate-spin" size={14} /> : <RefreshCw size={14} />}
          {settings?.ytdlpVersion ? 'Update' : 'Install'}
        </Button>
      </Row>
      <Row label="ffmpeg">
        <span className="text-fg">
          {!settings ? '…' : settings.hasFFmpeg ? 'Found. Downloads are full quality.' : `Not found. Install ffmpeg (${ffmpegHint}) for HD downloads.`}
        </span>
      </Row>
      {error && <p className="selectable text-[13px] text-danger">{error}</p>}
      <p className="text-[12px] text-muted">
        Downloading may break YouTube's terms of service, so only download videos you have the right to.
      </p>
    </Card>
  )
}

function AppearanceCard() {
  const { choice } = useTheme()
  const item = 'flex h-8 items-center gap-1.5 rounded px-3 text-[13px] text-muted data-[state=on]:bg-panel data-[state=on]:text-fg data-[state=on]:shadow-sm'
  return (
    <Card className="space-y-3 p-5">
      <h2 className="font-medium">Appearance</h2>
      <ToggleGroup.Root type="single" value={choice} onValueChange={(v) => v && setTheme(v)} className="inline-flex rounded-md bg-raised p-0.5" aria-label="Theme">
        <ToggleGroup.Item value="system" className={item}><Monitor size={14} /> {themeLabels.system}</ToggleGroup.Item>
        <ToggleGroup.Item value="light" className={item}><Sun size={14} /> {themeLabels.light}</ToggleGroup.Item>
        <ToggleGroup.Item value="dark" className={item}><Moon size={14} /> {themeLabels.dark}</ToggleGroup.Item>
      </ToggleGroup.Root>
      <p className="text-[13px] text-muted">System follows your computer's setting. ⌘/Ctrl + Shift + L switches between light and dark.</p>
    </Card>
  )
}

function GmailCard() {
  const [sig, setSig] = useState('')
  const [saved, setSaved] = useState(false)
  useEffect(() => { gmailLocalSignature().then((s) => setSig(s || '')) }, [])
  const save = async () => {
    await gmailSetLocalSignature(sig)
    setSaved(true)
    setTimeout(() => setSaved(false), 2000)
  }
  return (
    <Card className="space-y-3 p-5">
      <h2 className="font-medium">Gmail</h2>
      <p className="text-[13px] text-muted">New emails use the signature set in Gmail. If Googleplex can't read it, it uses this one instead.</p>
      <Textarea value={sig} onChange={(e) => setSig(e.target.value)} placeholder="Your signature" className="h-24 text-[13px]" />
      <div className="flex items-center gap-3">
        <Button size="sm" onClick={save}>Save signature</Button>
        {saved && <span className="text-[13px] text-success">Saved</span>}
      </div>
    </Card>
  )
}

function CalendarCard() {
  const [weekStart, setValue] = useState(null)
  useEffect(() => { getWeekStart().then(setValue) }, [])
  const change = async (e) => {
    const day = Number(e.target.value)
    await setWeekStart(day)
    setValue(day)
  }
  return (
    <Card className="space-y-3 p-5">
      <h2 className="font-medium">Calendar</h2>
      <Row label="Week starts on">
        <select value={weekStart ?? 0} onChange={change} disabled={weekStart === null}
          className="h-8 rounded-md border border-border bg-bg px-2 text-fg focus:outline-none focus:ring-2 focus:ring-accent/50">
          <option value={0}>Sunday</option>
          <option value={1}>Monday</option>
        </select>
      </Row>
    </Card>
  )
}

function Row({ label, children }) {
  return (
    <div className="flex items-center gap-3 text-[13px] text-muted">
      <span className="w-28 shrink-0">{label}</span>
      {children}
    </div>
  )
}
