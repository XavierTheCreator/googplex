export default function PageHeader({ title, subtitle, children }) {
  return (
    <header className="flex items-end justify-between border-b border-border px-8 pt-6 pb-5">
      <div>
        <h1 className="text-[20px] font-semibold tracking-tight">{title}</h1>
        {subtitle && <p className="mt-1 text-muted">{subtitle}</p>}
      </div>
      <div className="flex gap-2">{children}</div>
    </header>
  )
}
