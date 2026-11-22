import * as RT from '@radix-ui/react-tooltip'

export function TooltipProvider({ children }) {
  return <RT.Provider delayDuration={300}>{children}</RT.Provider>
}

export default function Tooltip({ label, side = 'right', children }) {
  return (
    <RT.Root>
      <RT.Trigger asChild>{children}</RT.Trigger>
      <RT.Portal>
        <RT.Content side={side} sideOffset={6} className="max-w-xs rounded bg-raised border border-border px-2 py-1 text-[12px] text-fg shadow-lg">
          {label}
        </RT.Content>
      </RT.Portal>
    </RT.Root>
  )
}
