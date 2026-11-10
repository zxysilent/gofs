<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import type { PathItem } from '../types'
import { parentRelPath } from '../types'
import { fileKind, formatSize, formatTime } from '../utils'
import { absUrl, readText, fileHash, saveFile } from '../api'
import { renderMarkdown } from '../markdown'
import { renderMath } from '../math'
import { renderDiagrams } from '../diagram'
import { ICONS } from '../icons'
import { showToast, showConfirmDialog, copyText } from '../dialog'
import { t } from '../i18n'
import { resolvedTheme } from '../theme'

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
  kind === 'text' || kind === 'code' || kind === 'json' || kind === 'markdown',
)

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

const dirty = computed(() => editing.value && editContent.value !== originalContent.value)

const mdRendered = computed(() => renderMarkdown(editContent.value, { baseUrl: docBaseUrl }))

// 预览区 DOM：公式需要拿到真实节点才能排版（v-html 内容无法用模板控制）。
const bodyRef = ref<HTMLElement | null>(null)
const mdPreviewRef = ref<HTMLElement | null>(null)

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

// Ctrl/Cmd+S 保存
function onEditorKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    save()
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

// 把文本内容渲染为只读预览（code/json/text 用 pre，markdown 渲染）。
function renderTextView(text: string) {
  if (kind === 'markdown') {
    bodyHtml.value = `<div class="markdown-body">${renderMarkdown(text, { baseUrl: docBaseUrl })}</div>`
  } else {
    bodyHtml.value = `<pre class="code-block">${escHtml(text)}</pre>`
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
    } else if (editable.value) {
      const text = await readText(props.path)
      originalContent.value = text
      editContent.value = text
      renderTextView(text)
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
  <div class="preview-overlay" @click.self="onClose">
    <div class="preview-modal" :class="{ 'preview-modal-editing': editing }">
      <div class="preview-head">
        <span class="preview-title" :title="item.name">{{ item.name }}</span>
        <span class="preview-meta">{{ formatSize(item.size) }} · {{ formatTime(item.mtime) }}</span>
        <span v-if="dirty" class="preview-dirty">{{ t('preview.unsaved') }}</span>
        <div class="preview-actions">
          <template v-if="!editing">
            <button v-if="editable && allowUpload" class="btn" @click="startEdit">
              <span v-html="ICONS.rename"></span>{{ t('preview.edit') }}
            </button>
            <button class="btn" @click="download"><span v-html="ICONS.download"></span>{{ t('preview.download') }}</button>
            <button class="btn" @click="copyLink"><span v-html="ICONS.link"></span>{{ t('preview.copyLink') }}</button>
            <button class="btn" :title="t('action.copyCurl')" @click="copyCurl"><span v-html="ICONS.terminal"></span>{{ t('preview.curlDownload') }}</button>
            <button class="btn" :title="t('preview.openNewTitle')" @click="openInNewTab"><span v-html="ICONS.external"></span>{{ t('preview.openNew') }}</button>
          </template>
          <template v-else>
            <button v-if="kind === 'markdown'" class="btn" @click="mdPreview = !mdPreview">
              {{ mdPreview ? t('preview.continueEdit') : t('preview.preview') }}
            </button>
            <button class="btn btn-primary" :disabled="saving || !dirty" @click="save">
              {{ saving ? t('preview.saving') : t('preview.save') }}
            </button>
            <button class="btn" @click="cancelEdit">{{ t('common.cancel') }}</button>
          </template>
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
            @keydown="onEditorKeydown"
          ></textarea>
        </template>
        <div v-else ref="bodyRef" v-html="bodyHtml"></div>
        <div v-if="!loading && !editing && kind === 'binary'" style="text-align:center;margin-top:12px">
          <button class="btn btn-primary" @click="download">{{ t('preview.downloadFile') }}</button>
        </div>
      </div>
      <div v-if="hash" class="preview-foot">
        <span class="preview-meta preview-hash" :title="hash">SHA256: {{ hash }}</span>
        <button class="icon-btn hash-copy" :title="t('preview.copyHash')" @click="copyHash" v-html="ICONS.copy"></button>
      </div>
    </div>
  </div>
</template>
