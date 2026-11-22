// A message at the bottom of the window, with an optional action (Undo).
export default function Toast({ toast, onAction, onDismiss }) {
  if (!toast) return null
  return (
    <div role="status" className="fixed bottom-5 left-1/2 z-[80] flex max-w-[90vw] -translate-x-1/2 items-center gap-4 rounded-lg border border-border bg-panel px-4 py-2.5 text-[13px] shadow-2xl">
      <span className={`selectable ${toast.error ? 'text-danger' : ''}`}>{toast.text}</span>
      {toast.action && <button onClick={onAction} className="font-medium text-accent hover:underline">{toast.action}</button>}
      <button onClick={onDismiss} aria-label="Dismiss" className="text-muted hover:text-fg">✕</button>
    </div>
  )
}
