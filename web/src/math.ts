// 公式排版：把 markdown.ts 输出的 .md-math 占位节点替换为 KaTeX 渲染结果。
//
// KaTeX（含字体）体积远大于整个应用其余部分，因此按需加载：文档里没有公式时
// 一个字节都不会下载。加载失败也只会保留原始 TeX 文本，不会破坏预览。
type KatexLike = {
  renderToString(tex: string, opts: Record<string, unknown>): string
}

let katexPromise: Promise<KatexLike | null> | null = null

function loadKatex(): Promise<KatexLike | null> {
  if (!katexPromise) {
    katexPromise = Promise.all([import('katex'), import('katex/dist/katex.min.css')])
      .then(([mod]) => ((mod as unknown as { default?: KatexLike }).default ?? mod) as KatexLike)
      .catch(() => null)
  }
  return katexPromise
}

// 是否需要加载 KaTeX（避免在无公式的文档上触发下载）。
// renderMath 自己会在没有节点时直接返回，这里不再单独暴露判断函数。

/**
 * renderMath 就地渲染 root 内尚未处理的公式节点。
 * 失败时保留原始 TeX，保证内容始终可见。
 */
export async function renderMath(root: HTMLElement | null | undefined): Promise<void> {
  if (!root) return
  const nodes = Array.from(root.querySelectorAll<HTMLElement>('.md-math'))
  if (nodes.length === 0) return

  const katex = await loadKatex()
  if (!katex) return

  for (const el of nodes) {
    if (el.dataset.rendered === '1') continue
    const tex = el.textContent ?? ''
    const display = el.dataset.display === '1'
    try {
      el.innerHTML = katex.renderToString(tex, {
        displayMode: display,
        throwOnError: false,
        strict: 'ignore',
        // 不让 KaTeX 输出 MathML，避免同一公式在复制时出现两份文本。
        output: 'html',
      })
      el.dataset.rendered = '1'
      if (display) el.classList.add('md-math-block')
    } catch {
      // 语法错误等：保留占位节点里的原始 TeX。
    }
  }
}
