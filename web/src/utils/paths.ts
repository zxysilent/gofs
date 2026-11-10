// 相对路径的规范化与拆分。服务端返回的 href 是相对服务根的路径，前端所有
// 导航、面包屑、目录树都基于这套约定。
import type { PathItem } from '../types'

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
