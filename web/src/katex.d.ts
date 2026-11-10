// KaTeX 只按需使用少数几个选项，且只在运行时动态 import，
// 因此手写最小声明，不额外引入 @types/katex。
declare module 'katex' {
  const katex: {
    renderToString(tex: string, options?: Record<string, unknown>): string
  }
  export default katex
}

declare module 'katex/dist/katex.min.css'
