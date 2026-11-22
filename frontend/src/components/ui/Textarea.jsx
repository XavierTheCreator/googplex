import clsx from 'clsx'

export default function Textarea({ className, ...props }) {
  return (
    <textarea
      className={clsx(
        'w-full resize-none rounded-md border border-border bg-bg px-3 py-2 text-fg placeholder:text-muted/70',
        'focus:outline-none focus:ring-2 focus:ring-accent/50',
        className,
      )}
      {...props}
    />
  )
}
