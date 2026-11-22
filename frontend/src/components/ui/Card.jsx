import clsx from 'clsx'

export default function Card({ className, ...props }) {
  return <div className={clsx('rounded-lg border border-border bg-panel', className)} {...props} />
}
