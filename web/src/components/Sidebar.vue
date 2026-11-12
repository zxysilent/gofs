<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import type { IndexData, PathItem, TreeNode } from '../types'
import { normalizeRelPath, joinRelPath } from '../utils/paths'
import { listDir, absUrl, readText, serverVersion } from '../api'
import { ICONS } from '../utils/icons'
import { copyText, showToast } from '../utils/dialog'
import { locale, t, toggleLocale } from '../locales'
import { themeMode, cycleTheme } from '../utils/theme'
import { HELP_CONTENT } from '../lib/help-content'
import { PROJECT_URL } from '../utils'
import TreeNodeComp from './TreeNode.vue'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'

const props = defineProps<{ currentPath: string }>()
const emit = defineEmits<{
  (e: 'navigate', path: string): void
  (e: 'action', payload: { action: string; path: string; name: string }): void
}>()

// 顶层目录节点。目录树按层懒加载：展开某节点时才请求它的子目录，
// 因此不受仓库规模限制，也不会出现"节点预算耗尽后兄弟目录消失"的问题。
const topNodes = reactive<TreeNode[]>([])
const showHelp = ref(false)

/* ---------- 目录树：过滤 / 键盘焦点 / 展开态持久化 ---------- */

// 树滚动容器，用于把当前选中项滚进可视区。
const treeBodyRef = ref<HTMLElement | null>(null)
// 键盘漫游焦点（roving tabindex）。与"当前目录"分离：移动焦点不等于导航。
const focusedPath = ref('')

// 展开状态存 localStorage，刷新页面后目录树不会整体塌陷回根目录。
const EXPANDED_KEY = 'gofs.tree.expanded'

function readExpanded(): string[] {
  try {
    const arr = JSON.parse(localStorage.getItem(EXPANDED_KEY) || '[]')
    return Array.isArray(arr) ? arr.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

function saveExpanded() {
  // 根目录还没加载好时写入，会用空数组把用户上次的展开状态抹掉，
  // 因此只在树真正就绪后才持久化。
  if (!topNodes.length) return
  const paths: string[] = []
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      if (n.expanded) {
        paths.push(n.path)
        if (n.children.length) walk(n.children)
      }
    }
  }
  walk(topNodes)
  try {
    localStorage.setItem(EXPANDED_KEY, JSON.stringify(paths))
  } catch {
    /* 隐私模式 / 配额不足时静默放弃持久化 */
  }
}

// 扁平化"当前可见"的节点序列，供键盘上下移动使用。
const visibleNodes = computed<TreeNode[]>(() => {
  const out: TreeNode[] = []
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      out.push(n)
      if (n.expanded && n.children.length) walk(n.children)
    }
  }
  walk(topNodes)
  return out
})

// 整棵树里只有一个 treeitem 处于 tab 顺序中（roving tabindex）。
const tabStopPath = computed(() => {
  const list = visibleNodes.value
  if (focusedPath.value && list.some((n) => n.path === focusedPath.value)) return focusedPath.value
  return list[0]?.path ?? ''
})

/* ---------- 目录过滤 ---------- */

const filterText = ref('')
const filterLoading = ref(false)
const filterResults = ref<{ path: string; name: string; dir: string }[]>([])
const filtering = computed(() => filterText.value.trim() !== '')

let filterTimer: ReturnType<typeof setTimeout> | undefined

watch(filterText, (val) => {
  if (filterTimer) clearTimeout(filterTimer)
  const q = val.trim()
  if (!q) {
    filterResults.value = []
    filterLoading.value = false
    return
  }
  filterLoading.value = true
  filterTimer = setTimeout(() => void runFilter(q), 250)
})

// 目录过滤复用服务端的递归搜索接口，因此未展开的层级也能搜到，
// 不依赖树的加载状态，也不会因为懒加载而漏结果。
async function runFilter(q: string) {
  try {
    // dirsOnly：只返回匹配的目录，文件不必穿回来再丢掉。
    const data = await listDir('/', { q, sort: 'name', order: 'asc', dirsOnly: true })
    if (filterText.value.trim() !== q) return // 结果已过期
    filterResults.value = (data.paths || [])
      .filter((p) => p.path_type.endsWith('Dir'))
      .map((p) => {
        const full = normalizeRelPath('/' + p.name)
        const cut = full.lastIndexOf('/')
        return {
          path: full,
          name: full.slice(cut + 1),
          dir: cut <= 0 ? '/' : full.slice(0, cut),
        }
      })
  } catch {
    if (filterText.value.trim() === q) filterResults.value = []
  } finally {
    if (filterText.value.trim() === q) filterLoading.value = false
  }
}

function clearFilter() {
  filterText.value = ''
}

// 使用说明弹窗分两个 tab，分别面向人类与 agent。
const activeTab = ref<'human' | 'agent'>('human')

// 后端提供的原始 Markdown 文档：readme 面向人，agent 面向 AI 调用。
const README_DOC = '/gofs-readme.md'
const AGENT_DOC = '/gofs-agent.md'

// agent 手册按需加载（切到该 tab 时才请求）。
const agentDoc = ref('')
const agentLoading = ref(false)
const agentError = ref('')

const currentDoc = computed(() => (activeTab.value === 'human' ? README_DOC : AGENT_DOC))

// 当前语言下的帮助内容（静态双语数据）。
const helpSections = computed(() => HELP_CONTENT[locale.value])

// 带语言参数的文档地址：后端虚拟文档支持 ?lang=。
function docUrl(path: string): string {
  return `${path}?lang=${locale.value}`
}

// 主题切换按钮：图标 + 当前模式文字，暗色下也能一眼看出当前状态。
const themeIcon = computed(() => {
  if (themeMode.value === 'light') return ICONS.sun
  if (themeMode.value === 'dark') return ICONS.moon
  return ICONS.auto
})

const themeLabel = computed(
  () =>
    ({
      auto: t('settings.themeAuto'),
      light: t('settings.themeLight'),
      dark: t('settings.themeDark'),
    })[themeMode.value],
)

const themeTitle = computed(() => `${t('settings.theme')}: ${themeLabel.value}`)

// 语言按钮：直接用当前语言的自称，切换时图标随之变化。
const localeLabel = computed(() => (locale.value === 'zh' ? '中' : 'EN'))
const localeTitle = computed(
  () => `${t('settings.language')}: ${locale.value === 'zh' ? '中文' : 'English'}`,
)

async function selectTab(tab: 'human' | 'agent') {
  activeTab.value = tab
  if (tab !== 'agent' || agentDoc.value || agentLoading.value) return
  await loadAgentDoc()
}

// 按当前语言拉取 agent 手册（切换语言时重新拉取）。
async function loadAgentDoc() {
  agentLoading.value = true
  agentError.value = ''
  try {
    agentDoc.value = await readText(docUrl(AGENT_DOC))
  } catch (e) {
    agentError.value = e instanceof Error ? e.message : String(e)
  } finally {
    agentLoading.value = false
  }
}

// 语言变化后，若 agent 手册已加载则重新拉取对应语言的版本。
watch(locale, () => {
  if (agentDoc.value) void loadAgentDoc()
})

// 在新标签页打开当前 tab 对应的原始 Markdown。
function openDoc() {
  window.open(location.origin + absUrl(currentDoc.value) + '?lang=' + locale.value, '_blank', 'noopener')
}

async function copyDocUrl() {
  const url = location.origin + absUrl(currentDoc.value) + '?lang=' + locale.value
  const ok = await copyText(url)
  showToast(
    ok ? t('sidebar.copiedUrl', { url }) : t('sidebar.copyUrlFailed', { url }),
    ok ? 'success' : 'error',
  )
}

// 跳到项目主页（帮助面板唯一一个指向外部站点的入口）。
function openProject() {
  window.open(PROJECT_URL, '_blank', 'noopener')
}

// 复制 agent 手册全文，方便直接粘给 AI。
async function copyAgentDoc() {
  const ok = await copyText(agentDoc.value)
  showToast(
    ok ? t('toast.copiedManual') : t('toast.copyFailedManual'),
    ok ? 'success' : 'error',
  )
}

// 说明内容里允许少量内联 HTML（<code> 等），复制成纯文本时去掉标签并把实体还原。
function plainText(html: string): string {
  return html
    .replace(/<[^>]+>/g, '')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&amp;/g, '&')
}

// 拼接面向人的说明全文：小节标题 + 无标签的条目，方便直接粘出去。
const helpPlainText = computed(() =>
  helpSections.value
    .map((s) => `## ${plainText(s.title)}\n` + s.items.map((i) => `- ${plainText(i)}`).join('\n'))
    .join('\n\n'),
)

// 当前 tab 是否有可复制的全文（agent 手册要等加载完）。
const canCopyAll = computed(() => (activeTab.value === 'human' ? true : !!agentDoc.value))

async function copyAll() {
  if (activeTab.value === 'agent') return copyAgentDoc()
  const ok = await copyText(helpPlainText.value)
  showToast(
    ok ? t('toast.copiedGuide') : t('toast.copyFailedGuide'),
    ok ? 'success' : 'error',
  )
}

function makeNode(path: string, name: string): TreeNode {
  return reactive({ path, name, expanded: false, loaded: false, children: [] })
}

function dirsOf(items: PathItem[]): PathItem[] {
  return (items || []).filter((p) => p.path_type.endsWith('Dir'))
}

async function fetchDir(path: string): Promise<IndexData> {
  return listDir(path, { sort: 'name', order: 'asc', q: '' })
}

// 用服务端返回的条目刷新某个节点的子目录列表，并尽量复用已有节点
// （保留已展开状态，避免刷新后整棵树塌陷）。
// 同时记录这个目录自身的写权限，右键菜单据此决定显示哪些条目。
function applyChildren(node: TreeNode, items: PathItem[], perms?: IndexData) {
  const prev = new Map(node.children.map((c) => [c.path, c]))
  node.children = dirsOf(items).map((d) => {
    const path = normalizeRelPath(joinRelPath(node.path, d.name))
    const old = prev.get(path)
    if (old) {
      old.name = d.name
      return old
    }
    return makeNode(path, d.name)
  })
  node.loaded = true
  if (perms) {
    node.canUpload = perms.allow_upload
    node.canDelete = perms.allow_delete
  }
}

// 首次需要时才加载某个节点的子目录。加载过程置 loading（折叠箭头位置显示
// spinner），失败则记录 error 并保持未加载，这样用户可以点箭头重试，
// 而不是把一个请求失败的目录伪装成空目录。
async function loadNode(node: TreeNode) {
  if (node.loaded || node.loading) return
  node.loading = true
  node.error = undefined
  try {
    const data = await fetchDir(node.path)
    applyChildren(node, data.paths || [], data)
  } catch (e) {
    node.error = e instanceof Error ? e.message : String(e)
  } finally {
    node.loading = false
  }
}

async function loadRoot() {
  const prev = new Map(topNodes.map((n) => [n.path, n]))
  const data = await fetchDir('/')
  // 根目录自身的写权限：右键菜单作用于根上一级时使用（如"新建子目录"落在根）。
  rootPerms.value = { upload: data.allow_upload, remove: data.allow_delete }
  topNodes.splice(
    0,
    topNodes.length,
    ...dirsOf(data.paths || []).map((d) => {
      const path = normalizeRelPath('/' + d.name)
      return prev.get(path) || makeNode(path, d.name)
    }),
  )
}

// 恢复上次的展开状态：按层级从浅到深展开，已加载过的层级不会重复请求。
async function restoreExpanded() {
  const saved = readExpanded()
  if (!saved.length) return
  saved.sort((a, b) => a.split('/').length - b.split('/').length)
  for (const p of saved) await expandTo(p)
}

async function expandTo(path: string) {
  const parts = path.split('/').filter(Boolean)
  let list: TreeNode[] = topNodes
  let acc = ''
  for (let i = 0; i < parts.length; i++) {
    acc = joinRelPath(acc || '/', parts[i])
    const node = list.find((n) => n.path === acc)
    if (!node) return
    if (!node.loaded) await loadNode(node)
    if (!node.children.length) return
    node.expanded = true
    if (i === parts.length - 1) return
    list = node.children
  }
}

// 根目录只加载一次；onNavigated 可能早于组件挂载触发，必须等待它完成，
// 否则遍历时 topNodes 还是空的，会导致当前目录及其祖先没有进入树。
// 注意：这里不能让 promise reject，也不能在失败时清空 rootReady——
// 否则并发调用会各自重新加载根目录，互相覆盖 topNodes，把恢复出来的展开态冲掉。
let rootReady: Promise<void> | null = null
let rootFailed = false
function ensureRoot(): Promise<void> {
  if (rootReady && !rootFailed) return rootReady
  rootFailed = false
  rootReady = loadRoot()
    .then(() => restoreExpanded())
    .catch(() => {
      rootFailed = true // 下次导航重试
    })
  return rootReady
}

// 串行化导航：并发的 onNavigated（首次加载、hashchange、刷新）会交替遍历同一棵树，
// 导致某些层级被跳过，因此排队执行。
let navChain: Promise<void> = Promise.resolve()

// 展开节点的折叠箭头（懒加载子目录）。
async function onToggle(node: TreeNode) {
  if (node.expanded) {
    node.expanded = false
    node.collapsed = true // 记录用户主动折叠，导航刷新时不再自动展开
    saveExpanded()
    return
  }
  await openNode(node)
}

// 展开并加载子目录；确认没有子目录时保持折叠（箭头随之隐藏）。
async function openNode(node: TreeNode) {
  await loadNode(node)
  node.collapsed = false
  node.expanded = node.children.length > 0
  saveExpanded()
}

// 折叠整棵树（键盘/按钮共用），当前目录的高亮仍在，只是被折叠起来。
function collapseAll() {
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      n.expanded = false
      n.collapsed = true
      if (n.children.length) walk(n.children)
    }
  }
  walk(topNodes)
  saveExpanded()
}

function parentPathOf(path: string): string {
  const idx = path.lastIndexOf('/')
  return idx <= 0 ? '/' : path.slice(0, idx)
}

function nodeEl(path: string): HTMLElement | null {
  const body = treeBodyRef.value
  if (!body) return null
  return body.querySelector<HTMLElement>(`.tree-node[data-path="${CSS.escape(path)}"]`)
}

// 把某个节点滚进侧栏可视区。不用 scrollIntoView，避免连带滚动外层容器。
async function revealPath(path: string) {
  await nextTick()
  const body = treeBodyRef.value
  const el = nodeEl(path)
  if (!body || !el) return
  const br = body.getBoundingClientRect()
  const er = el.getBoundingClientRect()
  const pad = 16
  if (er.top < br.top + pad) {
    body.scrollTop -= br.top + pad - er.top
  } else if (er.bottom > br.bottom - pad) {
    body.scrollTop += er.bottom - br.bottom + pad
  }
}

async function focusNodeAt(index: number) {
  const list = visibleNodes.value
  if (!list.length) return
  const i = Math.min(list.length - 1, Math.max(0, index))
  focusedPath.value = list[i].path
  await nextTick()
  nodeEl(list[i].path)?.focus()
}

// 目录树的键盘导航：↑↓ 移动、→ 展开/进入、← 折叠/回到父级、Home/End 首尾、Enter 打开。
function onTreeKeydown(ev: KeyboardEvent, node: TreeNode) {
  const list = visibleNodes.value
  const idx = list.findIndex((n) => n.path === node.path)
  switch (ev.key) {
    case 'ArrowDown':
      ev.preventDefault()
      void focusNodeAt(idx + 1)
      break
    case 'ArrowUp':
      ev.preventDefault()
      void focusNodeAt(idx - 1)
      break
    case 'Home':
      ev.preventDefault()
      void focusNodeAt(0)
      break
    case 'End':
      ev.preventDefault()
      void focusNodeAt(list.length - 1)
      break
    case 'ArrowRight':
      ev.preventDefault()
      if (node.expanded) void focusNodeAt(idx + 1)
      else void openNode(node)
      break
    case 'ArrowLeft': {
      ev.preventDefault()
      if (node.expanded) {
        node.expanded = false
        node.collapsed = true
        saveExpanded()
        break
      }
      const parent = parentPathOf(node.path)
      if (parent !== '/') {
        const pi = list.findIndex((n) => n.path === parent)
        if (pi >= 0) void focusNodeAt(pi)
      }
      break
    }
    case 'Enter':
    case ' ':
      ev.preventDefault()
      onSelect(node.path)
      break
  }
}

// 导航后由父组件调用：沿当前路径逐层加载并展开祖先目录。
// 逐层请求，不会递归展开整棵树；当前目录直接复用列表页已取到的 items，
// 不重复发请求（items 省略表示列表不可用，例如搜索时）。
function onNavigated(path: string, items?: PathItem[]): Promise<void> {
  navChain = navChain.then(() => walkTo(path, items)).catch(() => {})
  return navChain
}

async function walkTo(path: string, items?: PathItem[]) {
  path = normalizeRelPath(path)
  await ensureRoot()
  // 键盘焦点跟随当前目录，Tab 进入目录树时能直接落在附近。
  focusedPath.value = path
  if (path === '/') return
  const parts = path.split('/').filter(Boolean)
  let list: TreeNode[] = topNodes
  let acc = ''
  for (let i = 0; i < parts.length; i++) {
    acc = joinRelPath(acc || '/', parts[i])
    const node = list.find((n) => n.path === acc)
    if (!node) break // 该目录不在树中（隐藏或无权限），保持现状
    if (i === parts.length - 1) {
      if (items) applyChildren(node, items)
      else await loadNode(node)
      // 当前目录有子目录时展开，避免"树里看不到当前目录下的文件夹"；
      // 用户手动折叠过的则不打扰。
      if (node.children.length > 0 && !node.collapsed) node.expanded = true
    } else {
      if (!node.loaded) await loadNode(node)
      if (node.children.length > 0) node.expanded = true
    }
    list = node.children
  }
  saveExpanded()
  // 深路径下高亮可能落在可视区之外，导航后主动把它滚进视野。
  await revealPath(path)
}


function onSelect(path: string) {
  emit('navigate', path)
}

// 重建目录树（重命名/删除后由父组件调用），并恢复到当前路径。
async function rebuildTree() {
  topNodes.splice(0, topNodes.length)
  navChain = Promise.resolve()
  rootFailed = false
  rootReady = loadRoot().then(() => restoreExpanded())
  try {
    await rootReady
  } catch {
    rootFailed = true
  }
  await walkTo(props.currentPath)
}

/* ---------- 右键菜单 ---------- */

// 根目录的写权限（根节点本身不对应树里的某一行）。
const rootPerms = ref<{ upload: boolean; remove: boolean }>({ upload: false, remove: false })

const menu = ref<{ open: boolean; x: number; y: number; node: TreeNode | null }>({
  open: false,
  x: 0,
  y: 0,
  node: null,
})

const menuItems = computed<MenuItem[]>(() => {
  const node = menu.value.node
  if (!node) return []
  const canWrite = node.canUpload ?? rootPerms.value.upload
  const canRemove = node.canDelete ?? rootPerms.value.remove
  return [
    {
      key: 'new-folder',
      label: t('menu.newFolder'),
      icon: 'newFolder',
      disabled: !canWrite,
    },
    { key: 'copy-rel-path', label: t('menu.copyRelPath'), icon: 'copy', dividerBefore: true },
    { key: 'reload', label: t('common.refresh'), icon: 'refresh' },
    { key: 'rename', label: t('action.rename'), icon: 'rename', dividerBefore: true, disabled: !canWrite },
    { key: 'delete', label: t('action.delete'), icon: 'trash', danger: true, disabled: !canRemove },
  ]
})

function openMenu(node: TreeNode, ev: MouseEvent) {
  menu.value = { open: true, x: ev.clientX, y: ev.clientY, node }
  focusedPath.value = node.path
}

function closeMenu() {
  menu.value = { ...menu.value, open: false, node: null }
}

// 重新拉取单个目录的子目录列表（不重建整棵树，保留其他分支的展开状态）。
async function reloadNode(node: TreeNode) {
  node.loaded = false
  node.error = undefined
  await loadNode(node)
  node.expanded = node.children.length > 0
  saveExpanded()
}

function onMenuSelect(key: string) {
  const node = menu.value.node
  if (!node) return
  closeMenu()
  switch (key) {
    case 'new-folder':
      // 交给父组件弹窗并调用 mkdir，成功后重建树。
      emit('action', { action: 'new-folder', path: node.path, name: node.name })
      break
    case 'copy-rel-path':
      void copyText(node.path).then((ok) =>
        showToast(
          ok ? t('toast.copyRelPath') : t('toast.copyFailed', { text: node.path }),
          ok ? 'success' : 'error',
        ),
      )
      break
    case 'reload':
      void reloadNode(node)
      break
    case 'rename':
    case 'delete':
      emit('action', { action: key, path: node.path, name: node.name })
      break
  }
}

defineExpose({ onNavigated, rebuildTree })

onMounted(() => {
  void ensureRoot()
})
</script>

<template>
  <div class="sidebar-inner">
    <div class="sidebar-head">
      <div class="logo"><span v-html="ICONS.folder"></span><span>gofs</span></div>
      <span v-if="serverVersion" class="sidebar-version" :title="`gofs ${serverVersion}`">
        v{{ serverVersion }}
      </span>
    </div>
    <!-- FILES 固定在滚动区之外，展开长目录树时不会跟着滚走 -->
    <div
      class="sidebar-label sidebar-label-clickable"
      :class="{ active: currentPath === '/' }"
      :title="t('sidebar.rootTitle')"
      @click="onSelect('/')"
    >
      <span class="sidebar-label-icon" v-html="ICONS.list"></span>{{ t('sidebar.fileList') }}
    </div>
    <!-- 过滤目录 + 全部折叠，同样固定在滚动区外 -->
    <div class="tree-tools">
      <label class="tree-filter">
        <span class="tree-filter-icon" v-html="ICONS.search"></span>
        <input
          v-model="filterText"
          class="tree-filter-input"
          type="text"
          :placeholder="t('sidebar.filterPlaceholder')"
          spellcheck="false"
        />
        <button
          v-if="filtering"
          class="tree-filter-clear"
          :title="t('common.cancel')"
          @click="clearFilter"
          v-html="ICONS.close"
        ></button>
      </label>
      <button
        class="tree-tool-btn"
        :title="t('sidebar.collapseAll')"
        @click="collapseAll"
        v-html="ICONS.unfoldLess"
      ></button>
    </div>
    <div ref="treeBodyRef" class="sidebar-body">
      <!-- 过滤结果：平铺展示匹配的目录（未展开的层级也能搜到） -->
      <ul v-if="filtering" class="tree tree-filter-results" role="tree">
        <li v-for="hit in filterResults" :key="hit.path">
          <div
            class="tree-node"
            :class="{ active: hit.path === currentPath }"
            role="treeitem"
            tabindex="0"
            :title="hit.path"
            @click="onSelect(hit.path)"
            @keydown.enter.prevent="onSelect(hit.path)"
          >
            <span class="tree-icon" v-html="ICONS.folder"></span>
            <span class="tree-name">{{ hit.name }}</span>
            <span class="tree-path">{{ hit.dir }}</span>
          </div>
        </li>
        <li v-if="filterLoading" class="tree-hint">{{ t('common.loading') }}</li>
        <li v-else-if="!filterResults.length" class="tree-hint">
          {{ t('sidebar.filterEmpty') }}
        </li>
      </ul>
      <ul v-else class="tree" role="tree" :aria-label="t('sidebar.fileList')">
        <TreeNodeComp
          v-for="node in topNodes"
          :key="node.path"
          :node="node"
          :current-path="currentPath"
          :focused-path="tabStopPath"
          @select="onSelect"
          @toggle="onToggle"
          @focus="focusedPath = $event"
          @keydown="onTreeKeydown"
          @menu="openMenu"
        />
      </ul>
    </div>
    <!-- 右键菜单：整个侧栏共用一个实例，避免每行挂一份 -->
    <ContextMenu
      v-if="menu.open && menu.node"
      :x="menu.x"
      :y="menu.y"
      :items="menuItems"
      @select="onMenuSelect"
      @close="closeMenu"
    />
    <div class="sidebar-foot">
      <button class="help-btn" @click="showHelp = true">
        <span class="help-btn-icon" v-html="ICONS.info"></span>{{ t('sidebar.help') }}
      </button>
      <div class="sidebar-settings">
        <button class="setting-btn" :title="themeTitle" @click="cycleTheme()">
          <span class="setting-icon" v-html="themeIcon"></span>
          <span class="setting-text">{{ themeLabel }}</span>
        </button>
        <button class="setting-btn" :title="localeTitle" @click="toggleLocale()">
          <span class="setting-icon" v-html="ICONS.globe"></span>
          <span class="setting-lang">{{ localeLabel }}</span>
        </button>
      </div>
    </div>

    <!-- 使用说明弹窗 -->
    <div v-if="showHelp" class="modal-overlay" @click.self="showHelp = false">
      <div class="modal-box help-box">
        <div class="help-head">
          <div class="help-tabs">
            <button
              class="help-tab"
              :class="{ active: activeTab === 'human' }"
              @click="selectTab('human')"
            >{{ t('sidebar.tabGuide') }}</button>
            <button
              class="help-tab"
              :class="{ active: activeTab === 'agent' }"
              @click="selectTab('agent')"
            >{{ t('sidebar.tabAgent') }}</button>
          </div>
          <div class="toolbar-spacer"></div>
          <button
            v-if="canCopyAll"
            class="btn"
            :title="t('sidebar.copyAllTitle')"
            @click="copyAll"
          >
            <span v-html="ICONS.copy"></span>{{ t('sidebar.copyAll') }}
          </button>
          <button class="btn" :title="t('sidebar.rawFileTitle')" @click="openDoc">
            <span v-html="ICONS.external"></span>{{ t('sidebar.rawFile') }}
          </button>
          <button class="btn" :title="t('sidebar.copyUrlTitle')" @click="copyDocUrl">
            <span v-html="ICONS.copy"></span>{{ t('sidebar.copyUrl') }}
          </button>
          <button class="btn" :title="t('sidebar.projectUrlTitle')" @click="openProject">
            <span v-html="ICONS.globe"></span>{{ t('sidebar.projectUrl') }}
          </button>
        </div>
        <div v-show="activeTab === 'human'" class="help-content">
          <template v-for="section in helpSections" :key="section.title">
            <h4>{{ section.title }}</h4>
            <ul>
              <li v-for="(item, i) in section.items" :key="i" v-html="item"></li>
            </ul>
          </template>
        </div>
        <div v-show="activeTab === 'agent'" class="help-content help-agent">
          <div v-if="agentLoading" class="file-loading">
            <span v-html="ICONS.spinner"></span><span>{{ t('common.loading') }}</span>
          </div>
          <div v-else-if="agentError" class="error-banner">
            {{ t('common.loadFailed', { msg: agentError }) }}
          </div>
          <pre v-else class="help-md">{{ agentDoc }}</pre>
        </div>
        <div class="modal-actions">
          <button class="btn btn-primary" @click="showHelp = false">
            <span v-html="ICONS.x"></span>{{ t('common.close') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
