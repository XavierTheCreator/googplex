import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import SaveToPlaylistDialog from './SaveToPlaylistDialog.jsx'
import Toast from './Toast.jsx'
import { errorText, ytAddWatchLater, ytRemoveWatchLater, ytWatchLater } from '../lib/api.js'

const Ctx = createContext(null)

// What a video's ⋯ menu can do besides queueing: save to a playlist, Watch
// later (kept in the app), and short confirmations. Mounted by the YouTube page.
export const useVideoActions = () => useContext(Ctx)

export default function VideoActionsProvider({ children }) {
  const [saving, setSaving] = useState(null) // the video in the Save to playlist dialog
  const [later, setLater] = useState(new Set())
  const [toast, setToast] = useState(null)

  const notify = useCallback((text, error) => {
    setToast({ text, error })
    setTimeout(() => setToast(null), 3000)
  }, [])

  useEffect(() => {
    ytWatchLater().then((vs) => setLater(new Set((vs || []).map((v) => v.id)))).catch(() => {})
  }, [])

  const toggleWatchLater = async (v) => {
    const on = later.has(v.id)
    setLater((s) => {
      const n = new Set(s)
      on ? n.delete(v.id) : n.add(v.id)
      return n
    })
    try {
      if (on) await ytRemoveWatchLater(v.id)
      else await ytAddWatchLater({ ...v, views: v.views || 0, durationSeconds: v.durationSeconds || 0 })
      notify(on ? 'Removed from Watch later' : 'Saved to Watch later')
    } catch (err) {
      setLater((s) => {
        const n = new Set(s)
        on ? n.add(v.id) : n.delete(v.id)
        return n
      })
      notify(errorText(err), true)
    }
  }

  const value = { saveToPlaylist: setSaving, toggleWatchLater, inWatchLater: (id) => later.has(id), notify }
  return (
    <Ctx.Provider value={value}>
      {children}
      <SaveToPlaylistDialog video={saving} onClose={() => setSaving(null)} notify={notify} />
      <Toast toast={toast} onDismiss={() => setToast(null)} />
    </Ctx.Provider>
  )
}
