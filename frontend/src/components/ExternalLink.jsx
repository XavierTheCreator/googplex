import { ExternalLink as Icon } from 'lucide-react'
import Button from './ui/Button.jsx'
import { openUrl } from '../lib/api.js'

// Opens a Google page in the system browser, where the user is already signed in.
export default function ExternalLink({ href, children, variant = 'secondary', size = 'md' }) {
  return (
    <Button variant={variant} size={size} onClick={() => href && openUrl(href)} disabled={!href}>
      {children}
      <Icon size={14} />
    </Button>
  )
}
