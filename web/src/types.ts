// Shared types mirroring the Go server's JSON API.

export type PathType = 'Dir' | 'SymlinkDir' | 'File' | 'SymlinkFile'

export interface PathItem {
  path_type: PathType
  name: string
  mtime: number
  size: number
}

export interface IndexData {
  href: string
  kind: 'Index' | 'Edit' | 'View'
  uri_prefix: string
  allow_upload: boolean
  allow_delete: boolean
  allow_search: boolean
  allow_archive: boolean
  dir_exists: boolean
  auth: boolean
  user: string | null
  paths: PathItem[]
}

export interface TreeNode {
  path: string
  name: string
  expanded: boolean
  loaded: boolean
  collapsed?: boolean
  children: TreeNode[]
}

export interface UploadTask {
  id: string
  name: string
  progress: number
  status: 'uploading' | 'done' | 'error'
  error?: string
}

export type SortKey = 'name' | 'mtime' | 'size'
export type SortOrder = 'asc' | 'desc'

// 特殊前端路由：curl 使用手册页（不会请求服务端目录数据）。
export const HELP_ROUTE = '/__gofs_help__'

export function isHelpRoute(p: string): boolean {
  return normalizeRelPath(p) === HELP_ROUTE
}
export interface ListParams {
  sort: SortKey
  order: SortOrder
  q: string
}

export function isDir(item: PathItem): boolean {
  return item.path_type.endsWith('Dir')
}

// Normalize a server-returned href (relative to serve root) to a clean path
// that always starts with "/" and has no trailing slash (except root "/").
export function normalizeRelPath(href: string): string {
  let p = href || '/'
  if (!p.startsWith('/')) p = '/' + p
  // collapse "/." produced for the serve root
  p = p.replace(/\/\.$/, '')
  // collapse duplicate slashes
  p = p.replace(/\/+/g, '/')
  if (p === '') return '/'
  return p
}

// Join a child name onto a base relative path.
export function joinRelPath(base: string, name: string): string {
  const b = base === '/' ? '' : base
  return b + '/' + name
}

// Parent relative path of a given relative path ("/a/b" -> "/a", "/a" -> "/").
export function parentRelPath(p: string): string {
  const idx = p.lastIndexOf('/')
  if (idx <= 0) return '/'
  return p.slice(0, idx)
}

// Final segment of a relative path ("/a/b.txt" -> "b.txt").
export function baseName(p: string): string {
  const idx = p.lastIndexOf('/')
  return idx < 0 ? p : p.slice(idx + 1)
}

// File extension (lowercase, without dot); "" when none.
export function extName(name: string): string {
  const base = baseName(name)
  const idx = base.lastIndexOf('.')
  if (idx <= 0 || idx === base.length - 1) return ''
  return base.slice(idx + 1).toLowerCase()
}
