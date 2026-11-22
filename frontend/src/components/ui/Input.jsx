import { forwardRef } from 'react'
import clsx from 'clsx'

const Input = forwardRef(function Input({ className, ...props }, ref) {
  return (
    <input
      ref={ref}
      className={clsx(
        'h-9 w-full rounded-md border border-border bg-bg px-3 text-fg placeholder:text-muted/70',
        'focus:outline-none focus:ring-2 focus:ring-accent/50',
        className,
      )}
      {...props}
    />
  )
})

export default Input
