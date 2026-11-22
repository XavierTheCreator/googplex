import clsx from 'clsx'

const tones = {
  neutral: 'bg-raised text-muted',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning',
  danger: 'bg-danger/15 text-danger',
}

export default function Badge({ tone = 'neutral', className, ...props }) {
  return (
    <span
      className={clsx('inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-medium', tones[tone], className)}
      {...props}
    />
  )
}
