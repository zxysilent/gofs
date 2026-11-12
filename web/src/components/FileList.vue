<script setup lang="ts">
import { reactive } from 'vue'
import type { PathItem } from '../types'
import { isDir } from '../utils/paths'
import { fileKind, formatSize, formatDirSize, formatTime } from '../utils'
import { ICONS } from '../utils/icons'
import { t } from '../locales'
import ContextMenu, { type MenuItem } from './ContextMenu.vue'

const props = defineProps<{
  items: PathItem[]
  loading: boolean
  allowUpload: boolean
  allowDelete: boolean
  allowArchive: boolean
}>()

const emit = defineEmits<{
  (e: 'select', item: PathItem): void
  (e: 'download', item: PathItem): void
  (e: 'delete', item: PathItem): void
  (e: 'rename', item: PathItem): void
  (e: 'copy', item: PathItem): void
  (e: 'copy-name', item: PathItem): void
  (e: 'copy-curl', item: PathItem): void
  (e: 'open-new', item: PathItem): void
  (e: 'download-archive', item: PathItem): void
}>()

// 行内只保留两个主操作，其余收进“更多”菜单，避免一长串小图标。
const menu = reactive<{ open: boolean; x: number; y: number; item: PathItem | null }>({
  open: false,
  x: 0,
  y: 0,
  item: null,
})

function openMenu(item: PathItem, ev: MouseEvent) {
  // 锚到按钮右下角，菜单不会盖住被操作的行
  const rect = (ev.currentTarget as HTMLElement).getBoundingClientRect()
  menu.item = item
  menu.x = rect.right
  menu.y = rect.bottom + 4
  menu.open = true
}

function closeMenu() {
  menu.open = false
  menu.item = null
}

function menuItems(): MenuItem[] {
  // 行内主操作已经占了“打包下载”，菜单里不再重复；写操作按权限置灰，而不是直接消失
  return [
    { key: 'copy-curl', label: t('menu.copyCurl'), icon: 'terminal' },
    { key: 'open-new', label: t('menu.openNew'), icon: 'external' },
    { key: 'rename', label: t('action.rename'), icon: 'rename', dividerBefore: true, disabled: !props.allowUpload },
    { key: 'delete', label: t('action.delete'), icon: 'trash', danger: true, disabled: !props.allowDelete },
  ]
}

function onMenuSelect(key: string) {
  const item = menu.item
  closeMenu()
  if (!item) return
  switch (key) {
    case 'copy-curl': emit('copy-curl', item); break
    case 'open-new': emit('open-new', item); break
    case 'download-archive': emit('download-archive', item); break
    case 'rename': if (props.allowUpload) emit('rename', item); break
    case 'delete': if (props.allowDelete) emit('delete', item); break
  }
}

function kindLabel(item: PathItem): string {
  if (isDir(item)) return t('kind.dir')
  switch (fileKind(item.name)) {
    case 'image': return t('kind.image')
    case 'video': return t('kind.video')
    case 'audio': return t('kind.audio')
    case 'pdf': return t('kind.pdf')
    case 'markdown': return t('kind.markdown')
    case 'html': return t('kind.html')
    case 'json': return t('kind.json')
    case 'text': return t('kind.text')
    case 'code': return t('kind.code')
    case 'archive': return t('kind.archive')
    default: return t('kind.file')
  }
}
</script>

<template>
  <div class="file-area">
    <div v-if="loading" class="file-loading">
      <span v-html="ICONS.spinner"></span><span>{{ t('common.loading') }}</span>
    </div>

    <!-- 空目录不显示任何占位内容 -->
    <div v-else class="file-list">
      <div class="file-list-head">
        <div class="col-name">{{ t('list.name') }}</div>
        <div class="col-type">{{ t('list.type') }}</div>
        <div class="col-size">{{ t('list.size') }}</div>
        <div class="col-mtime">{{ t('list.mtime') }}</div>
        <div class="col-actions"></div>
      </div>
      <div
        v-for="item in items"
        :key="item.name"
        class="file-row"
        @click="emit('select', item)"
      >
        <div class="col-name">
          <span class="file-icon" :class="'kind-' + fileKind(item.name)" v-html="isDir(item) ? ICONS.folder : ICONS.file"></span>
          <span class="file-name" :title="item.name">{{ item.name }}</span>
          <!-- 仅悬停时出现的小按钮，平时不干扰阅读 -->
          <button
            class="icon-btn name-copy"
            :title="t('action.copyName')"
            @click.stop="emit('copy-name', item)"
            v-html="ICONS.copy"
          ></button>
        </div>
        <div class="col-type">{{ kindLabel(item) }}</div>
        <div class="col-size">{{ isDir(item) ? formatDirSize(item.size) : formatSize(item.size) }}</div>
        <div class="col-mtime">{{ formatTime(item.mtime) }}</div>
        <div class="col-actions" @click.stop>
          <!-- 两个主操作：目录是“打包下载”，文件是“下载” -->
          <button
            v-if="allowArchive && isDir(item)"
            class="icon-btn"
            :title="t('action.downloadArchive')"
            @click="emit('download-archive', item)"
            v-html="ICONS.archive"
          ></button>
          <button
            v-else-if="!isDir(item)"
            class="icon-btn"
            :title="t('preview.download')"
            @click="emit('download', item)"
            v-html="ICONS.download"
          ></button>
          <button
            class="icon-btn"
            :title="t('action.copyPath')"
            @click="emit('copy', item)"
            v-html="ICONS.link"
          ></button>
          <!-- 其余功能（curl / 新窗口 / 重命名 / 删除）收进菜单 -->
          <button
            class="icon-btn"
            :title="t('menu.more')"
            @click="openMenu(item, $event)"
            v-html="ICONS.more"
          ></button>
        </div>
      </div>
    </div>
    <ContextMenu
      v-if="menu.open && menu.item"
      :x="menu.x"
      :y="menu.y"
      :items="menuItems()"
      @select="onMenuSelect"
      @close="closeMenu"
    />
  </div>
</template>
