import type { IndexData, ListParams } from './types'
import { normalizeRelPath } from './types'

// The server injects the initial directory data as a base64-encoded JSON
// string into `window.__GOFS_DATA__` (via a placeholder in index.html).
declare global {
  interface Window {
    __GOFS_DATA__?: string
  }
}

// base64 decode that is safe for UTF-8 (Chinese filenames etc.).
export function b64decode(s: string): string {
  const bin = atob(s)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return new TextDecoder().decode(bytes)
}

function readInitialData(): IndexData {
  const b64 = window.__GOFS_DATA__
  if (!b64) {
    throw new Error('missing initial data')
  }
  try {
    return JSON.parse(b64decode(b64)) as IndexData
  } catch (e) {
    throw new Error('invalid initial data: ' + e)
  }
}

export const initialData: IndexData = readInitialData()

// The URI prefix (from the injected data) is used to build absolute request
// URLs so a `--path-prefix` deployment keeps working.
export const uriPrefix: string = initialData.uri_prefix || '/'

// Build an absolute URL path from a relative path (relative to serve root).
// Used for API calls, downloads, uploads — i.e. real server URLs.
export function absUrl(relPath: string, query?: string): string {
  // uriPrefix is "/" or "/prefix/" (always starts and ends with "/").
  const prefix = uriPrefix === '/' ? '' : uriPrefix.slice(0, -1)
  const p = relPath === '/' ? '' : relPath
  const q = query ? '?' + query : ''
  let result = prefix + p
  if (result === '') result = '/'
  return result + q
}

// ---- 前端路由 ----
// 目录导航使用真实路径（History API），因此地址栏里就是可直接分享、可刷新的
// 规范地址，例如 /sglang-das/docker/configs —— 服务端对每个目录都渲染同一份
// SPA，刷新不会 404。早期版本把完整路径又写进 hash，导致 pathname 与 hash 内容
// 重复（/a/b#/a/b），这里不再产生那种地址。
//
// 例外：帮助页是纯前端路由，服务端没有对应目录，只能放在 hash 里。

// 目录对应的规范地址（含 --path-prefix 前缀）。
export function dirUrl(relPath: string): string {
  return absUrl(relPath)
}

// 解析旧式 hash 路由（#/a/b）。旧书签仍可打开，随后会被规范化成真实路径。
export function parseHash(): string {
  const h = location.hash
  if (!h || h === '#') return '/'
  let p = h.slice(1)
  if (!p.startsWith('/')) p = '/' + p
  if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1)
  return p || '/'
}

// 解析当前地址对应的目录路径（相对服务根），已去掉 uriPrefix。
export function pathFromLocation(): string {
  const prefix = uriPrefix === '/' ? '' : uriPrefix.slice(0, -1)
  let p = location.pathname
  if (prefix && (p === prefix || p.startsWith(prefix + '/'))) p = p.slice(prefix.length)
  const out = normalizeRelPath(p || '/')
  // 保留根路径末端的斜杠语义统一为 "/"
  return out === '//' ? '/' : out
}

// Fetch a directory listing for the given relative path.
export async function listDir(relPath: string, params: ListParams): Promise<IndexData> {
  const qs = new URLSearchParams()
  qs.set('json', '')
  if (params.q) qs.set('q', params.q)
  if (params.sort) qs.set('sort', params.sort)
  if (params.order) qs.set('order', params.order)
  if (params.dirsOnly) qs.set('dirs', '')
  const res = await fetch(absUrl(relPath, qs.toString()))
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
  return (await res.json()) as IndexData
}

// Absolute download/preview URL for a file.
export function fileUrl(relPath: string): string {
  return absUrl(relPath)
}

export interface UploadProgress {
  loaded: number
  total: number
}

export async function uploadFile(
  relPath: string,
  file: File,
  onProgress?: (p: UploadProgress) => void,
): Promise<void> {
  const res = await new Promise<Response>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', absUrl(relPath))
    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress({ loaded: e.loaded, total: e.total })
      }
    })
    xhr.addEventListener('load', () => {
      resolve(new Response(xhr.responseText, { status: xhr.status, statusText: xhr.statusText }))
    })
    xhr.addEventListener('error', () => reject(new Error('upload failed')))
    xhr.addEventListener('abort', () => reject(new Error('upload aborted')))
    xhr.send(file)
  })
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
}

// Generic request helper for WebDAV-ish methods.
export async function api(
  method: string,
  relPath: string,
  body?: BodyInit | null,
  headers?: Record<string, string>,
): Promise<Response> {
  const res = await fetch(absUrl(relPath), { method, body: body ?? null, headers })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res
}

export async function deletePath(relPath: string): Promise<void> {
  const res = await fetch(absUrl(relPath), { method: 'DELETE' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

export async function mkdir(relPath: string): Promise<void> {
  const res = await fetch(absUrl(relPath), { method: 'MKCOL' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

export async function createFile(relPath: string): Promise<void> {
  const res = await fetch(absUrl(relPath), { method: 'PUT', body: '' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

// MOVE/COPY Destination header must be URI-encoded so non-ASCII (e.g. Chinese)
// filenames don't break the header.
export async function movePath(src: string, dest: string): Promise<void> {
  const res = await fetch(absUrl(src), {
    method: 'MOVE',
    headers: { Destination: encodeURI(location.origin + absUrl(dest)) },
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

export async function copyPath(src: string, dest: string): Promise<void> {
  const res = await fetch(absUrl(src), {
    method: 'COPY',
    headers: { Destination: encodeURI(location.origin + absUrl(dest)) },
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

export async function saveFile(relPath: string, content: string): Promise<void> {
  const res = await fetch(absUrl(relPath), { method: 'PUT', body: content })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

export async function fileHash(relPath: string): Promise<string> {
  const res = await fetch(absUrl(relPath, 'hash'))
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.text()
}

export async function readText(relPath: string, maxBytes = 2 * 1024 * 1024): Promise<string> {
  const res = await fetch(absUrl(relPath))
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  const len = Number(res.headers.get('Content-Length') || 0)
  if (len > maxBytes) {
    throw new Error('FILE_TOO_LARGE')
  }
  const text = await res.text()
  if (text.length > maxBytes) {
    throw new Error('FILE_TOO_LARGE')
  }
  return text
}
