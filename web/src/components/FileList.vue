<script setup lang="ts">
import type { PathItem } from '../types'
import { isDir } from '../types'
import { fileKind, formatSize, formatDirSize, formatTime } from '../utils'
import { ICONS } from '../icons'
import { t } from '../i18n'

defineProps<{
  items: PathItem[]
  loading: boolean
  viewMode: 'list' | 'grid'
  allowUpload: boolean
  allowDelete: boolean
  allowArchive: boolean
}>()

const emit = defineEmits<{
  (e: 'select', item: PathItem): void
  (e: 'delete', item: PathItem): void
  (e: 'rename', item: PathItem): void
  (e: 'copy', item: PathItem): void
  (e: 'copy-curl', item: PathItem): void
  (e: 'open-new', item: PathItem): void
  (e: 'download-archive', item: PathItem): void
}>()

function kindLabel(item: PathItem): string {
  if (isDir(item)) return t('kind.dir')
  switch (fileKind(item.name)) {
    case 'image': return t('kind.image')
    case 'video': return t('kind.video')
    case 'audio': return t('kind.audio')
    case 'pdf': return t('kind.pdf')
    case 'markdown': return t('kind.markdown')
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
    <div v-else-if="viewMode === 'grid'" class="file-grid">
      <div
        v-for="item in items"
        :key="item.name"
        class="file-card"
        @click="emit('select', item)"
      >
        <div class="file-card-icon" :class="'kind-' + fileKind(item.name)" v-html="isDir(item) ? ICONS.folder : ICONS.file"></div>
        <div class="file-card-name" :title="item.name">{{ item.name }}</div>
      </div>
    </div>

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
          <span class="file-name">{{ item.name }}</span>
        </div>
        <div class="col-type">{{ kindLabel(item) }}</div>
        <div class="col-size">{{ isDir(item) ? formatDirSize(item.size) : formatSize(item.size) }}</div>
        <div class="col-mtime">{{ formatTime(item.mtime) }}</div>
        <div class="col-actions" @click.stop>
          <button
            v-if="allowArchive && isDir(item)"
            class="icon-btn"
            :title="t('action.downloadArchive')"
            @click="emit('download-archive', item)"
            v-html="ICONS.archive"
          ></button>
          <button class="icon-btn" :title="t('action.copyPath')" @click="emit('copy', item)" v-html="ICONS.link"></button>
          <button class="icon-btn" :title="t('action.copyCurl')" @click="emit('copy-curl', item)" v-html="ICONS.terminal"></button>
          <button class="icon-btn" :title="t('action.openNew')" @click="emit('open-new', item)" v-html="ICONS.external"></button>
          <button v-if="allowUpload" class="icon-btn" :title="t('action.rename')" @click="emit('rename', item)" v-html="ICONS.rename"></button>
          <button v-if="allowDelete" class="icon-btn danger" :title="t('action.delete')" @click="emit('delete', item)" v-html="ICONS.trash"></button>
        </div>
      </div>
    </div>
  </div>
</template>
