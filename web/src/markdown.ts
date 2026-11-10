// Minimal markdown renderer (no external deps).
//
// Supports: fenced code blocks, ATX headings, GFM tables (with alignment),
// nested ordered/unordered lists, blockquotes, horizontal rules, images,
// links, bare autolinks, bold/italic/strikethrough and inline code.
// Inline code is protected from further formatting.
import { esc } from './icons'

type Align = 'left' | 'center' | 'right'

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
  RE_OL.test(line)

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

export function renderMarkdown(src: string): string {
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
      const cls = lang ? ` class="language-${esc(lang)}"` : ''
      out.push(`<pre><code${cls}>${esc(buf.join('\n'))}</code></pre>`)
      continue
    }

    // ---- blank ----
    if (line.trim() === '') {
      i++
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
        return `<${tag}${a ? ` class="md-align-${a}"` : ''}>${inline(text)}</${tag}>`
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
      out.push(`<h${lvl}>${inline(heading[2])}</h${lvl}>`)
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
      out.push(`<blockquote>${buf.map(inline).join('<br/>')}</blockquote>`)
      continue
    }

    // ---- lists ----
    if (RE_UL.test(line) || RE_OL.test(line)) {
      i = renderList(lines, i, out)
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
      `<p>${buf.map((l) => inline(l.trim()) + (/ {2,}$/.test(l) ? '<br/>' : '')).join('\n')}</p>`,
    )
  }

  return out.join('\n')
}

// renderList walks a whole list block, tracking indentation so nested lists
// can be emitted inside their parent <li>.
function renderList(lines: string[], start: number, out: string[]): number {
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
        out.push(inline(line.trim()))
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

    out.push(`<li>${inline(text)}`)
    openLi = true
    i++
  }

  closeLi()
  while (stack.length > 0) out.push(`</${stack.pop()!.tag}>`)
  return i
}

// Code spans are swapped for placeholders before formatting runs, so their
// contents are never re-interpreted, while emphasis may still wrap them
// (e.g. **`code`** -> <strong><code>code</code></strong>).
const CODE_MARK = '\u0000'

function inline(s: string): string {
  const codes: string[] = []
  const masked = s.replace(/(`+)([\s\S]*?)\1/g, (_m, _fence, code: string) => {
    codes.push(`<code>${esc(code.trim())}</code>`)
    return `${CODE_MARK}${codes.length - 1}${CODE_MARK}`
  })
  return inlineText(masked).replace(
    /\u0000(\d+)\u0000/g,
    (_m, idx: string) => codes[Number(idx)] ?? '',
  )
}

function inlineText(s: string): string {
  return esc(s)
    .replace(
      /!\[([^\]]*)\]\(([^)\s]+)\)/g,
      (_m, alt, href) => `<img alt="${alt}" src="${href}" loading="lazy" />`,
    )
    .replace(
      /\[([^\]]*)\]\(([^)\s]+)\)/g,
      (_m, text, href) => `<a href="${href}" target="_blank" rel="noopener">${text || href}</a>`,
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
