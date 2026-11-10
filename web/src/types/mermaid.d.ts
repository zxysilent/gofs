// Mermaid 的最小类型声明：只在运行时动态 import，且仅用到 initialize/render。
// 只做环境声明，不要在此导出类型——否则本文件变成模块，
// 里面的 declare module 'mermaid' 会退化成模块增强而失效。
declare module 'mermaid' {
  const mermaid: {
    initialize(config: Record<string, unknown>): void
    render(id: string, text: string): Promise<{ svg: string; bindFunctions?: (el: Element) => void }>
  }
  export default mermaid
}
