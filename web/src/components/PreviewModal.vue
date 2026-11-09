<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import type { PathItem } from '../types'
import { fileKind, formatSize, formatTime } from '../utils'
import { absUrl, readText, fileHash, saveFile } from '../api'
import { renderMarkdown } from '../markdown'
import { ICONS } from '../icons'
import { showToast, showConfirmDialog, copyText } from '../dialog'

const props = defineProps<{ item: PathItem; path: string; allowUpload: boolean }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const kind = fileKind(props.item.name)
const url = absUrl(props.path)

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

const mdRendered = computed(() => renderMarkdown(editContent.value))

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
  if (ok) showToast('已复制完整路径: ' + full, 'success')
  else showToast('复制失败，请手动复制: ' + full, 'error')
}

// 复制 curl 下载命令（预览只针对文件，目录不会走这里）。
async function copyCurl() {
  const full = location.origin + url
  const cmd = `curl -O "${full}"`
  const ok = await copyText(cmd)
  if (ok) showToast('已复制命令: ' + cmd, 'success')
  else showToast('复制失败: ' + cmd, 'error')
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
    const ok = await showConfirmDialog('放弃修改', '内容已修改，确定放弃未保存的修改吗？')
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
    showToast('已保存', 'success')
    // 刷新 sha256
    try { hash.value = await fileHash(props.path) } catch { /* ignore */ }
    emit('saved')
  } catch (e) {
    showToast('保存失败: ' + (e instanceof Error ? e.message : e), 'error')
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
    const ok = await showConfirmDialog('放弃修改', '内容已修改，确定关闭并放弃未保存的修改吗？')
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
    bodyHtml.value = `<div class="markdown-body">${renderMarkdown(text)}</div>`
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
      bodyHtml.value = `<div class="preview-center"><div class="binary-box">${ICONS.file}<p>此文件类型暂不支持在线预览</p><p class="preview-meta">${formatSize(props.item.size)}</p></div></div>`
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    bodyError.value = msg === 'FILE_TOO_LARGE' ? '文件过大，无法在线预览，请下载查看。' : msg
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
        <span v-if="dirty" class="preview-dirty">●未保存</span>
        <div class="preview-actions">
          <template v-if="!editing">
            <button v-if="editable && allowUpload" class="btn" @click="startEdit">
              <span v-html="ICONS.rename"></span>编辑
            </button>
            <button class="btn" @click="download"><span v-html="ICONS.download"></span>下载</button>
            <button class="btn" @click="copyLink"><span v-html="ICONS.link"></span>复制链接</button>
            <button class="btn" title="复制 curl 下载命令" @click="copyCurl"><span v-html="ICONS.terminal"></span>curl 下载</button>
            <button class="btn" title="在浏览器新窗口打开" @click="openInNewTab"><span v-html="ICONS.external"></span>新窗口打开</button>
          </template>
          <template v-else>
            <button v-if="kind === 'markdown'" class="btn" @click="mdPreview = !mdPreview">
              {{ mdPreview ? '继续编辑' : '预览' }}
            </button>
            <button class="btn btn-primary" :disabled="saving || !dirty" @click="save">
              {{ saving ? '保存中...' : '保存 (Ctrl+S)' }}
            </button>
            <button class="btn" @click="cancelEdit">取消</button>
          </template>
          <button class="icon-btn" title="关闭" @click="onClose" v-html="ICONS.close"></button>
        </div>
      </div>
      <div class="preview-body">
        <div v-if="loading" v-html="ICONS.spinner"></div>
        <div v-else-if="bodyError" class="preview-error">{{ bodyError }}</div>
        <template v-else-if="editing">
          <!-- markdown 编辑：左编辑右预览 / 切换预览 -->
          <div v-if="kind === 'markdown' && mdPreview" class="markdown-body editor-preview" v-html="mdRendered"></div>
          <textarea
            v-else
            v-model="editContent"
            class="editor-textarea"
            spellcheck="false"
            @keydown="onEditorKeydown"
          ></textarea>
        </template>
        <div v-else v-html="bodyHtml"></div>
        <div v-if="!loading && !editing && kind === 'binary'" style="text-align:center;margin-top:12px">
          <button class="btn btn-primary" @click="download">下载文件</button>
        </div>
      </div>
      <div v-if="hash" class="preview-foot">
        <span class="preview-meta">SHA256: {{ hash }}</span>
      </div>
    </div>
  </div>
</template>
