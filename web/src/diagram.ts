// 流程图 / 时序图等 Mermaid 图表的渲染。
//
// Mermaid 打包后超过 3 MB（100+ 依赖），远大于应用其余部分，因此和 KaTeX
// 一样按需加载：文档里没有 ```mermaid 围栏时一个字节都不会下载。
//
// 渲染在持有原始定义的占位节点上进行；失败（语法错误等）时保留源码并
// 提示原因，不会让整篇文档变成空白。
type MermaidLike = {
  initialize(config: Record<string, unknown>): void
  render(id: string, text: string): Promise<{ svg: string }>
}

let mermaidPromise: Promise<MermaidLike | null> | null = null
let lastTheme = ''

// 同一主题下复用同一个实例；主题切换时重新初始化以换配色。
async function loadMermaid(theme: 'light' | 'dark'): Promise<MermaidLike | null> {
  if (!mermaidPromise || lastTheme !== theme) {
    lastTheme = theme
    mermaidPromise = import('mermaid')
      .then((mod) => {
        const api = ((mod as unknown as { default?: MermaidLike }).default ?? mod) as MermaidLike
        api.initialize({
          startOnLoad: false,
          // strict 会过滤脚本与内联事件，预览不受信任的 Markdown 时必须如此
          securityLevel: 'strict',
          theme: theme === 'dark' ? 'dark' : 'default',
          fontFamily:
            "-apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', Roboto, sans-serif",
          // 出错时自己接管提示，不让 Mermaid 往页面里插它的错误图
          suppressErrorRendering: true,
        })
        return api
      })
      .catch(() => null)
  }
  return mermaidPromise
}

/** 文档里是否有待渲染的图表（避免无谓的下载）。 */

let seq = 0
// 渲染后 DOM 里只剩 SVG，源码另存一份，便于主题切换时重新着色。
const sources = new WeakMap<HTMLElement, string>()
let renderedTheme = ''

/**
 * renderDiagrams 就地渲染 root 内尚未处理的图表占位节点。
 * theme 决定明暗配色；失败时保留源码并标注错误。
 */
export async function renderDiagrams(
  root: HTMLElement | null | undefined,
  theme: 'light' | 'dark',
): Promise<void> {
  if (!root) return
  const nodes = Array.from(root.querySelectorAll<HTMLElement>('.md-diagram'))

  // Mermaid 的配色在初始化时固定下来，主题变了只能按源码重画一遍。
  if (renderedTheme && renderedTheme !== theme) {
    for (const el of nodes) {
      const src = sources.get(el)
      if (src === undefined) continue
      el.innerHTML = `<pre class="md-diagram-src">${escapeHtml(src)}</pre>`
      el.dataset.state = ''
    }
  }
  renderedTheme = theme

  const pending = nodes.filter((n) => n.dataset.state !== 'done' && n.dataset.state !== 'failed')
  if (pending.length === 0) return

  const mermaid = await loadMermaid(theme)
  if (!mermaid) return

  for (const el of pending) {
    const pre = el.querySelector('.md-diagram-src')
    const src = sources.get(el) ?? pre?.textContent ?? ''
    if (!src.trim()) continue
    sources.set(el, src)
    const id = `gofs-diagram-${++seq}`
    try {
      const { svg } = await mermaid.render(id, src)
      el.innerHTML = svg
      el.dataset.state = 'done'
    } catch (e) {
      // 保留源码（便于复制修改），并把错误原因写在下面。
      el.innerHTML = `<pre class="md-diagram-src">${escapeHtml(src)}</pre>`
      el.dataset.state = 'failed'
      const msg = e instanceof Error ? e.message : String(e)
      const hint = document.createElement('p')
      hint.className = 'md-diagram-error'
      hint.textContent = msg.split('\n')[0]
      el.appendChild(hint)
      // mermaid 渲染失败时可能残留临时节点
      document.getElementById('d' + id)?.remove()
      document.getElementById(id)?.remove()
    }
  }
}

function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c] as string)
}
