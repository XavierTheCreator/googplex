import { File, FileArchive, FileAudio, FileImage, FileSpreadsheet, FileText, FileType, FileVideo, Folder, Presentation } from 'lucide-react'
import { fileKind } from '../lib/files.js'

const icons = {
  folder: [Folder, 'text-muted'],
  doc: [FileText, 'text-[#4285f4]'],
  sheet: [FileSpreadsheet, 'text-[#0f9d58]'],
  slides: [Presentation, 'text-[#f4b400]'],
  pdf: [FileType, 'text-[#db4437]'],
  image: [FileImage, 'text-[#db4437]'],
  video: [FileVideo, 'text-[#db4437]'],
  audio: [FileAudio, 'text-accent'],
  archive: [FileArchive, 'text-muted'],
  text: [FileText, 'text-muted'],
  google: [File, 'text-accent'],
  other: [File, 'text-muted'],
}

export default function FileIcon({ mimeType, size = 18 }) {
  const [Icon, color] = icons[fileKind(mimeType)] || icons.other
  return <Icon size={size} className={`shrink-0 ${color}`} />
}
