import { Star, Users } from 'lucide-react'
import FileIcon from './FileIcon.jsx'
import { FileContextMenu, FileMenuButton } from './FileMenu.jsx'
import { formatSize, modifiedLabel, ownerOf } from '../lib/files.js'

export const rowGrid = 'grid grid-cols-[minmax(0,1fr)_150px_110px_90px_36px] items-center gap-3'

export default function FileRow({ file, selected, onSelect, onOpen, on, inTrash }) {
  return (
    <FileContextMenu file={file} on={on} inTrash={inTrash}>
      <div
        role="row"
        tabIndex={0}
        onClick={() => onSelect(file)}
        onDoubleClick={() => onOpen(file)}
        onKeyDown={(e) => e.key === 'Enter' && onOpen(file)}
        className={`${rowGrid} cursor-default border-b border-border px-4 py-2 text-[13px] outline-none ${selected ? 'bg-accent/10' : 'hover:bg-raised/60'}`}
      >
        <div className="flex min-w-0 items-center gap-3">
          <FileIcon mimeType={file.mimeType} />
          <span className="truncate">{file.name}</span>
          {file.starred && <Star size={13} className="shrink-0 fill-warning text-warning" />}
          {file.shared && <Users size={13} className="shrink-0 text-muted" />}
        </div>
        <span className="truncate text-muted">{ownerOf(file)}</span>
        <span className="text-muted">{modifiedLabel(file.modifiedTime)}</span>
        <span className="text-muted tabular-nums">{formatSize(file.size)}</span>
        <FileMenuButton file={file} on={on} inTrash={inTrash} />
      </div>
    </FileContextMenu>
  )
}
