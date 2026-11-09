<script setup lang="ts">
import { computed } from 'vue'
import type { TreeNode } from '../types'
import { ICONS } from '../icons'

const props = defineProps<{ node: TreeNode; currentPath: string }>()
const emit = defineEmits<{
  (e: 'select', path: string): void
  (e: 'toggle', node: TreeNode): void
  (e: 'action', payload: { action: string; path: string; name: string }): void
}>()

// 未加载过子目录的节点也要显示折叠箭头（子目录在展开时才请求）。
const hasChildren = computed(() => !props.node.loaded || props.node.children.length > 0)
const isActive = computed(() => props.node.path === props.currentPath)

function onToggle(e: MouseEvent) {
  e.stopPropagation()
  emit('toggle', props.node)
}

function onSelect() {
  emit('select', props.node.path)
}

function onAction(action: string, e: MouseEvent) {
  e.preventDefault()
  e.stopPropagation()
  emit('action', { action, path: props.node.path, name: props.node.name })
}
</script>

<template>
  <li>
    <div
      class="tree-node"
      :class="{ active: isActive }"
      :data-path="node.path"
      :title="node.path"
      @click="onSelect"
    >
      <span
        class="tree-toggle"
        :style="{ visibility: hasChildren ? 'visible' : 'hidden' }"
        @click="onToggle"
        v-html="node.expanded ? ICONS.chevronDown : ICONS.chevronRight"
      ></span>
      <span class="tree-icon" v-html="ICONS.folder"></span>
      <span class="tree-name">{{ node.name }}</span>
      <span class="tree-actions">
        <button class="tree-action-btn" title="重命名" @click="onAction('rename', $event)" v-html="ICONS.rename"></button>
        <button class="tree-action-btn" title="复制路径" @click="onAction('copy-path', $event)" v-html="ICONS.copy"></button>
        <button class="tree-action-btn danger" title="删除" @click="onAction('delete', $event)" v-html="ICONS.trash"></button>
      </span>
    </div>
    <ul v-show="node.expanded" v-if="hasChildren" class="tree-children">
      <TreeNodeComp
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :current-path="currentPath"
        @select="emit('select', $event)"
        @toggle="emit('toggle', $event)"
        @action="emit('action', $event)"
      />
    </ul>
  </li>
</template>

<script lang="ts">
export default { name: 'TreeNodeComp' }
</script>
