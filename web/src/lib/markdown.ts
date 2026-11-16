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
import { esc } from '../utils/icons'

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

// ---- raw HTML ----
// Markdown 里内嵌 HTML 是常见写法（对照表、details、带样式的代码块），
// 一律转义会把它当普通文字整段显示出来。这里允许渲染，但必须清洗：
// 预览是把 HTML 注入本站页面（同源），不清洗就等于允许 md 作者执行任意
// 脚本、读取当前用户的会话。

/** 允许渲染的标签。只收排版用标签，不含任何能加载/执行代码的元素。 */
const SAFE_TAGS = new Set([
  'a', 'abbr', 'address', 'article', 'aside', 'b', 'bdi', 'bdo', 'big',
  'blockquote', 'br', 'caption', 'center', 'cite', 'code', 'col', 'colgroup',
  'dd', 'del', 'details', 'dfn', 'div', 'dl', 'dt', 'em', 'figcaption',
  'figure', 'font', 'footer', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'header',
  'hgroup', 'hr', 'i', 'img', 'ins', 'kbd', 'li', 'main', 'mark', 'nav', 'ol',
  'p', 'pre', 'q', 's', 'samp', 'section', 'small', 'span', 'strike',
  'strong', 'sub', 'summary', 'sup', 'table', 'tbody', 'td', 'tfoot', 'th',
  'thead', 'tr', 'tt', 'u', 'ul', 'var', 'wbr',
])

/** 连同内容整块丢弃：脚本、样式、嵌入对象、表单控件、可自动播放的媒体。 */
const DROP_TAGS = new Set([
  'script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button',
  'select', 'option', 'optgroup', 'textarea', 'link', 'meta', 'base',
  'template', 'noscript', 'svg', 'math', 'video', 'audio', 'source', 'track',
  'canvas', 'applet', 'frame', 'frameset', 'dialog',
])

/** 能开启一个 HTML 块的标签（对齐 CommonMark 的块级标签集合）。 */
const BLOCK_HTML_TAGS = new Set([
  'address', 'article', 'aside', 'blockquote', 'caption', 'center', 'colgroup',
  'dd', 'details', 'dialog', 'dir', 'div', 'dl', 'dt', 'fieldset', 'figcaption',
  'figure', 'footer', 'form', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'header',
  'hgroup', 'hr', 'iframe', 'li', 'main', 'menu', 'nav', 'noscript', 'ol', 'p',
  'pre', 'script', 'section', 'style', 'summary', 'table', 'tbody', 'td',
  'tfoot', 'th', 'thead', 'tr', 'ul', 'svg', 'video', 'audio',
])

const VOID_TAGS = new Set([
  'area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta',
  'param', 'source', 'track', 'wbr',
])

/** 需要按 URL 处理（可触发请求或跳转）的属性。 */
const URL_ATTRS = new Set(['href', 'src', 'xlink:href', 'action', 'poster', 'cite', 'data'])

// 标签匹配：优先吃掉引号内的 > ，避免把属性值里的尖括号当成标签结尾。
const RE_HTML_TAG = /<\/?[a-zA-Z][a-zA-Z0-9-]*(?:"[^"]*"|'[^']*'|[^>"'])*>/g

function tagNameOf(tag: string): string {
  const m = /^<\/?([a-zA-Z][a-zA-Z0-9-]*)/.exec(tag)
  return m ? m[1].toLowerCase() : ''
}

// 只允许 http(s)/相对路径与内联位图，挡掉 javascript:/vbscript:/data:text/html。
function sanitizeUrl(raw: string, base: string): string | null {
  // 去掉控制字符与空白：`java\nscript:` 这类绕过手段都要拦。
  const v = raw.replace(/[\u0000-\u0020]/g, '')
  if (!v) return ''
  const lower = v.toLowerCase()
  if (lower.startsWith('javascript:') || lower.startsWith('vbscript:')) return null
  if (lower.startsWith('data:')) {
    // 只放行位图；data:image/svg+xml 里可以带脚本，一律拒绝。
    return /^data:image\/(png|jpe?g|gif|webp|avif|bmp);/i.test(v) ? v : null
  }
  return resolveUrl(v, base)
}

// style 属性保留（内嵌表格靠它排版），但要挡掉能执行代码的老写法。
function sanitizeStyle(raw: string): string {
  if (/expression\s*\(|javascript:|vbscript:|-moz-binding|behavior\s*:/i.test(raw)) {
    return ''
  }
  return raw
}

function cleanNode(root: ParentNode, base: string): void {
  for (const el of Array.from(root.querySelectorAll('*'))) {
    const tag = el.tagName.toLowerCase()
    if (DROP_TAGS.has(tag)) {
      el.remove()
      continue
    }
    if (!SAFE_TAGS.has(tag)) {
      // 不认识的标签：拆掉外壳，保留里面的文字与子元素。
      el.replaceWith(...Array.from(el.childNodes))
      continue
    }
    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase()
      if (name.startsWith('on') || name === 'srcdoc' || name === 'formaction') {
        el.removeAttribute(attr.name)
        continue
      }
      if (URL_ATTRS.has(name)) {
        const url = sanitizeUrl(attr.value, base)
        if (url === null) el.removeAttribute(attr.name)
        else el.setAttribute(attr.name, url)
        continue
      }
      if (name === 'style') {
        const css = sanitizeStyle(attr.value)
        if (css) el.setAttribute('style', css)
        else el.removeAttribute('style')
      }
    }
    if (tag === 'a') {
      el.setAttribute('target', '_blank')
      el.setAttribute('rel', 'noopener noreferrer')
    }
    // src 被移除的图片只会显示一个破图图标，不如整张拿掉。
    if (tag === 'img' && !el.getAttribute('src')) el.remove()
  }
}

/** 去掉标签只留文字，用来判断一段渲染结果是否等于空。 */
function stripTags(s: string): string {
  return s.replace(/<[^>]*>/g, '')
}

/** 清洗一段 HTML 并返回可安全插入页面的字符串。 */
function sanitizeHtml(html: string, base: string): string {
  if (typeof document === 'undefined') return esc(html)
  const tpl = document.createElement('template')
  tpl.innerHTML = html
  cleanNode(tpl.content, base)
  const wrap = document.createElement('div')
  wrap.appendChild(tpl.content)
  return wrap.innerHTML
}

// 单行内某个块级标签的开合差；void 与自闭合标签不计入深度。
function tagDepth(line: string, tag: string): number {
  const opens = (line.match(new RegExp(`<${tag}(?=[\\s/>])`, 'gi')) || []).length
  const closes = (line.match(new RegExp(`</${tag}\\s*>`, 'gi')) || []).length
  const selfClosed = (line.match(new RegExp(`<${tag}[^>]*/>`, 'gi')) || []).length
  return opens - closes - selfClosed
}

// takeHtmlBlock 从 start 行开始吞掉一个完整的 HTML 块。
// 按同名标签的开合深度判断结束，因此块内部的空行不会提前截断表格。
function takeHtmlBlock(lines: string[], start: number): { text: string; next: number } | null {
  const m = /^ {0,3}<([a-zA-Z][a-zA-Z0-9-]*)/.exec(lines[start])
  if (!m) return null
  const tag = m[1].toLowerCase()
  if (!BLOCK_HTML_TAGS.has(tag)) return null
  if (VOID_TAGS.has(tag)) return { text: lines[start], next: start + 1 }

  const buf = [lines[start]]
  let depth = tagDepth(lines[start], tag)
  let next = start + 1
  // 单行闭合（<div>…</div> 写在同一行）直接收工。
  while (depth > 0 && next < lines.length) {
    buf.push(lines[next])
    depth += tagDepth(lines[next], tag)
    next++
  }
  return { text: buf.join('\n'), next }
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

    // ---- HTML 注释 ----
    // 单独成行的注释直接丢弃，否则会留下一个空段落。
    if (line.trimStart().startsWith('<!--')) {
      while (i < lines.length && !lines[i].includes('-->')) i++
      i++
      continue
    }

    // ---- HTML block ----
    // 内嵌 HTML 整块保留（表格、details、带样式的对照表），但先过白名单清洗。
    if (line.trimStart().startsWith('<')) {
      const block = takeHtmlBlock(lines, i)
      if (block) {
        // 清洗后什么都不剩（script / iframe / 纯注释）就不要留空壳。
        const html = sanitizeHtml(block.text, ctx.base).trim()
        if (html) out.push(`<div class="md-html">${html}</div>`)
        i = block.next
        continue
      }
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
    // 只剩被清洗掉的标签（例如 javascript: 的 img）时，不要留一个空段落
    const paraHtml = buf
      .map((l) => inline(l.trim(), ctx) + (/ {2,}$/.test(l) ? '<br/>' : ''))
      .join('\n')
    if (stripTags(paraHtml).trim() === '' && !/<(img|br|hr)\b/i.test(paraHtml)) continue
    out.push(`<p>${paraHtml}</p>`)
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

  // 3) 行内 HTML：白名单标签保留并清洗，其余（如 `a < b`）照旧转义。
  // HTML 注释直接丢掉，否则会以 `<!-- … -->` 原样显示出来。
  masked = masked.replace(/<!--[\s\S]*?-->/g, '')
  masked = masked.replace(RE_HTML_TAG, (tag: string) => {
    const name = tagNameOf(tag)
    if (!name || !SAFE_TAGS.has(name)) return tag
    // 闭合标签没有属性可清，直接归一成小写；开标签交给统一清洗。
    return hold(tag.startsWith('</') ? `</${name}>` : sanitizeHtml(tag, ctx.base))
  })

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
