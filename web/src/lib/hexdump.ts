// 二进制预览用的 hexdump 渲染。
//
// 输出经典三列布局：偏移 | 16 字节十六进制 | ASCII。按行返回纯文本，由调用方
// 决定用插值还是 v-html 渲染 —— 这里不做 HTML 转义，避免和 Vue 的自动转义重复。

const BYTES_PER_LINE = 16

function hex8(n: number): string {
  return n.toString(16).padStart(8, '0')
}

// 可打印 ASCII 保留原字符，其余（控制字符、多字节 UTF-8）显示为 '.'，
// 和 hexdump -C 的习惯一致：不试图猜编码，只保证列宽稳定。
function asciiOf(byte: number): string {
  return byte >= 0x20 && byte < 0x7f ? String.fromCharCode(byte) : '.'
}

export function hexDumpLines(bytes: Uint8Array, startOffset = 0): string[] {
  const lines: string[] = []
  for (let i = 0; i < bytes.length; i += BYTES_PER_LINE) {
    const end = Math.min(i + BYTES_PER_LINE, bytes.length)
    let hex = ''
    let ascii = ''
    for (let j = 0; j < BYTES_PER_LINE; j++) {
      if (i + j < end) {
        const b = bytes[i + j]
        hex += b.toString(16).padStart(2, '0') + ' '
        ascii += asciiOf(b)
      } else {
        // 末行不足 16 字节时补空格，让 ASCII 列对齐
        hex += '   '
      }
      // 8 字节后加一个空格，肉眼数位更方便
      if (j === 7) hex += ' '
    }
    lines.push(`${hex8(startOffset + i)}  ${hex} |${ascii}|`)
  }
  return lines
}
