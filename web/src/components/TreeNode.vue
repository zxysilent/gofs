<script setup lang="ts">
import { computed } from 'vue'
import type { TreeNode } from '../types'
import { ICONS } from '../utils/icons'
import { t } from '../locales'

const props = defineProps<{
  node: TreeNode
  currentPath: string
  focusedPath: string
}>()
const emit = defineEmits<{
  (e: 'select', path: string): void
  (e: 'toggle', node: TreeNode): void
  (e: 'focus', path: string): void
  (e: 'keydown', ev: KeyboardEvent, node: TreeNode): void
  (e: 'menu', node: TreeNode, ev: MouseEvent): void
}>()

// 只有"已加载且确实没有子目录"时才隐藏箭头；未加载过一律保留，
// 因为子目录要等展开时才请求。加载过程中箭头位置显示 spinner，
// 这样点击后立刻有反馈，不会出现"点了没反应"的空窗期。
const hasChildren = computed(() => !props.node.loaded || props.node.children.length > 0)
const isEmpty = computed(() => props.node.loaded && props.node.children.length === 0)
const isActive = computed(() => props.node.path === props.currentPath)
const isFocused = computed(() => props.node.path === props.focusedPath)
const isOpen = computed(() => props.node.expanded && props.node.children.length > 0)

// 图标按状态区分：加载失败用警示图标，展开用打开态，确认空目录用空心文件夹。
const icon = computed(() => {
  if (props.node.error) return ICONS.alert
  if (isOpen.value) return ICONS.folderOpen
  if (isEmpty.value) return ICONS.folderEmpty
  return ICONS.folder
})

// 依赖 / 隐藏目录弱化显示：.git、node_modules 之类不该和业务目录抢注意力。
const DIM_NAMES = new Set([
  '.git',
  '.svn',
  '.hg',
  'node_modules',
  '__pycache__',
  '.venv',
  '.next',
  '.cache',
  '.pytest_cache',
])
const isDim = computed(() => props.node.name.startsWith('.') || DIM_NAMES.has(props.node.name))

function onToggle(e: MouseEvent) {
  e.stopPropagation()
  emit('toggle', props.node)
}

function onSelect() {
  emit('select', props.node.path)
}

function onContextMenu(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  emit('menu', props.node, e)
}

// 行内的 ⋯ 按钮：以按钮位置为锚点弹出同一个菜单。
function onMore(e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  emit('menu', props.node, {
    clientX: r.left,
    clientY: r.bottom + 2,
  } as MouseEvent)
}
</script>

<template>
  <li>
    <div
      class="tree-node"
      :class="{
        active: isActive,
        focused: isFocused,
        dim: isDim,
        empty: isEmpty,
        failed: !!node.error,
      }"
      role="treeitem"
      :aria-expanded="hasChildren ? (isOpen ? 'true' : 'false') : undefined"
      :aria-selected="isActive"
      :data-path="node.path"
      :tabindex="isFocused ? 0 : -1"
      :title="node.error || node.path"
      @click="onSelect"
      @focus="emit('focus', node.path)"
      @keydown="emit('keydown', $event, node)"
      @contextmenu="onContextMenu"
    >
      <span class="tree-toggle" @click="onToggle">
        <span v-if="node.loading" class="tree-spinner" v-html="ICONS.spinner"></span>
        <span
          v-else-if="hasChildren"
          class="tree-chevron"
          v-html="isOpen ? ICONS.chevronDown : ICONS.chevronRight"
        ></span>
      </span>
      <span class="tree-icon" v-html="icon"></span>
      <span class="tree-name">{{ node.name }}</span>
      <span class="tree-actions">
        <button
          class="tree-action-btn"
          :title="t('menu.more')"
          @click="onMore"
          v-html="ICONS.more"
        ></button>
      </span>
    </div>
    <ul v-if="isOpen" class="tree-children" role="group">
      <TreeNodeComp
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :current-path="currentPath"
        :focused-path="focusedPath"
        @select="emit('select', $event)"
        @toggle="emit('toggle', $event)"
        @focus="emit('focus', $event)"
        @keydown="(ev: KeyboardEvent, n: TreeNode) => emit('keydown', ev, n)"
        @menu="(n: TreeNode, e: MouseEvent) => emit('menu', n, e)"
      />
    </ul>
  </li>
</template>

<script lang="ts">
export default { name: 'TreeNodeComp' }
</script>
