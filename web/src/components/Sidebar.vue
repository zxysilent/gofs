<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import type { TreeNode } from '../types'
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

// 顶层的各个目录节点（不再有"根目录"这个根）。
const topNodes = reactive<TreeNode[]>([])
const built = ref(false)
const showHelp = ref(false)

const MAX_TREE_NODES = 2000
let nodeCount = 0

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
  nodeCount++
  return reactive({ path, name, expanded: false, children: [] })
}

// 完整目录树：一次性递归加载所有层级，展开折叠即时、无需再次请求。
// path 为 '/' 时填充 topNodes，否则填充传入 node 的 children。
async function buildTree() {
  nodeCount = 0
  await loadChildren('/', topNodes)
  built.value = true
  expandPathAncestors(props.currentPath)
}

async function loadChildren(path: string, out: TreeNode[]) {
  let data
  try {
    data = await listDir(path, { sort: 'name', order: 'asc', q: '' })
  } catch {
    return
  }
  const dirs = (data.paths || []).filter((p) => p.path_type.endsWith('Dir'))
  for (const d of dirs) {
    if (nodeCount >= MAX_TREE_NODES) return
    const child = makeNode(normalizeRelPath(joinRelPath(path, d.name)), d.name)
    out.push(child)
    await loadChildren(child.path, child.children)
  }
}

// 展开当前路径的所有祖先目录节点。
function expandPathAncestors(path: string) {
  if (path === '/') return
  const parts = path.split('/').filter(Boolean)
  let cur = topNodes.find((c) => c.path === '/' + parts[0])
  if (!cur) return
  if (cur.children.length > 0) cur.expanded = true
  let acc = '/' + parts[0]
  for (let i = 1; i < parts.length; i++) {
    acc = joinRelPath(acc, parts[i])
    const next = cur.children.find((c) => c.path === acc)
    if (!next) break
    if (next.children.length > 0) next.expanded = true
    cur = next
  }
}

// 导航后由父组件调用：展开祖先（高亮由 TreeNode 的 isActive computed 自动处理）。
function onNavigated(path: string) {
  if (built.value) expandPathAncestors(path)
}

function onSelect(path: string) {
  emit('navigate', path)
}

// 刷新目录树（重命名/删除/新建后由父组件调用）。
async function rebuildTree() {
  topNodes.splice(0, topNodes.length)
  built.value = false
  await buildTree()
}

defineExpose({ onNavigated, rebuildTree })

onMounted(buildTree)
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
