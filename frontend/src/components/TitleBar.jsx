// On macOS the window has no native title bar, so this strip gives the
// traffic-light buttons their own space and lets the window be dragged.
export default function TitleBar() {
  return <div className="drag h-10 shrink-0 border-b border-border bg-panel" />
}
