// File-kind detection and formatting helpers.

export type FileKind =
  | 'image'
  | 'video'
  | 'audio'
  | 'pdf'
  | 'markdown'
  | 'json'
  | 'text'
  | 'code'
  | 'archive'
  | 'binary'

const IMAGE_EXT = new Set([
  'png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico', 'avif', 'tiff',
])
const VIDEO_EXT = new Set(['mp4', 'webm', 'ogv', 'mov', 'm4v', 'avi', 'mkv', 'flv'])
const AUDIO_EXT = new Set(['mp3', 'wav', 'ogg', 'flac', 'm4a', 'aac', 'opus', 'oga'])
const ARCHIVE_EXT = new Set(['zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', '7z', 'rar'])
const MARKDOWN_EXT = new Set(['md', 'markdown', 'mdown', 'mkd'])
const CODE_EXT = new Set([
  'go', 'py', 'js', 'mjs', 'cjs', 'ts', 'jsx', 'tsx', 'vue', 'svelte',
  'java', 'c', 'h', 'cpp', 'cc', 'hpp', 'cs', 'rb', 'php', 'rs', 'swift',
  'kt', 'kts', 'scala', 'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd',
  'sql', 'r', 'm', 'pl', 'lua', 'dart', 'ex', 'exs', 'erl', 'hs', 'clj',
  'groovy', 'gradle', 'proto', 'graphql', 'toml', 'yaml', 'yml', 'xml',
  'html', 'css', 'scss', 'less', 'sass', 'ini', 'cfg', 'conf', 'properties',
  'dockerfile', 'makefile', 'cmake', 'lock', 'log',
])
const TEXT_EXT = new Set(['txt', 'text', 'csv', 'tsv', 'diff', 'patch', 'env', 'gitignore', 'editorconfig', 'license', 'readme'])

export function fileKind(name: string): FileKind {
  const ext = extName(name)
  if (IMAGE_EXT.has(ext)) return 'image'
  if (VIDEO_EXT.has(ext)) return 'video'
  if (AUDIO_EXT.has(ext)) return 'audio'
  if (ext === 'pdf') return 'pdf'
  if (MARKDOWN_EXT.has(ext)) return 'markdown'
  if (ext === 'json') return 'json'
  if (ARCHIVE_EXT.has(ext)) return 'archive'
  if (CODE_EXT.has(ext)) return 'code'
  if (TEXT_EXT.has(ext)) return 'text'
  // extensionless files are often text
  if (ext === '') return 'text'
  return 'binary'
}

export function extName(name: string): string {
  const idx = name.lastIndexOf('.')
  if (idx <= 0 || idx === name.length - 1) return ''
  return name.slice(idx + 1).toLowerCase()
}

const SIZE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']

export function formatSize(size: number): string {
  if (size == null || size < 0) return '-'
  if (size === 0) return '0 B'
  let i = Math.floor(Math.log(size) / Math.log(1024))
  if (i >= SIZE_UNITS.length) i = SIZE_UNITS.length - 1
  const v = size / Math.pow(1024, i)
  const text = v >= 100 || i === 0 ? Math.round(v).toString() : v.toFixed(1)
  return `${text} ${SIZE_UNITS[i]}`
}

export function formatDirSize(size: number): string {
  if (size >= 1000) return `>999 items`
  return `${size} item${size === 1 ? '' : 's'}`
}

export function formatTime(mtime: number): string {
  if (!mtime) return '-'
  const d = new Date(mtime)
  const pad = (n: number) => String(n).padStart(2, '0')
  const today = new Date()
  const sameDay =
    d.getFullYear() === today.getFullYear() &&
    d.getMonth() === today.getMonth() &&
    d.getDate() === today.getDate()
  if (sameDay) {
    return `今天 ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
