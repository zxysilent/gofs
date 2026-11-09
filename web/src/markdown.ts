// Minimal markdown renderer (no external deps).
import { esc } from './icons'

export function renderMarkdown(src: string): string {
  const lines = src.replace(/\r\n/g, '\n').split('\n')
  let html = ''
  let inCode = false
  let codeBuf: string[] = []
  let inList = false
  let listType = ''

  const inline = (s: string): string =>
    esc(s)
      .replace(/`([^`]+)`/g, '<code>$1</code>')
      .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
      .replace(/\*([^*]+)\*/g, '<em>$1</em>')
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')

  const closeList = () => {
    if (inList) {
      html += `</${listType}>`
      inList = false
    }
  }

  for (const line of lines) {
    if (line.trim().startsWith('```')) {
      if (inCode) {
        html += `<pre><code>${esc(codeBuf.join('\n'))}</code></pre>`
        codeBuf = []
        inCode = false
      } else {
        closeList()
        inCode = true
      }
      continue
    }
    if (inCode) {
      codeBuf.push(line)
      continue
    }

    if (/^\s*[-*+]\s+/.test(line)) {
      if (!inList || listType !== 'ul') {
        closeList()
        html += '<ul>'
        inList = true
        listType = 'ul'
      }
      html += `<li>${inline(line.replace(/^\s*[-*+]\s+/, ''))}</li>`
      continue
    }
    if (/^\s*\d+\.\s+/.test(line)) {
      if (!inList || listType !== 'ol') {
        closeList()
        html += '<ol>'
        inList = true
        listType = 'ol'
      }
      html += `<li>${inline(line.replace(/^\s*\d+\.\s+/, ''))}</li>`
      continue
    }
    closeList()
    if (/^#{1,6}\s+/.test(line)) {
      const lvl = (line.match(/^#+/) as RegExpMatchArray)[0].length
      html += `<h${lvl}>${inline(line.replace(/^#+\s+/, ''))}</h${lvl}>`
    } else if (/^>\s?/.test(line)) {
      html += `<blockquote>${inline(line.replace(/^>\s?/, ''))}</blockquote>`
    } else if (/^\s*---+\s*$/.test(line)) {
      html += '<hr/>'
    } else if (line.trim() === '') {
      // skip
    } else {
      html += `<p>${inline(line)}</p>`
    }
  }
  if (inCode) html += `<pre><code>${esc(codeBuf.join('\n'))}</code></pre>`
  closeList()
  return html
}
