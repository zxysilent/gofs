<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import type { PathItem } from '../types'
import { parentRelPath } from '../utils/paths'
import { fileKind, formatSize, formatTime } from '../utils'
import { absUrl, readText, readBytes, fileHash, saveFile } from '../api'
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

// 不能直接预览的二进制改用 hexdump：按 Range 分块拉取，看几行魔数不需要下载整个文件。
const hexMode = computed(() => !inlinePreview.value)
const HEX_CHUNK = 16 * 1024
const HEX_INITIAL = 32 * 1024
// 上限：再大就只是在刷 DOM，真要看内容应该下载下来。
const HEX_LIMIT = 256 * 1024
const hexLines = ref<string[]>([])
const hexLoaded = ref(0)
const hexLoading = ref(false)
const hexHasMore = ref(false)
const hexError = ref('')

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

const dirty = computed(() => editing.value && editContent.value !== originalContent.value)

const hexText = computed(() => hexLines.value.join('\n'))
const hexTotal = computed(() => Math.max(0, props.item.size || 0))
const hexCanLoadMore = computed(
  () => hexHasMore.value && hexLoaded.value < HEX_LIMIT && hexLoaded.value < hexTotal.value,
)
// 首行提示：已渲染多少 / 文件多大，避免误以为看的就是全部内容。
const hexNotice = computed(() =>
  t('preview.hexShown', {
    shown: formatSize(hexLoaded.value),
    total: formatSize(hexTotal.value),
  }),
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

// 读取二进制片段并追加到 hexdump。
// append=true 时从已加载位置续读一个块，否则从 0 开始读首屏大小。
async function loadHex(append: boolean) {
  if (hexLoading.value) return
  const size = hexTotal.value
  if (size === 0) {
    hexLines.value = []
    hexLoaded.value = 0
    hexHasMore.value = false
    return
  }
  const start = append ? hexLoaded.value : 0
  if (start >= size) {
    hexHasMore.value = false
    return
  }
  hexLoading.value = true
  hexError.value = ''
  try {
    const want = append ? HEX_CHUNK : HEX_INITIAL
    const end = Math.min(start + want, size) - 1
    const bytes = await readBytes(props.path, start, end)
    const lines = hexDumpLines(bytes, start)
    hexLines.value = append ? hexLines.value.concat(lines) : lines
    hexLoaded.value = start + bytes.length
    hexHasMore.value = hexLoaded.value < size
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    hexError.value = msg === 'RANGE_UNSUPPORTED' ? t('preview.hexNoRange') : msg
    hexHasMore.value = false
  } finally {
    hexLoading.value = false
  }
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
      await loadHex(false)
    } else {
      bodyHtml.value = `<div class="preview-center"><div class="binary-box">${ICONS.file}<p>${t('preview.notSupported')}</p><p class="preview-meta">${formatSize(props.item.size)}</p></div></div>`
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    bodyError.value = msg === 'FILE_TOO_LARGE' ? t('preview.tooLarge') : msg
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
            spellcheck="false"
          ></textarea>
        </template>
        <template v-else-if="hexMode">
          <div class="hex-wrap">
            <div class="hex-bar">
              <span class="preview-meta">{{ hexNotice }}</span>
              <button
                v-if="hexCanLoadMore"
                class="btn"
                :disabled="hexLoading"
                @click="loadHex(true)"
              >
                <span v-html="ICONS.more"></span>{{ hexLoading ? t('common.loading') : t('preview.hexMore') }}
              </button>
            </div>
            <div v-if="hexError" class="preview-error">{{ hexError }}</div>
            <div v-else-if="!hexTotal" class="preview-error">{{ t('preview.hexEmpty') }}</div>
            <!-- 用插值而不是 v-html：ASCII 列可能包含 < > & 这些字符 -->
            <pre v-else class="hex-dump">{{ hexText }}</pre>
          </div>
        </template>
        <div v-else ref="bodyRef" v-html="bodyHtml"></div>
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
      <!-- 底栏常驻：大小 / 修改时间 / SHA256。标题栏只留文件名，因此永远是单行。 -->
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
      </div>
    </div>
  </div>
</template>
