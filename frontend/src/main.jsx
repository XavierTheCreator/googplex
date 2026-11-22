import React from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.jsx'
import PipPage from './pages/PipPage.jsx'
import { getTheme, onThemeChanged, pipConfig } from './lib/api.js'
import { applyTheme } from './lib/theme.js'
import './index.css'

// The same frontend runs in the main window and in the pop-out player window
// (started with --pip); the pop-out only shows PipPage.
const root = createRoot(document.getElementById('root'))
pipConfig()
  .then((config) => {
    if (config.active) {
      applyTheme(config.theme)
      onThemeChanged(applyTheme) // the main app's theme, checked every few seconds
      document.documentElement.classList.add('pip') // no scroll bars in the pop-out
      root.render(<PipPage config={config} />)
    } else {
      getTheme().then(applyTheme).catch(() => {})
      root.render(<App />)
    }
  })
  .catch(() => root.render(<App />))
