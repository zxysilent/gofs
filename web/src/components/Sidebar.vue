<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import type { PathItem, TreeNode } from '../types'
import { normalizeRelPath, joinRelPath } from '../types'
import { listDir, absUrl, readText } from '../api'
import { ICONS } from '../icons'
import { copyText, showToast } from '../dialog'
import { locale, t, toggleLocale } from '../i18n'
import { themeMode, resolvedTheme, cycleTheme } from '../theme'
import { HELP_CONTENT } from '../help-content'
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

// 复制 agent 手册全文，方便直接粘给 AI。
async function copyAgentDoc() {
  const ok = await copyText(agentDoc.value)
  showToast(
    ok ? t('toast.copiedManual') : t('toast.copyFailedManual'),
    ok ? 'success' : 'error',
  )
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
    <!-- FILES 固定在滚动区之外，展开长目录树时不会跟着滚走 -->
    <div
      class="sidebar-label sidebar-label-clickable"
      :class="{ active: currentPath === '/' }"
      :title="t('sidebar.rootTitle')"
      @click="onSelect('/')"
    >
      <span class="sidebar-label-icon" v-html="ICONS.list"></span>{{ t('sidebar.fileList') }}
    </div>
    <div class="sidebar-body">
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
        <span class="help-btn-icon" v-html="ICONS.info"></span>{{ t('sidebar.help') }}
      </button>
      <div class="sidebar-settings">
        <button class="setting-btn" :title="themeTitle" @click="cycleTheme()">
          <span class="setting-icon" v-html="themeIcon"></span>
          <span class="setting-text">{{ themeLabel }}</span>
        </button>
        <button class="setting-btn" :title="localeTitle" @click="toggleLocale()">
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
            v-if="activeTab === 'agent' && agentDoc"
            class="btn"
            :title="t('sidebar.copyAll')"
            @click="copyAgentDoc"
          >
            <span v-html="ICONS.copy"></span>{{ t('sidebar.copyAll') }}
          </button>
          <button class="btn" :title="t('sidebar.rawFileTitle')" @click="openDoc">
            <span v-html="ICONS.external"></span>{{ t('sidebar.rawFile') }}
          </button>
          <button class="btn" :title="t('sidebar.copyUrlTitle')" @click="copyDocUrl">
            <span v-html="ICONS.copy"></span>{{ t('sidebar.copyUrl') }}
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
          <button class="btn btn-primary" @click="showHelp = false">{{ t('common.close') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>
