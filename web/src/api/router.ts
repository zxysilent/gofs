// 前端路由：目录导航使用真实路径（History API），地址栏里就是可直接分享、
// 可刷新的规范地址，例如 /sglang-das/docker/configs —— 服务端对每个目录都渲染
// 同一份 SPA，刷新不会 404。早期版本把完整路径又写进 hash，导致 pathname 与
// hash 内容重复（/a/b#/a/b），这里不再产生那种地址。
//
// 例外：帮助页是纯前端路由，服务端没有对应目录，只能放在 hash 里。
import { absUrl, uriPrefix } from './index'
import { normalizeRelPath } from '../utils/paths'

// 特殊前端路由：curl 使用手册页（不会请求服务端目录数据）。
export const HELP_ROUTE = '/__gofs_help__'

export function isHelpRoute(p: string): boolean {
  return normalizeRelPath(p) === HELP_ROUTE
}

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
