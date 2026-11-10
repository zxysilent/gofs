// Minimal markdown renderer (no external deps).
//
// Supports: fenced code blocks, ATX headings, GFM tables (with alignment),
// nested ordered/unordered lists, blockquotes, horizontal rules, images,
// links, bare autolinks, bold/italic/strikethrough, inline code and math
// (KaTeX-compatible placeholders, rendered by ./math.ts).
//
// ```mermaid fences become placeholders too; ./diagram.ts renders them.
//
// Images and links may be relative to the document; pass `baseUrl` (the
// absolute directory URL, ending with "/") so they resolve against it instead
// of against the current page.
import { esc } from './icons'

type Align = 'left' | 'center' | 'right'

export interface MarkdownOptions {
  /** 文档所在目录的绝对 URL，必须以 "/" 结尾。用于解析相对图片/链接。 */
  baseUrl?: string
}

/** 渲染期间的解析上下文；显式传参，避免模块级可变状态。 */
interface Ctx {
  base: string
}

const RE_FENCE = /^\s{0,3}(`{3,}|~{3,})\s*([^`\s]*)\s*$/
const RE_HEADING = /^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$/
// A thematic break: three or more of the same marker, nothing else on the line.
const RE_HR = /^\s{0,3}([-*_])(?:\s*\1){2,}\s*$/
const RE_UL = /^(\s*)[-*+]\s+(.*)$/
const RE_OL = /^(\s*)(\d{1,9})[.)]\s+(.*)$/
const RE_QUOTE = /^\s{0,3}>\s?(.*)$/
// Table delimiter row: every cell is dashes with optional alignment colons.
const RE_TABLE_SEP = /^\s*\|?\s*:?-+:?\s*(?:\|\s*:?-+:?\s*)*\|?\s*$/

const isTableDelim = (line: string): boolean =>
  line.includes('-') && line.includes('|') && RE_TABLE_SEP.test(line)

const isBlockStart = (line: string): boolean =>
  RE_FENCE.test(line) ||
  RE_HEADING.test(line) ||
  RE_HR.test(line) ||
  RE_QUOTE.test(line) ||
  RE_UL.test(line) ||
  RE_OL.test(line) ||
  isMathFence(line)

// ---- math ----
// 两种常见写法都支持：$$...$$ / \[...\] 为独立公式，$...$ / \(...\) 为行内公式。
// 这里只输出带原始 TeX 的占位节点，真正的排版交给 math.ts（按需加载 KaTeX）。
const MATH_CLASS = 'md-math'

function isMathFence(line: string): boolean {
  const t = line.trimStart()
  return t.startsWith('$$') || t.startsWith('\\[')
}

function mathNode(tex: string, display: boolean): string {
  const tag = display ? 'div' : 'span'
  return `<${tag} class="${MATH_CLASS}" data-display="${display ? '1' : '0'}">${esc(tex)}</${tag}>`
}

// ---- diagrams ----
// ```mermaid 围栏转成带原始定义的占位节点；真正的渲染在 diagram.ts 里按需进行。
const DIAGRAM_CLASS = 'md-diagram'

function diagramNode(source: string): string {
  return `<div class="${DIAGRAM_CLASS}"><pre class="md-diagram-src">${esc(source)}</pre></div>`
}

// splitRow breaks a table row into trimmed cells, honouring `\|` escapes.
function splitRow(row: string): string[] {
  let s = row.trim()
  if (s.startsWith('|')) s = s.slice(1)
  if (s.endsWith('|') && !s.endsWith('\\|')) s = s.slice(0, -1)
  const cells: string[] = []
  let cur = ''
  for (let k = 0; k < s.length; k++) {
    if (s[k] === '\\' && s[k + 1] === '|') {
      cur += '|'
      k++
      continue
    }
    if (s[k] === '|') {
      cells.push(cur.trim())
      cur = ''
      continue
    }
    cur += s[k]
  }
  cells.push(cur.trim())
  return cells
}

function cellAlign(delim: string): Align | '' {
  const s = delim.trim()
  const left = s.startsWith(':')
  const right = s.endsWith(':')
  if (left && right) return 'center'
  if (right) return 'right'
  if (left) return 'left'
  return ''
}

export function renderMarkdown(src: string, opts?: MarkdownOptions): string {
  const ctx: Ctx = { base: opts?.baseUrl ?? '' }
  const lines = src.replace(/\r\n?/g, '\n').split('\n')
  const out: string[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    // ---- fenced code ----
    const fence = RE_FENCE.exec(line)
    if (fence) {
      const marker = fence[1]
      const lang = fence[2]
      const buf: string[] = []
      i++
      while (i < lines.length && !lines[i].trim().startsWith(marker)) {
        buf.push(lines[i])
        i++
      }
      i++ // consume the closing fence
      // ```mermaid：输出带原始定义的占位节点，图由 diagram.ts 按需渲染。
      // 这里不直接渲染，Mermaid 体积远大于应用本身，且渲染是异步的。
      if (lang.toLowerCase() === 'mermaid') {
        out.push(diagramNode(buf.join('\n')))
        continue
      }
      const cls = lang ? ` class="language-${esc(lang)}"` : ''
      out.push(`<pre><code${cls}>${esc(buf.join('\n'))}</code></pre>`)
      continue
    }

    // ---- blank ----
    if (line.trim() === '') {
      i++
      continue
    }

    // ---- block math ----
    // $$...$$ 或 \[...\]，可跨行；单独成块，不与段落混排。
    if (isMathFence(line)) {
      const fenceText = line.trimStart()
      const open = fenceText.startsWith('$$') ? '$$' : '\\['
      const close = open === '$$' ? '$$' : '\\]'
      const rest = fenceText.slice(open.length)
      const trimmedRest = rest.trimEnd()
      // 单行写法：$$x$$ / \[x\]
      if (trimmedRest.endsWith(close) && trimmedRest.length >= close.length) {
        out.push(mathNode(trimmedRest.slice(0, -close.length).trim(), true))
        i++
        continue
      }
      const buf: string[] = []
      if (rest.trim() !== '') buf.push(rest)
      i++
      while (i < lines.length) {
        const cur = lines[i]
        const t = cur.trimEnd()
        if (t.endsWith(close)) {
          buf.push(t.slice(0, -close.length))
          i++
          break
        }
        buf.push(cur)
        i++
      }
      out.push(mathNode(buf.join('\n').trim(), true))
      continue
    }

    // ---- GFM table ----
    if (line.includes('|') && i + 1 < lines.length && isTableDelim(lines[i + 1])) {
      const header = splitRow(line)
      const aligns = splitRow(lines[i + 1]).map(cellAlign)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && lines[i].trim() !== '' && lines[i].includes('|')) {
        rows.push(splitRow(lines[i]))
        i++
      }
      const cell = (tag: 'th' | 'td', text: string, idx: number): string => {
        const a = aligns[idx]
        return `<${tag}${a ? ` class="md-align-${a}"` : ''}>${inline(text, ctx)}</${tag}>`
      }
      const head = header.map((c, idx) => cell('th', c, idx)).join('')
      const body = rows
        .map((r) => `<tr>${header.map((_, idx) => cell('td', r[idx] ?? '', idx)).join('')}</tr>`)
        .join('')
      out.push(
        `<div class="md-table"><table><thead><tr>${head}</tr></thead><tbody>${body}</tbody></table></div>`,
      )
      continue
    }

    // ---- heading ----
    const heading = RE_HEADING.exec(line)
    if (heading) {
      const lvl = heading[1].length
      out.push(`<h${lvl}>${inline(heading[2], ctx)}</h${lvl}>`)
      i++
      continue
    }

    // ---- thematic break ----
    if (RE_HR.test(line)) {
      out.push('<hr/>')
      i++
      continue
    }

    // ---- blockquote ----
    if (RE_QUOTE.test(line)) {
      const buf: string[] = []
      while (i < lines.length) {
        const q = RE_QUOTE.exec(lines[i])
        if (!q) break
        buf.push(q[1])
        i++
      }
      out.push(`<blockquote>${buf.map((l) => inline(l, ctx)).join('<br/>')}</blockquote>`)
      continue
    }

    // ---- lists ----
    if (RE_UL.test(line) || RE_OL.test(line)) {
      i = renderList(lines, i, out, ctx)
      continue
    }

    // ---- paragraph ----
    const buf: string[] = []
    while (i < lines.length) {
      const cur = lines[i]
      if (cur.trim() === '' || isBlockStart(cur)) break
      if (cur.includes('|') && i + 1 < lines.length && isTableDelim(lines[i + 1])) break
      buf.push(cur)
      i++
    }
    if (buf.length === 0) {
      // Defensive: never stall on an unclassified line.
      i++
      continue
    }
    out.push(
      `<p>${buf.map((l) => inline(l.trim(), ctx) + (/ {2,}$/.test(l) ? '<br/>' : '')).join('\n')}</p>`,
    )
  }

  return out.join('\n')
}

// renderList walks a whole list block, tracking indentation so nested lists
// can be emitted inside their parent <li>.
function renderList(lines: string[], start: number, out: string[], ctx: Ctx): number {
  const stack: { indent: number; tag: 'ul' | 'ol' }[] = []
  let openLi = false
  const closeLi = () => {
    if (openLi) {
      out.push('</li>')
      openLi = false
    }
  }

  let i = start
  while (i < lines.length) {
    const line = lines[i]

    if (line.trim() === '') {
      // A blank line only ends the list if the next line is not a list item.
      const next = lines[i + 1]
      if (next === undefined || (!RE_UL.test(next) && !RE_OL.test(next))) break
      i++
      continue
    }

    const ul = RE_UL.exec(line)
    const ol = ul ? null : RE_OL.exec(line)
    if (!ul && !ol) {
      // Lazy continuation: an indented, non-blank line that is not itself a
      // block start belongs to the item above, so keep the <li> open.
      if (openLi && !isBlockStart(line)) {
        out.push(inline(line.trim(), ctx))
        i++
        continue
      }
      break
    }

    const indent = (ul ? ul[1] : ol![1]).replace(/\t/g, '  ').length
    const tag: 'ul' | 'ol' = ul ? 'ul' : 'ol'
    const text = ul ? ul[2] : ol![3]

    while (stack.length > 0 && indent < stack[stack.length - 1].indent) {
      closeLi()
      out.push(`</${stack.pop()!.tag}>`)
    }

    const top = stack[stack.length - 1]
    if (!top || indent > top.indent) {
      // Deeper indent: nest the new list inside the still-open parent <li>.
      stack.push({ indent, tag })
      out.push(`<${tag}>`)
    } else {
      closeLi()
      if (top.tag !== tag) {
        out.push(`</${top.tag}><${tag}>`)
        top.tag = tag
      }
    }

    out.push(`<li>${inline(text, ctx)}`)
    openLi = true
    i++
  }

  closeLi()
  while (stack.length > 0) out.push(`</${stack.pop()!.tag}>`)
  return i
}

// Code spans and math are swapped for placeholders before formatting runs, so
// their contents are never re-interpreted, while emphasis may still wrap them
// (e.g. **`code`** -> <strong><code>code</code></strong>).
const HOLD_MARK = '\u0000'

function inline(s: string, ctx: Ctx): string {
  const held: string[] = []
  const hold = (html: string): string => `${HOLD_MARK}${held.push(html) - 1}${HOLD_MARK}`

  // 1) code spans first: `$` inside code must not start a formula.
  let masked = s.replace(/(`+)([\s\S]*?)\1/g, (_m, _f, code: string) =>
    hold(`<code>${esc(code.trim())}</code>`),
  )
  // 2) math. 先处理 $$...$$，否则会被当成两对行内 $...$。
  masked = masked.replace(/\$\$([\s\S]+?)\$\$/g, (_m, tex: string) => hold(mathNode(tex.trim(), true)))
  masked = masked.replace(/\\\[([\s\S]+?)\\\]/g, (_m, tex: string) => hold(mathNode(tex.trim(), true)))
  masked = masked.replace(/\\\(([\s\S]+?)\\\)/g, (_m, tex: string) => hold(mathNode(tex.trim(), false)))
  // 行内 $...$：开头后不能是空白、结尾前不能是空白、结束符后不能紧跟数字，
  // 这样 "$5 和 $10" 之类的金额不会被误判成公式。
  masked = masked.replace(/\$(?=\S)([^$\n]*?\S)\$(?!\d)/g, (_m, tex: string) => hold(mathNode(tex, false)))

  return inlineText(masked, ctx).replace(
    /\u0000(\d+)\u0000/g,
    (_m, idx: string) => held[Number(idx)] ?? '',
  )
}

// resolveUrl 把文档内的相对地址解析成可直接请求的绝对路径。
// 已是协议地址、协议相对地址、根路径或纯锚点时原样返回。
function resolveUrl(href: string, base: string): string {
  if (!href) return href
  if (/^[a-z][a-z0-9+.-]*:/i.test(href) || href.startsWith('//') || href.startsWith('#')) {
    return href
  }
  if (href.startsWith('/')) return href
  if (!base) return href
  try {
    // 借一个固定 origin 解析 ./ 与 ../，再取回路径部分。
    const u = new URL(href, 'http://md.local' + base)
    return u.pathname + u.search + u.hash
  } catch {
    return href
  }
}

function inlineText(s: string, ctx: Ctx): string {
  return esc(s)
    .replace(
      /!\[([^\]]*)\]\(([^)\s]+)\)/g,
      // 不用 loading="lazy"：预览是弹窗（position: fixed），Chromium 对这类
      // 容器内的懒加载图片会一直推迟请求，图片永久停在 2x2 占位状态。
      (_m, alt, href) => `<img alt="${alt}" src="${resolveUrl(href, ctx.base)}" />`,
    )
    .replace(
      /\[([^\]]*)\]\(([^)\s]+)\)/g,
      (_m, text, href) =>
        `<a href="${resolveUrl(href, ctx.base)}" target="_blank" rel="noopener">${text || href}</a>`,
    )
    .replace(
      /(^|[\s(])(https?:\/\/[^\s<)]+)/g,
      (_m, lead, url) => `${lead}<a href="${url}" target="_blank" rel="noopener">${url}</a>`,
    )
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/__([^_]+)__/g, '<strong>$1</strong>')
    .replace(/~~([^~]+)~~/g, '<del>$1</del>')
    .replace(/(^|[^*\w])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>')
}
