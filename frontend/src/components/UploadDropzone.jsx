import { useEffect, useRef } from 'react'
import { onFileDrop } from '../lib/api.js'

// Files dropped here upload to the current folder. Wails delivers the dropped
// files' paths for elements with --wails-drop-target, and marks the element
// with the wails-drop-target-active class while something is dragged over it.
export default function UploadDropzone({ onDrop, disabled, children, className = '' }) {
  const handler = useRef(onDrop)
  handler.current = onDrop

  useEffect(() => {
    if (disabled) return
    return onFileDrop((paths) => paths?.length && handler.current(paths))
  }, [disabled])

  return (
    <div
      style={disabled ? undefined : { '--wails-drop-target': 'drop' }}
      className={`relative [&.wails-drop-target-active]:outline [&.wails-drop-target-active]:outline-2 [&.wails-drop-target-active]:-outline-offset-4 [&.wails-drop-target-active]:outline-accent [&.wails-drop-target-active]:bg-accent/5 ${className}`}
    >
      {children}
    </div>
  )
}
