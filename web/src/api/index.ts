import type { IndexData, ListParams } from '../types'
import { normalizeRelPath } from '../utils/paths'

// The server injects the initial directory data as a base64-encoded JSON
// string into `window.__GOFS_DATA__` (via a placeholder in index.html), and the
// build version into `window.__GOFS_VERSION__`.
declare global {
  interface Window {
    __GOFS_DATA__?: string
    __GOFS_VERSION__?: string
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

// Server version, injected at render time. Falls back to the unreplaced
// placeholder being visible only in a broken build, so an empty string is the
// honest value when it is missing.
export const serverVersion: string = window.__GOFS_VERSION__ || ''

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

// 按字节区间读取，用于二进制文件的 hex 预览。
// 服务端实现了 RFC 7233 的 Range（并把超界的 last-byte-pos 裁剪到文件末尾），
// 所以几 GB 的文件也只传输这一段，不会把内存和带宽吃满。
// 小文件会被裁剪成整个文件并返回 206，行为一致；只有空文件会退化成 200。
export async function readBytes(relPath: string, start: number, end: number): Promise<Uint8Array> {
  const res = await fetch(absUrl(relPath), { headers: { Range: `bytes=${start}-${end}` } })
  // 200 表示服务端忽略了 Range（例如上游代理去掉了头）：为防止把整个大文件
  // 读进内存，只在文件本身很小时才接受。
  if (res.status === 200) {
    const len = Number(res.headers.get('Content-Length') || 0)
    if (len > 4 * 1024 * 1024) throw new Error('RANGE_UNSUPPORTED')
  } else if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`)
  }
  return new Uint8Array(await res.arrayBuffer())
}
