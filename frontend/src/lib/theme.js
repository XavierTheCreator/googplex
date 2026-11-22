// The app's theme: 'system' (default), 'light' or 'dark'. The choice is saved
// by the Go side (settings table) and mirrored in localStorage so index.html
// can apply it before the first paint. Components read it with useTheme().
import { useSyncExternalStore } from 'react'

const KEY = 'gp.theme'
const media = window.matchMedia('(prefers-color-scheme: dark)')
const listeners = new Set()

const read = () => {
  try { return localStorage.getItem(KEY) || 'system' } catch { return 'system' }
}
let choice = read()

export const resolve = (c = choice) => (c === 'system' ? (media.matches ? 'dark' : 'light') : c)

// applyTheme switches the page; it doesn't save (setTheme in api.js does).
export function applyTheme(c) {
  choice = ['light', 'dark'].includes(c) ? c : 'system'
  try { localStorage.setItem(KEY, choice) } catch { /* only the startup copy is lost */ }
  document.documentElement.classList.toggle('dark', resolve() === 'dark')
  listeners.forEach((f) => f())
}

// "System" follows the OS as it changes.
media.addEventListener('change', () => { if (choice === 'system') applyTheme('system') })

const subscribe = (f) => {
  listeners.add(f)
  return () => listeners.delete(f)
}

// { choice, dark } and re-renders when either changes.
export function useTheme() {
  const c = useSyncExternalStore(subscribe, () => choice)
  const dark = useSyncExternalStore(subscribe, () => resolve() === 'dark')
  return { choice: c, dark }
}

export const themeLabels = { system: 'System', light: 'Light', dark: 'Dark' }
export const nextTheme = (c) => ({ light: 'dark', dark: 'system', system: 'light' })[c] || 'light'
