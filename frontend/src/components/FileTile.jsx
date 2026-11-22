import { useState } from 'react'
import { Star } from 'lucide-react'
import FileIcon from './FileIcon.jsx'
import { FileContextMenu, FileMenuButton } from './FileMenu.jsx'

export default function FileTile({ file, selected, onSelect, onOpen, on, inTrash }) {
  const [thumbFailed, setThumbFailed] = useState(false)
  return (
    <FileContextMenu file={file} on={on} inTrash={inTrash}>
      <div
        tabIndex={0}
        onClick={() => onSelect(file)}
        onDoubleClick={() => onOpen(file)}
        onKeyDown={(e) => e.key === 'Enter' && onOpen(file)}
        className={`group overflow-hidden rounded-lg border outline-none ${selected ? 'border-accent bg-accent/10' : 'border-border bg-panel hover:border-accent/40'}`}
      >
        <div className="grid aspect-[4/3] place-items-center overflow-hidden bg-raised">
          {file.thumbnailLink && !thumbFailed ? (
            <img src={file.thumbnailLink} alt="" loading="lazy" referrerPolicy="no-referrer" onError={() => setThumbFailed(true)}
              className="h-full w-full object-cover" />
          ) : (
            <FileIcon mimeType={file.mimeType} size={40} />
          )}
        </div>
        <div className="flex items-center gap-2 px-2.5 py-2 text-[13px]">
          <FileIcon mimeType={file.mimeType} size={15} />
          <span className="min-w-0 flex-1 truncate">{file.name}</span>
          {file.starred && <Star size={12} className="shrink-0 fill-warning text-warning" />}
          <span className="opacity-0 group-hover:opacity-100"><FileMenuButton file={file} on={on} inTrash={inTrash} /></span>
        </div>
      </div>
    </FileContextMenu>
  )
}
