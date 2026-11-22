import { useEffect, useState } from 'react'
import * as ContextMenu from '@radix-ui/react-context-menu'
import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { MoreHorizontal } from 'lucide-react'
import { driveExportFormats } from '../lib/api.js'
import { fileKind, isGoogleFile } from '../lib/files.js'

const content = 'z-50 min-w-48 rounded-lg border border-border bg-panel p-1 text-[13px] shadow-xl'
const item = 'flex cursor-default select-none items-center rounded px-2.5 py-1.5 outline-none data-[highlighted]:bg-raised data-[disabled]:opacity-40'
const sep = 'my-1 h-px bg-border'

// The actions for one file, rendered with either Radix ContextMenu (right
// click) or DropdownMenu (the ⋯ button). on(action, file, format?) runs one.
function Items({ M, file, on, inTrash }) {
  const [formats, setFormats] = useState([])
  const google = isGoogleFile(file.mimeType)
  useEffect(() => {
    if (google) driveExportFormats(file.mimeType).then((f) => setFormats(f || []))
  }, [file.mimeType, google])

  const folder = fileKind(file.mimeType) === 'folder'
  return (
    <>
      <M.Item className={item} onSelect={() => on('open', file)}>Open</M.Item>
      {file.webViewLink && <M.Item className={item} onSelect={() => on('browser', file)}>Open in browser</M.Item>}
      {!folder && !google && <M.Item className={item} onSelect={() => on('download', file)}>Download</M.Item>}
      {google && formats.length > 0 && (
        <M.Sub>
          <M.SubTrigger className={item}>Export as…</M.SubTrigger>
          <M.Portal>
            <M.SubContent className={content} sideOffset={4}>
              {formats.map((f) => (
                <M.Item key={f} className={item} onSelect={() => on('export', file, f)}>{f.toUpperCase()}</M.Item>
              ))}
            </M.SubContent>
          </M.Portal>
        </M.Sub>
      )}
      {!inTrash && (
        <>
          <M.Separator className={sep} />
          <M.Item className={item} disabled={!file.capabilities?.canRename} onSelect={() => on('rename', file)}>Rename</M.Item>
          <M.Item className={item} onSelect={() => on('star', file)}>{file.starred ? 'Remove star' : 'Star'}</M.Item>
          <M.Separator className={sep} />
          <M.Item className={`${item} text-danger`} disabled={!file.capabilities?.canTrash} onSelect={() => on('trash', file)}>Move to trash</M.Item>
        </>
      )}
    </>
  )
}

// Wraps a row or tile so right-clicking it shows the file's actions.
export function FileContextMenu({ file, on, inTrash, children }) {
  return (
    <ContextMenu.Root>
      <ContextMenu.Trigger asChild>{children}</ContextMenu.Trigger>
      <ContextMenu.Portal>
        <ContextMenu.Content className={content}>
          <Items M={ContextMenu} file={file} on={on} inTrash={inTrash} />
        </ContextMenu.Content>
      </ContextMenu.Portal>
    </ContextMenu.Root>
  )
}

// The ⋯ button with the same actions.
export function FileMenuButton({ file, on, inTrash }) {
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger asChild>
        <button aria-label="More actions" onClick={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}
          className="grid h-7 w-7 place-items-center rounded text-muted hover:bg-raised hover:text-fg">
          <MoreHorizontal size={16} />
        </button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content className={content} align="end">
          <Items M={DropdownMenu} file={file} on={on} inTrash={inTrash} />
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}
