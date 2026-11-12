<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import type { PathItem } from '../types'
import { parentRelPath } from '../utils/paths'
import { fileKind, formatSize, formatTime } from '../utils'
import { absUrl, readText, readBytes, fileHash, saveFile, MAX_TEXT_BYTES } from '../api'
import { renderMarkdown } from '../lib/markdown'
import { hexDumpLines } from '../lib/hexdump'
import { renderMath } from '../lib/math'
import { renderDiagrams } from '../lib/diagram'
import { ICONS } from '../utils/icons'
import { showToast, showConfirmDialog, copyText } from '../utils/dialog'
import { t } from '../locales'
import { resolvedTheme } from '../utils/theme'

const props = defineProps<{ item: PathItem; path: string; allowUpload: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const kind = fileKind(props.item.name)
const url = absUrl(props.path)

// 文档所在目录的绝对 URL（以 / 结尾），用于把 md 里的相对图片/链接解析成
// 真实地址；否则它们会相对当前页面路径，深层目录下的图片全都 404。
const docBaseUrl = (() => {
  const dir = absUrl(parentRelPath(props.path))
  return dir.endsWith('/') ? dir : dir + '/'
})()

// 可编辑的文本类文件
const editable = computed(() =>
  kind === 'text' || kind === 'code' || kind === 'json' || kind === 'markdown' || kind === 'html',
)

// 能在弹窗内直接看到内容的类型；其余（压缩包、未知二进制、读取失败）只提供下载。
const inlinePreview = computed(() => editable.value || ['image', 'video', 'audio', 'pdf'].includes(kind))

// 不能直接预览的二进制改用 hexdump：按 Range 分窗拉取，看几行魔数不需要下载
// 整个文件。窗口固定 4 KiB（正好 256 行），想看别处就输入偏移量跳过去 ——
// 不做无限追加，避免几 GB 的文件把 DOM 和内存拖垮。
const hexMode = computed(() => !inlinePreview.value)
const HEX_WINDOW = 4 * 1024
const HEX_ALIGN = 16
const hexLines = ref<string[]>([])
const hexOffset = ref(0) // 当前窗口起始偏移，始终按 16 字节对齐
const hexLoaded = ref(0) // 当前窗口实际读到的字节数
const hexLoading = ref(false)
const hexError = ref('')
const hexInput = ref('')

const bodyHtml = ref<string>('')
const bodyError = ref('')
const loading = ref(true)
const hash = ref('')

// 编辑状态
const editing = ref(false)
const editContent = ref('')
const originalContent = ref('')
const saving = ref(false)
const mdPreview = ref(false) // markdown 编辑时的预览开关
// HTML 默认渲染成网页，可切回源码（看 meta / 样式等）。
const htmlPreview = ref(true)
// 源码是否确实读进来了。读不到时（比如超过 2MiB）禁止进入编辑与源码视图，
// 否则编辑器是空的，一保存就把原文件截成 0 字节。
const hasSource = ref(false)
// 自动换行默认开启：代码/日志常见超长行，不换行的话要横向拖动才看得到行尾。
const wrapLines = ref(true)

const dirty = computed(() => editing.value && editContent.value !== originalContent.value)

// 只读预览里是否渲染成一块代码/文本块（用于让它铺满内容区）。
const textBody = computed(
  () => kind === 'text' || kind === 'code' || kind === 'json' || (kind === 'html' && !htmlPreview.value),
)

// 图片/视频/音频需要撑满内容区：否则外层 div 只有内容高度，媒体元素的
// max-height:100% 失去参照，超大分辨率的图会把弹窗撑出界。
const mediaBody = computed(() => kind === 'image' || kind === 'video' || kind === 'audio')

// 只有「按文本逐行展示」的视图才谈得上换行：富文本 markdown、图片、PDF 不需要。
// 内容没读进来（超大文件、读失败）时也没有可换行的东西，开关一并隐藏。
const canWrap = computed(() => {
  if (bodyError.value) return false
  if (editing.value) return !(kind === 'markdown' && mdPreview.value)
  if (kind === 'html') return !htmlPreview.value && hasSource.value
  return textBody.value && hasSource.value
})

const hexText = computed(() => hexLines.value.join('\n'))
const hexTotal = computed(() => Math.max(0, props.item.size || 0))
const hexWindowEnd = computed(() => hexOffset.value + hexLoaded.value)
const hexHasPrev = computed(() => hexOffset.value > 0)
const hexHasNext = computed(() => hexWindowEnd.value < hexTotal.value)
// 读数一行说完：每个地址后面紧跟括号里的十进制偏移，两种口径一对就对得上。
function hexAddr(n: number): string {
  return '0x' + n.toString(16).padStart(8, '0')
}
function groupNum(n: number): string {
  return n.toLocaleString('en-US')
}
const hexEnd = computed(() => Math.max(hexOffset.value, hexWindowEnd.value - 1))
const hexRangeLine = computed(
  () =>
    `${hexAddr(hexOffset.value)} (${groupNum(hexOffset.value)}) – ` +
    `${hexAddr(hexEnd.value)} (${groupNum(hexEnd.value)})`,
)
// 悬停补一条换算后的大小，看「还剩多少」比看字节数快。
const hexRangeTitle = computed(
  () => `${formatSize(hexOffset.value)} / ${formatSize(hexTotal.value)}`,
)

const mdRendered = computed(() => renderMarkdown(editContent.value, { baseUrl: docBaseUrl }))

// 预览区 DOM：公式需要拿到真实节点才能排版（v-html 内容无法用模板控制）。
const bodyRef = ref<HTMLElement | null>(null)
const mdPreviewRef = ref<HTMLElement | null>(null)

// 网页内全屏：把弹窗铺满整个视口，不改动浏览器窗口本身（不进 Fullscreen API），
// 所以地址栏、标签页都在，切走后回来也不会被浏览器卡在全屏状态。
const fullscreen = ref(false)

function toggleFullscreen() {
  fullscreen.value = !fullscreen.value
}

// 内容变化后把公式/图表占位节点替换为真实渲染结果（KaTeX 与 Mermaid 均按需加载）。
// 两个容器互斥：只读预览用 bodyRef，编辑态预览用 mdPreviewRef。
watch(
  [bodyHtml, mdRendered, mdPreview, resolvedTheme],
  async () => {
    await nextTick()
    await renderMath(bodyRef.value ?? mdPreviewRef.value)
    await renderDiagrams(bodyRef.value ?? mdPreviewRef.value, resolvedTheme.value)
  },
  { flush: 'post' },
)

function download() {
  const a = document.createElement('a')
  a.href = url
  a.download = props.item.name
  document.body.appendChild(a)
  a.click()
  a.remove()
}

async function copyLink() {
  const full = location.origin + url
  const ok = await copyText(full)
  showToast(
    ok ? t('toast.copyPath') + ': ' + full : t('toast.copyFailed', { text: full }),
    ok ? 'success' : 'error',
  )
}

// 复制 curl 下载命令（预览只针对文件，目录不会走这里）。
async function copyCurl() {
  const full = location.origin + url
  const cmd = `curl -O "${full}"`
  const ok = await copyText(cmd)
  showToast(
    ok ? t('toast.copyCmd') + ': ' + cmd : t('toast.copyFailed', { text: cmd }),
    ok ? 'success' : 'error',
  )
}

// 复制 SHA256 摘要。
async function copyHash() {
  if (!hash.value) return
  const ok = await copyText(hash.value)
  showToast(
    ok ? t('toast.copyHash') : t('toast.copyFailed', { text: hash.value }),
    ok ? 'success' : 'error',
  )
}

// 在浏览器新窗口/新标签页中打开当前文件。
function openInNewTab() {
  window.open(location.origin + url, '_blank', 'noopener')
}

function startEdit() {
  if (!editable.value) return
  editing.value = true
  mdPreview.value = false
}

async function cancelEdit() {
  if (dirty.value) {
    const ok = await showConfirmDialog(t('dialog.discard'), t('dialog.discardEditMsg'))
    if (!ok) return
  }
  editing.value = false
  editContent.value = originalContent.value
}

async function save() {
  if (saving.value) return
  saving.value = true
  try {
    await saveFile(props.path, editContent.value)
    originalContent.value = editContent.value
    // 保存成功后重新渲染只读视图
    renderTextView(editContent.value)
    editing.value = false
    showToast(t('toast.saved'), 'success')
    // 刷新 sha256
    try { hash.value = await fileHash(props.path) } catch { /* ignore */ }
    emit('saved')
  } catch (e) {
    showToast(t('toast.saveFailed', { msg: e instanceof Error ? e.message : String(e) }), 'error')
  } finally {
    saving.value = false
  }
}

async function onClose() {
  if (dirty.value) {
    const ok = await showConfirmDialog(t('dialog.discard'), t('dialog.discardCloseMsg'))
    if (!ok) return
  }
  emit('close')
}

function escHtml(s: string): string {
  return s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c] as string))
}

// 直接把文件当网页跑。sandbox 刻意不给 allow-same-origin：被预览的 HTML 运行在
// 独立的不透明源里，脚本能跑（图表/自动排版可用），但拿不到 gofs 页面的
// DOM、cookie 与同源接口，避免有人上传一份 HTML 就借浏览者的身份改文件。
// 代价是依赖 fetch/localStorage 的页面需要走“新窗口打开”。
function htmlFrame(): string {
  return `<iframe src="${url}" class="preview-iframe html-frame" sandbox="allow-scripts allow-forms allow-popups allow-modals allow-downloads"></iframe>`
}

function toggleHtmlPreview() {
  htmlPreview.value = !htmlPreview.value
  // 源码视图需要内容；首次切换时 originalContent 已在挂载阶段读好。
  renderTextView(originalContent.value)
}

// 把文本内容渲染为只读预览（code/json/text 用 pre，markdown/html 按自身方式渲染）。
function renderTextView(text: string) {
  if (kind === 'markdown') {
    bodyHtml.value = `<div class="markdown-body">${renderMarkdown(text, { baseUrl: docBaseUrl })}</div>`
  } else if (kind === 'html' && htmlPreview.value) {
    bodyHtml.value = htmlFrame()
  } else {
    bodyHtml.value = `<pre class="code-block">${escHtml(text)}</pre>`
  }
}

// 读取一个 4 KiB 窗口并整屏替换。
// 偏移量按 16 字节对齐（hexdump 的行边界），超出文件末尾的请求会被服务端裁剪，
// 所以不用先把 offset 夹到 size 以内。
async function loadHex(start: number) {
  if (hexLoading.value) return
  const size = hexTotal.value
  const offset = Math.max(0, Math.floor(start / HEX_ALIGN) * HEX_ALIGN)
  hexOffset.value = Math.min(offset, size)
  // 输入框兼作当前位置的读数，翻页后也能看出自己在哪儿；和上方读数一样用十进制，
  // 不用十六进制就不用在脑子里换算。
  hexInput.value = String(hexOffset.value)
  if (size === 0) {
    hexLines.value = []
    hexLoaded.value = 0
    return
  }
  if (hexOffset.value >= size) {
    hexLines.value = []
    hexLoaded.value = 0
    return
  }
  hexLoading.value = true
  hexError.value = ''
  try {
    const end = Math.min(hexOffset.value + HEX_WINDOW, size) - 1
    const bytes = await readBytes(props.path, hexOffset.value, end)
    hexLines.value = hexDumpLines(bytes, hexOffset.value)
    hexLoaded.value = bytes.length
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    hexError.value = msg === 'RANGE_UNSUPPORTED' ? t('preview.hexNoRange') : msg
    hexLines.value = []
    hexLoaded.value = 0
  } finally {
    hexLoading.value = false
  }
}

// 解析用户输入的偏移量。支持三类写法，覆盖「我知道确切位置」和「我只想看个大概」
// 两类需求：
//   4194304      十进制字节偏移
//   0x400000     十六进制（和行首地址同一写法，可整段复制过来）
//   4K / 1M / 2G 二进制单位，K=KiB=1024、M=MiB、G=GiB，也接受 4KB、4KiB 写法
// 返回 null 表示无法解析。
const UNIT_SCALE: Record<string, number> = { k: 1024, m: 1024 ** 2, g: 1024 ** 3 }

function parseOffset(raw: string): number | null {
  const s = raw.trim().toLowerCase().replace(/[\s_,]/g, '')
  if (!s) return null
  // o 可选：兼容 4kB / 4KiB / 4Ko 这类写法
  const unit = s.match(/^(\d+(?:\.\d+)?)([kmg])(?:i?b|o)?$/)
  if (unit) {
    return Math.round(Number(unit[1]) * UNIT_SCALE[unit[2]])
  }
  const hex = s.match(/^0x([0-9a-f]+)$/)
  if (hex) return parseInt(hex[1], 16)
  const n = Number(s)
  return Number.isFinite(n) && n >= 0 ? n : null
}

// 跳到输入框里的偏移量；负数与超出末尾都夹回合法区间。
function jumpHex() {
  const n = parseOffset(hexInput.value)
  if (n === null) {
    showToast(t('preview.hexBadOffset'), 'error')
    return
  }
  void loadHex(Math.min(n, Math.max(0, hexTotal.value - 1)))
}

onMounted(async () => {
  try {
    if (kind === 'image') {
      bodyHtml.value = `<div class="preview-center"><img src="${url}" alt="${props.item.name}" /></div>`
    } else if (kind === 'video') {
      bodyHtml.value = `<div class="preview-center"><video src="${url}" controls autoplay style="max-width:100%;max-height:100%"></video></div>`
    } else if (kind === 'audio') {
      bodyHtml.value = `<div class="preview-center"><div class="audio-box">${ICONS.file}<audio src="${url}" controls></audio></div></div>`
    } else if (kind === 'pdf') {
      bodyHtml.value = `<iframe src="${url}" class="preview-iframe"></iframe>`
    } else if (kind === 'html') {
      // 先出页面：渲染不需要先读完源码，超大的 HTML 也能正常预览。
      bodyHtml.value = htmlFrame()
      try {
        const text = await readText(props.path)
        originalContent.value = text
        editContent.value = text
        hasSource.value = true
      } catch {
        // 只是拿不到“源码/编辑”能力，网页预览照旧。
      }
    } else if (editable.value) {
      const text = await readText(props.path)
      originalContent.value = text
      editContent.value = text
      hasSource.value = true
      renderTextView(text)
    } else if (hexMode.value) {
      // 压缩包与未知二进制：给 hexdump，而不是把「不支持预览」直接写在脸上。
      await loadHex(0)
    } else {
      bodyHtml.value = `<div class="preview-center"><div class="binary-box">${ICONS.file}<p>${t('preview.notSupported')}</p><p class="preview-meta">${formatSize(props.item.size)}</p></div></div>`
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    bodyError.value =
      msg === 'FILE_TOO_LARGE'
        ? t('preview.tooLarge', { limit: formatSize(MAX_TEXT_BYTES) })
        : msg
  } finally {
    loading.value = false
  }

  // sha256 hash footer
  try {
    hash.value = await fileHash(props.path)
  } catch { /* ignore */ }
})
</script>

<template>
  <div class="preview-overlay" :class="{ 'preview-overlay-full': fullscreen }" @click.self="onClose">
    <div class="preview-modal" :class="{ 'preview-modal-full': fullscreen }">
      <div class="preview-head">
        <span class="preview-title" :title="item.name">{{ item.name }}</span>
        <span v-if="dirty" class="preview-dirty">{{ t('preview.unsaved') }}</span>
        <div class="preview-actions">
          <template v-if="!editing">
            <!-- 同一组按钮统一为「图标 + 文字」，不混用纯图标 -->
            <button
              v-if="kind === 'html' && hasSource"
              class="btn"
              :title="htmlPreview ? t('preview.sourceTitle') : t('preview.renderedTitle')"
              @click="toggleHtmlPreview"
            >
              <span v-html="htmlPreview ? ICONS.code : ICONS.eye"></span
              >{{ htmlPreview ? t('preview.source') : t('preview.rendered') }}
            </button>
            <button v-if="editable && allowUpload && hasSource" class="btn" @click="startEdit">
              <span v-html="ICONS.rename"></span>{{ t('preview.edit') }}
            </button>
            <button class="btn" @click="download"><span v-html="ICONS.download"></span>{{ t('preview.download') }}</button>
            <button class="btn" :title="t('preview.copyLink')" @click="copyLink"><span v-html="ICONS.link"></span>{{ t('preview.copyLink') }}</button>
            <button class="btn" :title="t('action.copyCurl')" @click="copyCurl"><span v-html="ICONS.terminal"></span>{{ t('preview.curlDownload') }}</button>
            <button class="btn" :title="t('preview.openNewTitle')" @click="openInNewTab"><span v-html="ICONS.external"></span>{{ t('preview.openNew') }}</button>
          </template>
          <template v-else>
            <button v-if="kind === 'markdown'" class="btn" @click="mdPreview = !mdPreview">
              <span v-html="mdPreview ? ICONS.rename : ICONS.eye"></span
              >{{ mdPreview ? t('preview.continueEdit') : t('preview.preview') }}
            </button>
            <button class="btn btn-primary" :disabled="saving || !dirty" @click="save">
              <span v-html="ICONS.check"></span>{{ saving ? t('preview.saving') : t('preview.save') }}
            </button>
            <button class="btn" @click="cancelEdit"><span v-html="ICONS.x"></span>{{ t('common.cancel') }}</button>
          </template>
          <button
            class="btn"
            :title="fullscreen ? t('preview.exitFullscreenTitle') : t('preview.fullscreenTitle')"
            @click="toggleFullscreen"
          >
            <span v-html="fullscreen ? ICONS.compress : ICONS.expand"></span
            >{{ fullscreen ? t('preview.exitFullscreen') : t('preview.fullscreen') }}
          </button>
          <button class="icon-btn" :title="t('common.close')" @click="onClose" v-html="ICONS.close"></button>
        </div>
      </div>
      <div class="preview-body">
        <div v-if="loading" v-html="ICONS.spinner"></div>
        <div v-else-if="bodyError" class="preview-error">{{ bodyError }}</div>
        <template v-else-if="editing">
          <!-- markdown 编辑：左编辑右预览 / 切换预览 -->
          <div
            v-if="kind === 'markdown' && mdPreview"
            ref="mdPreviewRef"
            class="markdown-body editor-preview"
            v-html="mdRendered"
          ></div>
          <textarea
            v-else
            v-model="editContent"
            class="editor-textarea"
            :class="{ 'no-wrap': !wrapLines }"
            :wrap="wrapLines ? 'soft' : 'off'"
            spellcheck="false"
          ></textarea>
        </template>
        <template v-else-if="hexMode">
          <div class="hex-wrap">
            <div class="hex-bar">
              <span class="hex-range" :title="hexRangeTitle">{{ hexRangeLine }}</span>
              <button
                class="icon-btn"
                :title="t('preview.hexPrev')"
                :disabled="!hexHasPrev || hexLoading"
                @click="loadHex(hexOffset - HEX_WINDOW)"
                v-html="ICONS.chevronLeft16"
              ></button>
              <button
                class="icon-btn"
                :title="t('preview.hexNext')"
                :disabled="!hexHasNext || hexLoading"
                @click="loadHex(hexOffset + HEX_WINDOW)"
                v-html="ICONS.chevronRight16"
              ></button>
              <span class="hex-field">
                <input
                  v-model="hexInput"
                  class="hex-offset"
                  type="text"
                  spellcheck="false"
                  :placeholder="t('preview.hexOffsetPlaceholder')"
                  :title="t('preview.hexOffsetTitle')"
                  @focus="($event.target as HTMLInputElement).select()"
                  @keyup.enter="jumpHex"
                />
                <!-- 有内容才显示清除按钮，空的时候不用占一个“点了没事干”的图标 -->
                <button
                  v-if="hexInput"
                  class="input-clear"
                  type="button"
                  :title="t('common.clear')"
                  :aria-label="t('common.clear')"
                  @click="hexInput = ''"
                  v-html="ICONS.x"
                ></button>
              </span>
              <button class="btn" :disabled="hexLoading || !hexInput.trim()" @click="jumpHex">
                <span v-html="ICONS.arrowRight"></span>{{ t('preview.hexGo') }}
              </button>
            </div>
            <div v-if="hexError" class="preview-error">{{ hexError }}</div>
            <div v-else-if="!hexTotal" class="preview-error">{{ t('preview.hexEmpty') }}</div>
            <!-- 用插值而不是 v-html：ASCII 列可能包含 < > & 这些字符 -->
            <pre v-else class="hex-dump">{{ hexText }}</pre>
          </div>
        </template>
        <div
          v-else
          ref="bodyRef"
          :class="{
            'no-wrap': !wrapLines,
            'preview-code-wrap': textBody,
            'preview-fill': mediaBody,
          }"
          v-html="bodyHtml"
        ></div>
        <!-- hex 模式下下载按钮已在标题栏，这里再放一个会把 dump 挤出可视区 -->
        <div
          v-if="!loading && !editing && (bodyError || (!inlinePreview && !hexMode))"
          style="text-align:center;margin-top:12px"
        >
          <button class="btn btn-primary" @click="download">
            <span v-html="ICONS.download"></span>{{ t('preview.downloadFile') }}
          </button>
        </div>
      </div>
      <!-- 底栏常驻：大小 / 修改时间 / SHA256；右侧是换行开关。标题栏只留文件名，因此永远是单行。 -->
      <div class="preview-foot">
        <span class="preview-meta">{{ formatSize(item.size) }} · {{ formatTime(item.mtime) }}</span>
        <span v-if="hash" class="preview-meta preview-hash" :title="hash">SHA256: {{ hash }}</span>
        <button
          v-if="hash"
          class="icon-btn hash-copy"
          :title="t('preview.copyHash')"
          @click="copyHash"
          v-html="ICONS.copy"
        ></button>
        <!-- 换行开关：两种状态都叫两个字（换行 / 滚动），配固定宽度，
             点击时按钮尺寸和相邻元素位置都不动。 -->
        <button
          v-if="canWrap"
          class="btn wrap-toggle"
          :class="{ active: wrapLines }"
          :aria-pressed="wrapLines"
          :title="wrapLines ? t('preview.wrapOnTitle') : t('preview.wrapOffTitle')"
          @click="wrapLines = !wrapLines"
        >
          <span v-html="wrapLines ? ICONS.wrap : ICONS.noWrap"></span
          >{{ wrapLines ? t('preview.wrap') : t('preview.scroll') }}
        </button>
      </div>
    </div>
  </div>
</template>
