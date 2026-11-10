// 构建后处理：KaTeX 的 CSS 为每个字体同时声明 woff2 / woff / ttf，
// 现代浏览器只会用 woff2，另外两种纯属让产物（和嵌入的二进制）白白变大。
// 这里在每个 @font-face 里只保留 woff2 那一条 src，并删掉对应的字体文件。
//
// 只在 dist 内操作，且基于文本替换 —— KaTeX 升级后文件名/顺序变了也能工作。
// 注意：小于 assetsInlineLimit 的字体会被 Vite 内联成 data:font/woff2;base64，
// 所以不能靠 ".woff2" 后缀判断，必须看 format("woff2")。
import { readdirSync, readFileSync, writeFileSync, unlinkSync, statSync } from 'node:fs'
import { join } from 'node:path'

const dir = new URL('../dist/assets/', import.meta.url).pathname

const isLegacyFont = (f) => /\.(woff|ttf)$/i.test(f)

const files = readdirSync(dir)
const css = files.find((f) => /^katex.*\.css$/.test(f))
if (!css) {
  console.log('trim-katex-fonts: no katex css found, nothing to do')
  process.exit(0)
}

const path = join(dir, css)
const before = readFileSync(path, 'utf8')

// src 可能形如：
//   src:url(...woff2) format("woff2"),url(...woff) format("woff"),url(...ttf) format("truetype")
// 做法：把 src 的值截断到第一个 format("woff2") 为止。
//
// 两个坑：
// 1) 不能只删「下一条 fallback」—— 删掉第一段后第二段的逗号成了孤立的，
//    后续 fallback 不再匹配，ttf 会被留下。
// 2) 边界不能用 ";"：被 Vite 内联的字体是 data:font/woff2;base64,...，
//    其中的分号会提前截断，导致该 @font-face 整条被跳过。
let kept = 0
const after = before.replace(/src:([^}]*)/g, (whole, value) => {
  const at = value.indexOf('format("woff2")')
  if (at < 0) return whole
  const trimmed = value.slice(0, at + 'format("woff2")'.length)
  if (trimmed !== value) kept++
  return 'src:' + trimmed
})

if (after === before) {
  console.log('trim-katex-fonts: no woff/ttf fallbacks found')
  process.exit(0)
}

writeFileSync(path, after)

let removed = 0
let freed = 0
for (const f of files) {
  if (!isLegacyFont(f)) continue
  const p = join(dir, f)
  freed += statSync(p).size
  unlinkSync(p)
  removed++
}

console.log(
  `trim-katex-fonts: trimmed ${kept} @font-face rules, removed ${removed} files (${Math.round(freed / 1024)} kB)`,
)
