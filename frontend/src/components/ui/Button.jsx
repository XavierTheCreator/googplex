import { Slot } from '@radix-ui/react-slot'
import clsx from 'clsx'

const variants = {
  primary: 'bg-accent text-on-accent hover:bg-accent/90',
  secondary: 'bg-raised text-fg border border-border hover:bg-border/60',
  ghost: 'text-muted hover:text-fg hover:bg-raised',
  danger: 'bg-danger/10 text-danger hover:bg-danger/20',
}
const sizes = {
  sm: 'h-7 px-2.5 text-[13px] gap-1.5',
  md: 'h-9 px-3.5 gap-2',
}

export default function Button({ variant = 'secondary', size = 'md', asChild, className, ...props }) {
  const Comp = asChild ? Slot : 'button'
  return (
    <Comp
      className={clsx(
        'inline-flex items-center justify-center rounded-md font-medium transition-colors',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/60',
        'disabled:opacity-50 disabled:pointer-events-none',
        variants[variant],
        sizes[size],
        className,
      )}
      {...props}
    />
  )
}
