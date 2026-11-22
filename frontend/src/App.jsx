import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { TooltipProvider } from './components/ui/Tooltip.jsx'
import Sidebar from './components/Sidebar.jsx'
import TitleBar from './components/TitleBar.jsx'
import { Environment } from '../wailsjs/runtime/runtime'
import SetupPage from './pages/SetupPage.jsx'
import HomePage from './pages/HomePage.jsx'
import GmailPage from './pages/GmailPage.jsx'
import CalendarPage from './pages/CalendarPage.jsx'
import YouTubePage from './pages/YouTubePage.jsx'
import SettingsPage from './pages/SettingsPage.jsx'
import DrivePage from './pages/DrivePage.jsx'
import DocsPage from './pages/DocsPage.jsx'
import SheetsPage from './pages/SheetsPage.jsx'
import TransferTray from './components/TransferTray.jsx'
import PlayerProvider from './components/PlayerProvider.jsx'
import DownloadTray from './components/DownloadTray.jsx'
import { getSetupState, setTheme } from './lib/api.js'
import { resolve } from './lib/theme.js'

export default function App() {
  const [state, setState] = useState(null)
  const [route, setRoute] = useState('home')
  const [openFile, setOpenFile] = useState({ docs: '', sheets: '' }) // the Doc or Sheet each viewer shows
  const [isMac, setIsMac] = useState(false)

  useEffect(() => {
    getSetupState().then(setState)
    Environment().then((env) => setIsMac(env.platform === 'darwin'))
  }, [])

  // Shortcuts stay inside the window (Ctrl/Cmd + 1..4, comma for settings).
  useEffect(() => {
    const keys = { 1: 'home', 2: 'gmail', 3: 'calendar', 4: 'youtube', ',': 'settings' }
    const onKey = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key.toLowerCase() === 'l') { // toggle light and dark
        e.preventDefault()
        setTheme(resolve() === 'dark' ? 'light' : 'dark')
        return
      }
      if ((e.metaKey || e.ctrlKey) && keys[e.key]) {
        e.preventDefault()
        setRoute(keys[e.key])
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  if (!state) {
    return <div className="grid h-full place-items-center text-muted"><Loader2 className="animate-spin" /></div>
  }
  if (state.startError) {
    return <div className="grid h-full place-items-center p-8 text-danger selectable">Googleplex could not start: {state.startError}</div>
  }
  if (!state.signedIn) {
    return <SetupPage state={state} onChange={setState} />
  }

  const pages = {
    home: <HomePage account={state.account} onNavigate={setRoute} />,
    gmail: <GmailPage />,
    calendar: <CalendarPage />,
    youtube: <YouTubePage />,
    drive: <DrivePage onOpenDoc={(kind, f) => { setOpenFile((o) => ({ ...o, [kind]: f.id })); setRoute(kind) }} />,
    docs: <DocsPage docId={openFile.docs} onOpen={(id) => setOpenFile((o) => ({ ...o, docs: id }))} />,
    sheets: <SheetsPage sheetId={openFile.sheets} onOpen={(id) => setOpenFile((o) => ({ ...o, sheets: id }))} />,
    settings: <SettingsPage state={state} onChange={setState} />,
  }

  return (
    <TooltipProvider>
      <PlayerProvider onShowYouTube={() => setRoute('youtube')}>
        <div className="flex h-full flex-col">
          {isMac && <TitleBar />}
          <div className="flex min-h-0 flex-1">
            <Sidebar route={route} onSelect={setRoute} />
            <main id="main-scroll" className="min-w-0 flex-1 overflow-y-auto">{pages[route]}</main>
          </div>
        </div>
        <DownloadTray />
        <TransferTray />
      </PlayerProvider>
    </TooltipProvider>
  )
}
