<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import type { IndexData, ListParams, PathItem, SortKey, SortOrder, UploadTask } from './types'
import { baseName, joinRelPath, normalizeRelPath, parentRelPath, isDir } from './utils/paths'
import { initialData, listDir, absUrl, uploadFile, deletePath, mkdir, createFile, movePath } from './api'
import { HELP_ROUTE, isHelpRoute, dirUrl, parseHash, pathFromLocation } from './api/router'
import { showInputDialog, showConfirmDialog, showToast, copyText } from './utils/dialog'
import { t, locale } from './locales'
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
  // 读取 locale 让面包屑在切换语言时重新计算
  void locale.value
  if (p === '/') return [{ name: t('app.root'), path: '/' }]
  const crumbs = [{ name: t('app.root'), path: '/' }]
  let acc = ''
  for (const part of p.split('/').filter(Boolean)) {
    acc = joinRelPath(acc || '/', part)
    crumbs.push({ name: part, path: acc })
  }
  return crumbs
})

const sidebarRef = ref<InstanceType<typeof Sidebar> | null>(null)

// 目录树节点的右键菜单动作（复制链接/路径已在前端直接处理，不经过这里）。
async function onTreeAction(payload: { action: string; path: string; name: string }) {
  const { action, path, name } = payload
  if (action === 'rename') {
    const newName = await showInputDialog(t('dialog.rename'), t('dialog.renamePlaceholder'), name)
    if (!newName || newName === name) return
    await run(async () => {
      await movePath(path, joinRelPath(parentRelPath(path), newName))
      return t('toast.renamed', { name: newName })
    }, { rebuild: true })
    return
  }
  if (action === 'delete') {
    const ok = await showConfirmDialog(
      t('dialog.deleteConfirm'),
      t('dialog.deleteDirMsg', { name }),
    )
    if (!ok) return
    await run(async () => {
      await deletePath(path)
      return t('toast.deleted', { name })
    }, { rebuild: true })
    return
  }
  // 在指定目录内新建子目录（右键菜单）。
  if (action === 'new-folder') {
    const newName = await showInputDialog(t('dialog.newFolder'), t('dialog.newFolderPlaceholder'), '')
    if (!newName) return
    await run(async () => {
      await mkdir(joinRelPath(path, newName))
      return t('toast.folderCreated', { name: newName })
    }, { rebuild: true })
  }
}

async function refresh() {
  await navigate(relPath.value, { replace: true })
}

async function navigate(path: string, opts?: { replace?: boolean }) {
  path = normalizeRelPath(path)

  // 特殊路由：接口手册页（展示 /gofs-agent.md），不请求服务端目录数据。
  // 该路由服务端不存在，只能放在 hash 里，刷新时才不会 404。
  if (isHelpRoute(path)) {
    loading.value = false
    error.value = ''
    relPath.value = HELP_ROUTE
    document.title = t('app.agentDocTitle')
    const target = '#' + HELP_ROUTE
    if (location.hash !== target) {
      const url = location.pathname + location.search + target
      if (opts?.replace) history.replaceState(null, '', url)
      else history.pushState(null, '', url)
    }
    return
  }

  loading.value = true
  error.value = ''
  let listedPaths: PathItem[] | undefined
  try {
    const params: ListParams = { sort: sort.value, order: order.value, q: search.value }
    const d = await listDir(path, params)
    d.paths = d.paths || []
    data.value = d
    relPath.value = normalizeRelPath(d.href || path)
    listedPaths = d.paths
    document.title = `Index of ${relPath.value} - gofs`
    // 目录用真实路径写入地址栏（不再往 hash 里塞一遍路径）。
    // 与当前地址一致时不产生多余的历史记录；refresh 走 replace。
    const target = dirUrl(relPath.value)
    const current = location.pathname + location.search
    if (current !== target) {
      if (opts?.replace) history.replaceState(null, '', target)
      else history.pushState(null, '', target)
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
    // 把本次列表数据一并交给目录树，避免为刷新树再发一次请求；
    // 列表不可用（如搜索）时传 undefined，保留树中已有结构。
    sidebarRef.value?.onNavigated(relPath.value, search.value ? undefined : listedPaths)
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
async function onUploadFiles(files: FileList | File[] | null) {
  if (!files) return
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

// 执行一个写操作：出错统一 toast；成功后刷新列表，必要时重建目录树。
async function run(fn: () => Promise<string | null>, opts?: { rebuild?: boolean }) {
  try {
    const msg = await fn()
    if (msg === null) return // 用户取消或无需操作
    await refresh()
    if (opts?.rebuild) await sidebarRef.value?.rebuildTree()
    showToast(msg, 'success')
  } catch (e) {
    showToast(t('app.operationFailed', { msg: e instanceof Error ? e.message : String(e) }), 'error')
  }
}

async function onNewFolder() {
  const name = await showInputDialog(t('dialog.newFolder'), t('dialog.newFolderPlaceholder'), '')
  if (!name) return
  await run(async () => {
    await mkdir(joinRelPath(relPath.value, name))
    return t('toast.folderCreated', { name })
  }, { rebuild: true })
}

async function onNewFile() {
  const name = await showInputDialog(t('dialog.newFile'), t('dialog.newFilePlaceholder'), '')
  if (!name) return
  await run(async () => {
    await createFile(joinRelPath(relPath.value, name))
    return t('toast.fileCreated', { name })
  })
}

async function onDeleteItem(item: PathItem) {
  const ok = await showConfirmDialog(
    t('dialog.deleteConfirm'),
    t('dialog.deleteItemMsg', { name: item.name }),
  )
  if (!ok) return
  await run(async () => {
    await deletePath(joinRelPath(relPath.value, item.name))
    return t('toast.deleted', { name: item.name })
  }, { rebuild: isDir(item) })
}

async function onRenameItem(item: PathItem) {
  const newName = await showInputDialog(t('dialog.rename'), t('dialog.renamePlaceholder'), item.name)
  if (!newName || newName === item.name) return
  await run(async () => {
    await movePath(joinRelPath(relPath.value, item.name), joinRelPath(relPath.value, newName))
    return t('toast.renamed', { name: newName })
  }, { rebuild: isDir(item) })
}

// 复制文本并 toast 提示。
async function copyAndToast(text: string, label: string) {
  const ok = await copyText(text)
  showToast(
    ok ? `${label}: ${text}` : t('toast.copyFailed', { text }),
    ok ? 'success' : 'error',
  )
}

// 复制完整路径（绝对 URL）。
async function onCopyItem(item: PathItem) {
  await copyAndToast(
    location.origin + absUrl(joinRelPath(relPath.value, item.name)),
    t('toast.copyPath'),
  )
}

// 只复制文件名（不含路径）。
async function onCopyName(item: PathItem) {
  await copyAndToast(item.name, t('toast.copyName'))
}

// 复制 curl 下载命令。
async function onCopyCurl(item: PathItem) {
  const fullUrl = location.origin + absUrl(joinRelPath(relPath.value, item.name))
  const cmd = isDir(item) ? `curl -o "${item.name}.zip" "${fullUrl}?zip"` : `curl -O "${fullUrl}"`
  await copyAndToast(cmd, t('toast.copyCmd'))
}

// 在浏览器新窗口/新标签页中打开（文件直接预览，目录进入该目录）。
function onOpenInNewTab(item: PathItem) {
  window.open(location.origin + absUrl(joinRelPath(relPath.value, item.name)), '_blank', 'noopener')
}

// 复制 curl 上传到当前目录的命令。
async function onCopyCurlUpload() {
  const base = location.origin + absUrl(relPath.value)
  const dirUrl = base.endsWith('/') ? base : base + '/'
  await copyAndToast(`curl -T "<file>" "${dirUrl}"`, t('toast.copyCmd'))
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

// 文件行内的"下载"：直接指向文件地址，由浏览器自己另存。
function onDownloadItem(item: PathItem) {
  if (isDir(item)) return onDownloadItemArchive(item)
  const a = document.createElement('a')
  a.href = absUrl(joinRelPath(relPath.value, item.name))
  a.download = item.name
  document.body.appendChild(a)
  a.click()
  a.remove()
}

// ---- 路由 ----
// 目录用真实路径（History API），因此前进/后退走 popstate；
// 帮助页仍在使用 hash，所以 hashchange 也要监听。
function syncFromLocation() {
  if (location.hash && location.hash !== '#' && isHelpRoute(parseHash())) {
    if (relPath.value !== HELP_ROUTE) void navigate(HELP_ROUTE, { replace: true })
    return
  }
  const p = pathFromLocation()
  if (normalizeRelPath(p) !== relPath.value) void navigate(p, { replace: true })
}

onMounted(() => {
  document.title = `Index of ${relPath.value} - gofs`
  window.addEventListener('popstate', syncFromLocation)
  window.addEventListener('hashchange', syncFromLocation)

  // 带 hash 的地址：帮助页，或旧版本的 #/a/b 链接 —— 都以 hash 为准。
  if (location.hash && location.hash !== '#') {
    const hp = normalizeRelPath(parseHash())
    if (hp !== relPath.value) {
      void navigate(hp, { replace: true })
    } else {
      const url = isHelpRoute(hp) ? location.pathname + '#' + HELP_ROUTE : dirUrl(relPath.value)
      history.replaceState(null, '', url)
    }
    return
  }
  // 直接打开了某个目录 URL（新窗口打开、书签、刷新）时，服务端已按该路径
  // 渲染了数据，relPath 就是当前目录；把地址规范成不含多余参数的规范形式。
  history.replaceState(null, '', dirUrl(relPath.value))
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
  window.removeEventListener('popstate', syncFromLocation)
  window.removeEventListener('hashchange', syncFromLocation)
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
          @download="onDownloadItem"
          @delete="onDeleteItem"
          @rename="onRenameItem"
          @copy="onCopyItem"
          @copy-name="onCopyName"
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
