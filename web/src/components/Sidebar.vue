<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { PathItem, TreeNode } from '../types'
import { normalizeRelPath, joinRelPath } from '../types'
import { listDir, absUrl } from '../api'
import { ICONS } from '../icons'
import { copyText, showToast } from '../dialog'
import TreeNodeComp from './TreeNode.vue'

const props = defineProps<{ currentPath: string }>()
const emit = defineEmits<{
  (e: 'navigate', path: string): void
  (e: 'action', payload: { action: string; path: string; name: string }): void
}>()

// 顶层目录节点。目录树按层懒加载：展开某节点时才请求它的子目录，
// 因此不受仓库规模限制，也不会出现"节点预算耗尽后兄弟目录消失"的问题。
const topNodes = reactive<TreeNode[]>([])
const showHelp = ref(false)

// 使用说明文档地址（后端提供的原始 Markdown：面向使用者的 gofs-readme.md）。
const README_DOC = '/gofs-readme.md'
const helpPageUrl = computed(() => location.origin + absUrl(README_DOC))

function openHelpPage() {
  window.open(helpPageUrl.value, '_blank', 'noopener')
}

async function copyHelpPageUrl() {
  const ok = await copyText(helpPageUrl.value)
  showToast(ok ? '已复制地址: ' + helpPageUrl.value : '复制失败: ' + helpPageUrl.value, ok ? 'success' : 'error')
}

function makeNode(path: string, name: string): TreeNode {
  return reactive({ path, name, expanded: false, loaded: false, children: [] })
}

function dirsOf(items: PathItem[]): PathItem[] {
  return (items || []).filter((p) => p.path_type.endsWith('Dir'))
}

async function fetchDirItems(path: string): Promise<PathItem[]> {
  try {
    const data = await listDir(path, { sort: 'name', order: 'asc', q: '' })
    return data.paths || []
  } catch {
    return []
  }
}

// 用服务端返回的条目刷新某个节点的子目录列表，并尽量复用已有节点
// （保留已展开状态，避免刷新后整棵树塌陷）。
function applyChildren(node: TreeNode, items: PathItem[]) {
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
}

// 首次需要时才加载某个节点的子目录。
async function loadNode(node: TreeNode) {
  if (node.loaded) return
  applyChildren(node, await fetchDirItems(node.path))
}

async function loadRoot() {
  const prev = new Map(topNodes.map((n) => [n.path, n]))
  const dirs = dirsOf(await fetchDirItems('/'))
  topNodes.splice(
    0,
    topNodes.length,
    ...dirs.map((d) => {
      const path = normalizeRelPath('/' + d.name)
      return prev.get(path) || makeNode(path, d.name)
    }),
  )
}

// 根目录只加载一次；onNavigated 可能早于组件 onMounted 触发，必须等待它完成，
// 否则遍历时 topNodes 还是空的，会导致当前目录及其祖先没有进入树。
let rootReady: Promise<void> | null = null
function ensureRoot(): Promise<void> {
  if (!rootReady) rootReady = loadRoot()
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
    return
  }
  await loadNode(node)
  node.collapsed = false
  node.expanded = node.children.length > 0
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
  if (path === '/') return
  const parts = path.split('/').filter(Boolean)
  let list: TreeNode[] = topNodes
  let acc = ''
  for (let i = 0; i < parts.length; i++) {
    acc = joinRelPath(acc || '/', parts[i])
    const node = list.find((n) => n.path === acc)
    if (!node) return // 该目录不在树中（隐藏或无权限），保持现状
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
}

function onSelect(path: string) {
  emit('navigate', path)
}

// 重建目录树（重命名/删除后由父组件调用），并恢复到当前路径。
async function rebuildTree() {
  topNodes.splice(0, topNodes.length)
  navChain = Promise.resolve()
  rootReady = loadRoot()
  await rootReady
  await walkTo(props.currentPath)
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
    </div>
    <div class="sidebar-body">
      <div
        class="sidebar-label sidebar-label-clickable"
        :class="{ active: currentPath === '/' }"
        title="回到根目录"
        @click="onSelect('/')"
      >
        <span class="sidebar-label-icon" v-html="ICONS.list"></span>文件列表
      </div>
      <ul class="tree">
        <TreeNodeComp
          v-for="node in topNodes"
          :key="node.path"
          :node="node"
          :current-path="currentPath"
          @select="onSelect"
          @toggle="onToggle"
          @action="emit('action', $event)"
        />
      </ul>
    </div>
    <div class="sidebar-foot">
      <button class="help-btn" @click="showHelp = true">
        <span class="help-btn-icon" v-html="ICONS.info"></span>使用说明
      </button>
    </div>

    <!-- 使用说明弹窗 -->
    <div v-if="showHelp" class="modal-overlay" @click.self="showHelp = false">
      <div class="modal-box help-box">
        <div class="help-head">
          <div class="modal-title">gofs 使用说明</div>
          <div class="toolbar-spacer"></div>
          <button class="btn" title="在新标签页打开 gofs-readme.md" @click="openHelpPage">
            <span v-html="ICONS.external"></span>新页面打开
          </button>
          <button class="btn" title="复制 gofs-readme.md 地址" @click="copyHelpPageUrl">
            <span v-html="ICONS.copy"></span>复制
          </button>
        </div>
        <div class="help-content">
          <h4>浏览与导航</h4>
          <ul>
            <li>左侧目录树点击文件夹进入；右侧列表点击文件夹进入、点击文件预览。</li>
            <li>顶部面包屑可快速回到任意上级目录；地址用 <code>#/路径</code> 记录，可收藏/刷新不丢位置。</li>
            <li>侧栏与内容之间的分隔条可左右拖动调整宽度。</li>
          </ul>
          <h4>搜索与排序</h4>
          <ul>
            <li>搜索框输入后回车，在当前目录递归搜索文件。</li>
            <li>按名称 / 修改时间 / 大小排序，再次点击切换升降序。</li>
          </ul>
          <h4>文件操作</h4>
          <ul>
            <li>上传：点"上传"按钮选择文件，显示实时进度。</li>
            <li>新建文件夹 / 新建文件；重命名、删除（均不可恢复，删除有二次确认）。</li>
            <li>打包下载：把当前目录或某个子目录打包为 zip 下载。</li>
            <li>复制完整路径 / 复制 curl 下载命令 / curl 上传命令，便于命令行操作。</li>
          </ul>
          <h4>curl 命令示例</h4>
          <ul>
            <li>上传文件：<code>curl -T local/path/to/file.ext http://host:port/path/to/file.ext</code></li>
            <li>下载文件：<code>curl -O http://host:port/path/to/file.ext</code></li>
            <li>查看列表：<code>curl "http://host:port/?json="</code></li>
            <li>搜索文件：<code>curl "http://host:port/?json=&amp;q=keyword"</code></li>
            <li>重命名/移动：<code>curl -X MOVE -H "Destination: http://host:port/new/path" http://host:port/old/path</code></li>
            <li>删除：<code>curl -X DELETE http://host:port/path/to/file.ext</code></li>
            <li>新建目录：<code>curl -X MKCOL http://host:port/path/to/dir</code></li>
            <li>打包下载目录：加 <code>?zip</code>，如 <code>curl -o dir.zip "http://host:port/path/to/dir?zip"</code></li>
            <li>获取文件 SHA256：加 <code>?hash</code>，如 <code>curl "http://host:port/path/to/file.ext?hash"</code></li>
          </ul>
          <h4>预览与编辑</h4>
          <ul>
            <li>支持图片、视频、音频、PDF、Markdown、代码、JSON、文本在线预览。</li>
            <li>文本类文件可直接点"编辑"在线修改，<code>Ctrl/Cmd+S</code> 保存；Markdown 编辑时可切换预览。</li>
          </ul>
        </div>
        <div class="modal-actions">
          <button class="btn btn-primary" @click="showHelp = false">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>
