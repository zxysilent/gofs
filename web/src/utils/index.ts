// File-kind detection and formatting helpers.
import { t } from '../locales'

// 项目主页；帮助面板里作为唯一一个指向外部站点的入口。
export const PROJECT_URL = 'https://github.com/zxysilent/gofs'

export type FileKind =
  | 'image'
  | 'video'
  | 'audio'
  | 'pdf'
  | 'markdown'
  | 'html'
  | 'json'
  | 'text'
  | 'code'
  | 'archive'
  | 'binary'

// 浏览器真能渲染的图片格式。tiff 之类不要放进图片：浏览器不支持时
// 只剩一个破图，还不如走 hex 视图看文件头。
const IMAGE_EXT = new Set(['png', 'jpg', 'jpeg', 'gif', 'svg', 'webp', 'bmp', 'ico', 'avif'])
// 只保留浏览器能直接播放的容器；avi/mkv/flv 等现代浏览器基本不解码，
// 放进视频预览只会得到一个永远转圈的黑块。
const VIDEO_EXT = new Set(['mp4', 'webm', 'ogv', 'm4v', 'mov'])
const AUDIO_EXT = new Set(['mp3', 'wav', 'ogg', 'oga', 'flac', 'm4a', 'aac', 'opus'])
const ARCHIVE_EXT = new Set(['zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', '7z', 'rar'])
const MARKDOWN_EXT = new Set(['md', 'markdown', 'mdown', 'mkd'])
// 网页单独成一类：默认渲染成页面，同时保留“看源码”的切换。
const HTML_EXT = new Set(['html', 'htm'])
const CODE_EXT = new Set([
  'go', 'py', 'js', 'mjs', 'cjs', 'ts', 'jsx', 'tsx', 'vue', 'svelte',
  'java', 'c', 'h', 'cpp', 'cc', 'hpp', 'cs', 'rb', 'php', 'rs', 'swift',
  'kt', 'kts', 'scala', 'sh', 'bash', 'zsh', 'fish', 'ps1', 'bat', 'cmd',
  'sql', 'r', 'm', 'pl', 'lua', 'dart', 'ex', 'exs', 'erl', 'hs', 'clj',
  'groovy', 'gradle', 'proto', 'graphql', 'toml', 'yaml', 'yml', 'xml',
  'css', 'scss', 'less', 'sass', 'ini', 'cfg', 'conf', 'properties',
  'dockerfile', 'makefile', 'cmake', 'lock', 'log',
])
const TEXT_EXT = new Set(['txt', 'text', 'csv', 'tsv', 'diff', 'patch', 'env', 'gitignore', 'editorconfig', 'license', 'readme'])

// 没有后缀、但公认是文本的文件名（比较时用小写 basename）。
// 除此之外的无后缀文件一律按二进制处理：可执行文件、共享库、数据文件都属此类，
// 早先一律当成文本，点开一个几 MB 的 ELF 会直接把 8 MB 文本塞进 <pre>，页面就卡死了。
const TEXT_NAMES = new Set([
  'makefile', 'gnumakefile', 'dockerfile', 'containerfile', 'vagrantfile',
  'gemfile', 'rakefile', 'procfile', 'brewfile', 'justfile', 'cmakelists',
  'license', 'licence', 'copying', 'notice', 'authors', 'contributors',
  'readme', 'changelog', 'changes', 'install', 'todo', 'version', 'revision',
  'hosts', 'passwd', 'group', 'fstab', 'sudoers', 'codeowners',
  '.gitignore', '.gitattributes', '.gitmodules', '.npmrc', '.nvmrc', '.env',
  '.editorconfig', '.dockerignore', '.babelrc', '.eslintrc', '.prettierrc',
])

// 是否应该按文本打开（预览/编辑）：文本、代码、Markdown、HTML、JSON 都算。
export function isTextName(name: string): boolean {
  const k = fileKind(name)
  return k === 'text' || k === 'code' || k === 'markdown' || k === 'html' || k === 'json'
}

export function fileKind(name: string): FileKind {
  const ext = extName(name)
  if (IMAGE_EXT.has(ext)) return 'image'
  if (VIDEO_EXT.has(ext)) return 'video'
  if (AUDIO_EXT.has(ext)) return 'audio'
  if (ext === 'pdf') return 'pdf'
  if (MARKDOWN_EXT.has(ext)) return 'markdown'
  if (HTML_EXT.has(ext)) return 'html'
  if (ext === 'json') return 'json'
  if (ARCHIVE_EXT.has(ext)) return 'archive'
  if (CODE_EXT.has(ext)) return 'code'
  if (TEXT_EXT.has(ext)) return 'text'
  if (ext === '') {
    // 无后缀：只有白名单里的名字当文本，其余按二进制
    return TEXT_NAMES.has(lastSegment(name).toLowerCase()) ? 'text' : 'binary'
  }
  return 'binary'
}

export function extName(name: string): string {
  const idx = name.lastIndexOf('.')
  if (idx <= 0 || idx === name.length - 1) return ''
  return name.slice(idx + 1).toLowerCase()
}

// 取路径里最后一段（文件名）
function lastSegment(name: string): string {
  const idx = name.lastIndexOf('/')
  return idx >= 0 ? name.slice(idx + 1) : name
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
  if (size >= 1000) return '>999'
  return t('list.items', { n: size })
}

// 修改时间统一为 YYYY-MM-DD HH:mm:ss。
// 以前列表里“今天”的行只显示时:分，与其他行格式不一致，也难以复制出去比对。
export function formatTime(mtime: number): string {
  if (!mtime) return '-'
  const d = new Date(mtime)
  if (Number.isNaN(d.getTime())) return '-'
  const pad = (n: number) => String(n).padStart(2, '0')
  const date = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  return `${date} ${time}`
}
