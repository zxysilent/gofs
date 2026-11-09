<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import type { IndexData, ListParams, PathItem, SortKey, SortOrder, UploadTask } from './types'
import { baseName, joinRelPath, normalizeRelPath, parentRelPath, isDir, HELP_ROUTE, isHelpRoute } from './types'
import {
  initialData, listDir, absUrl, hashUrl, parseHash, uploadFile,
  deletePath, mkdir, createFile, movePath,
} from './api'
import { showInputDialog, showConfirmDialog, showToast, copyText } from './dialog'
import Sidebar from './components/Sidebar.vue'
import Toolbar from './components/Toolbar.vue'
import FileList from './components/FileList.vue'
import PreviewModal from './components/PreviewModal.vue'
import UploadManager from './components/UploadManager.vue'
import HelpPage from './components/HelpPage.vue'

const data = ref<IndexData>(initialData)
const relPath = ref<string>(normalizeRelPath(initialData.href))
const loading = ref(false)
const error = ref('')

const sort = ref<SortKey>('name')
const order = ref<SortOrder>('asc')
const search = ref('')
const viewMode = ref<'list' | 'grid'>('list')
const showSidebar = ref(true)

const preview = ref<{ item: PathItem; path: string } | null>(null)
const uploads = ref<UploadTask[]>([])

const allowUpload = computed(() => data.value.allow_upload)
const allowDelete = computed(() => data.value.allow_delete)
const allowSearch = computed(() => data.value.allow_search)
const allowArchive = computed(() => data.value.allow_archive)

const items = computed(() => data.value.paths || [])

const breadcrumbs = computed(() => {
  const p = relPath.value
  if (p === '/') return [{ name: '根目录', path: '/' }]
  const crumbs = [{ name: '根目录', path: '/' }]
  let acc = ''
  for (const part of p.split('/').filter(Boolean)) {
    acc = joinRelPath(acc || '/', part)
    crumbs.push({ name: part, path: acc })
  }
  return crumbs
})

const sidebarRef = ref<InstanceType<typeof Sidebar> | null>(null)

// 目录树节点的操作按钮（hover 显示）
async function onTreeAction(payload: { action: string; path: string; name: string }) {
  const { action, path, name } = payload
  if (action === 'copy-path') {
    await copyAndToast(location.origin + absUrl(path), '已复制完整路径')
    return
  }
  if (action === 'rename') {
    const newName = await showInputDialog('重命名', '请输入新名称', name)
    if (!newName || newName === name) return
    await run(async () => {
      await movePath(path, joinRelPath(parentRelPath(path), newName))
      return `已重命名为 "${newName}"`
    }, { rebuild: true })
    return
  }
  if (action === 'delete') {
    const ok = await showConfirmDialog('删除确认', `确定删除目录 "${name}" 及其全部内容吗？此操作不可恢复。`)
    if (!ok) return
    await run(async () => {
      await deletePath(path)
      return `已删除 "${name}"`
    }, { rebuild: true })
  }
}

async function refresh() {
  await navigate(relPath.value)
}

// Guard against the hashchange handler re-entering navigate for the same
// navigation we just triggered programmatically.
let suppressHashEvent = false

async function navigate(path: string) {
  path = normalizeRelPath(path)

  // 特殊路由：接口手册页（展示 /gofs-agent.md），不请求服务端目录数据。
  if (isHelpRoute(path)) {
    loading.value = false
    error.value = ''
    relPath.value = HELP_ROUTE
    document.title = 'gofs 接口手册'
    const target = '#' + HELP_ROUTE
    if (location.hash !== target) {
      suppressHashEvent = true
      location.hash = target
    }
    return
  }

  loading.value = true
  error.value = ''
  try {
    const params: ListParams = { sort: sort.value, order: order.value, q: search.value }
    const d = await listDir(path, params)
    d.paths = d.paths || []
    data.value = d
    relPath.value = normalizeRelPath(d.href || path)
    document.title = `Index of ${relPath.value} - gofs`
    // update the URL hash (client-side route) without a full page load
    const target = '#' + relPath.value
    if (location.hash !== target) {
      suppressHashEvent = true
      location.hash = target
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
    sidebarRef.value?.onNavigated(relPath.value)
  }
}

function onSearch(q: string) {
  search.value = q
  navigate(relPath.value)
}

function onSortChange(k: SortKey, o: SortOrder) {
  sort.value = k
  order.value = o
  navigate(relPath.value)
}

function onSelect(item: PathItem) {
  if (isDir(item)) {
    navigate(joinRelPath(relPath.value, item.name))
  } else {
    preview.value = { item, path: joinRelPath(relPath.value, item.name) }
  }
}

// ---- write operations ----
async function onUploadFiles(files: FileList | File[]) {
  const list = Array.from(files)
  for (const file of list) {
    const task: UploadTask = reactive({
      id: Math.random().toString(36).slice(2),
      name: file.name,
      progress: 0,
      status: 'uploading',
    })
    uploads.value.push(task)
    const target = joinRelPath(relPath.value, file.name)
    try {
      await uploadFile(target, file, (p) => {
        task.progress = p.total ? Math.round((p.loaded / p.total) * 100) : 0
      })
      task.status = 'done'
      task.progress = 100
    } catch (e) {
      task.status = 'error'
      task.error = e instanceof Error ? e.message : String(e)
    } finally {
      setTimeout(() => {
        const idx = uploads.value.indexOf(task)
        if (idx >= 0 && task.status !== 'uploading') uploads.value.splice(idx, 1)
      }, 2500)
    }
    await refresh()
  }
}

// 执行一个写操作：出错统一 toast，成功后可选刷新目录树与文件列表。
async function run(fn: () => Promise<string | null>, opts?: { rebuild?: boolean }) {
  try {
    const msg = await fn()
    if (msg === null) return // 用户取消或无需操作
    if (opts?.rebuild) await sidebarRef.value?.rebuildTree()
    await refresh()
    showToast(msg, 'success')
  } catch (e) {
    showToast('操作失败: ' + (e instanceof Error ? e.message : e), 'error')
  }
}

async function onNewFolder() {
  const name = await showInputDialog('新建文件夹', '请输入文件夹名称', '')
  if (!name) return
  await run(async () => {
    await mkdir(joinRelPath(relPath.value, name))
    return `已创建文件夹 "${name}"`
  })
}

async function onNewFile() {
  const name = await showInputDialog('新建文件', '请输入文件名', '')
  if (!name) return
  await run(async () => {
    await createFile(joinRelPath(relPath.value, name))
    return `已创建文件 "${name}"`
  })
}

async function onDeleteItem(item: PathItem) {
  const ok = await showConfirmDialog('删除确认', `确定删除 "${item.name}" 吗？此操作不可恢复。`)
  if (!ok) return
  await run(async () => {
    await deletePath(joinRelPath(relPath.value, item.name))
    return `已删除 "${item.name}"`
  })
}

async function onRenameItem(item: PathItem) {
  const newName = await showInputDialog('重命名', '请输入新名称', item.name)
  if (!newName || newName === item.name) return
  await run(async () => {
    await movePath(joinRelPath(relPath.value, item.name), joinRelPath(relPath.value, newName))
    return `已重命名为 "${newName}"`
  })
}

// 复制文本并 toast 提示。
async function copyAndToast(text: string, label: string) {
  const ok = await copyText(text)
  showToast(ok ? `${label}: ${text}` : `复制失败，请手动复制: ${text}`, ok ? 'success' : 'error')
}

// 复制完整路径（绝对 URL）。
async function onCopyItem(item: PathItem) {
  await copyAndToast(location.origin + absUrl(joinRelPath(relPath.value, item.name)), '已复制完整路径')
}

// 复制 curl 下载命令。
async function onCopyCurl(item: PathItem) {
  const fullUrl = location.origin + absUrl(joinRelPath(relPath.value, item.name))
  const cmd = isDir(item) ? `curl -o "${item.name}.zip" "${fullUrl}?zip"` : `curl -O "${fullUrl}"`
  await copyAndToast(cmd, '已复制命令')
}

// 在浏览器新窗口/新标签页中打开（文件直接预览，目录进入该目录）。
function onOpenInNewTab(item: PathItem) {
  window.open(location.origin + absUrl(joinRelPath(relPath.value, item.name)), '_blank', 'noopener')
}

// 复制 curl 上传到当前目录的命令。
async function onCopyCurlUpload() {
  const base = location.origin + absUrl(relPath.value)
  const dirUrl = base.endsWith('/') ? base : base + '/'
  await copyAndToast(`curl -T "<文件名>" "${dirUrl}"`, '已复制命令')
}

// 触发浏览器下载某个路径的 zip 打包。
function downloadZip(path: string, filename: string) {
  const a = document.createElement('a')
  a.href = absUrl(path, 'zip')
  a.download = filename + '.zip'
  a.click()
  a.remove()
}

function onDownloadArchive() {
  downloadZip(relPath.value, baseName(relPath.value) || 'archive')
}

// 目录行内的"打包下载"。
function onDownloadItemArchive(item: PathItem) {
  downloadZip(joinRelPath(relPath.value, item.name), item.name)
}

// ---- hash routing ----
function onHashChange() {
  // ignore the hashchange we triggered ourselves inside navigate()
  if (suppressHashEvent) {
    suppressHashEvent = false
    return
  }
  const p = parseHash()
  if (normalizeRelPath(p) !== relPath.value) {
    navigate(p)
  }
}

onMounted(() => {
  document.title = `Index of ${relPath.value} - gofs`
  window.addEventListener('hashchange', onHashChange)
  // If the page was opened with a hash (deep link / refresh), honour it.
  const hp = parseHash()
  if (normalizeRelPath(hp) !== relPath.value) {
    navigate(hp)
  } else if (location.hash !== '#' + relPath.value) {
    // normalize the URL to the canonical hash route without adding history
    history.replaceState(null, '', hashUrl(relPath.value))
  }
})

// ---- 可拖动分栏 ----
const SIDEBAR_MIN = 180
const SIDEBAR_MAX = 560
const sidebarWidth = ref(240)
let dragging = false

function startDrag(e: MouseEvent) {
  dragging = true
  e.preventDefault()
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
}

function onDrag(e: MouseEvent) {
  if (!dragging) return
  // sidebar 从视口左侧开始，宽度即鼠标 x 坐标
  const w = e.clientX
  sidebarWidth.value = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, w))
}

function stopDrag() {
  dragging = false
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

onBeforeUnmount(() => {
  window.removeEventListener('hashchange', onHashChange)
  stopDrag()
})
</script>

<template>
  <div class="app">
    <aside
      v-show="showSidebar"
      class="sidebar"
      :style="{ width: sidebarWidth + 'px', minWidth: sidebarWidth + 'px' }"
    >
      <Sidebar ref="sidebarRef" :current-path="relPath" @navigate="navigate" @action="onTreeAction" />
    </aside>
    <div
      v-show="showSidebar"
      class="splitter"
      title="拖动调整侧栏宽度"
      @mousedown="startDrag"
    ></div>

    <main class="main">
      <!-- 特殊路由：curl 使用手册页 -->
      <HelpPage v-if="relPath === HELP_ROUTE" @back="navigate('/')" />

      <template v-else>
        <Toolbar
          :breadcrumbs="breadcrumbs"
          :allow-upload="allowUpload"
          :allow-archive="allowArchive"
          :allow-search="allowSearch"
          :view-mode="viewMode"
          :search="search"
          :sort="sort"
          :order="order"
          @toggle-sidebar="showSidebar = !showSidebar"
          @search="onSearch"
          @sort-change="onSortChange"
          @view-change="viewMode = $event"
          @upload="onUploadFiles"
          @new-folder="onNewFolder"
          @new-file="onNewFile"
          @curl-upload="onCopyCurlUpload"
          @download-archive="onDownloadArchive"
          @refresh="refresh"
          @navigate="navigate"
        />

        <div v-if="error" class="error-banner">{{ error }}</div>

        <FileList
          :items="items"
          :loading="loading"
          :view-mode="viewMode"
          :allow-upload="allowUpload"
          :allow-delete="allowDelete"
          :allow-archive="allowArchive"
          @select="onSelect"
          @delete="onDeleteItem"
          @rename="onRenameItem"
          @copy="onCopyItem"
          @copy-curl="onCopyCurl"
          @open-new="onOpenInNewTab"
          @download-archive="onDownloadItemArchive"
        />
      </template>
    </main>

    <UploadManager :tasks="uploads" />

    <PreviewModal
      v-if="preview"
      :item="preview.item"
      :path="preview.path"
      :allow-upload="allowUpload"
      @close="preview = null"
      @saved="refresh"
    />
  </div>
</template>
