<script setup lang="ts">
import type { PathItem } from '../types'
import { isDir } from '../types'
import { fileKind, formatSize, formatDirSize, formatTime } from '../utils'
import { ICONS } from '../icons'

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
  if (isDir(item)) return '文件夹'
  switch (fileKind(item.name)) {
    case 'image': return '图片'
    case 'video': return '视频'
    case 'audio': return '音频'
    case 'pdf': return 'PDF'
    case 'markdown': return 'Markdown'
    case 'json': return 'JSON'
    case 'text': return '文本'
    case 'code': return '代码'
    case 'archive': return '压缩包'
    default: return '文件'
  }
}
</script>

<template>
  <div>
    <div v-if="loading" class="file-loading">
      <span v-html="ICONS.spinner"></span><span>加载中...</span>
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
        <div class="col-name">名称</div>
        <div class="col-type">类型</div>
        <div class="col-size">大小</div>
        <div class="col-mtime">修改时间</div>
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
            title="打包下载"
            @click="emit('download-archive', item)"
            v-html="ICONS.archive"
          ></button>
          <button class="icon-btn" title="复制完整路径" @click="emit('copy', item)" v-html="ICONS.link"></button>
          <button class="icon-btn" title="复制 curl 下载命令" @click="emit('copy-curl', item)" v-html="ICONS.terminal"></button>
          <button class="icon-btn" title="新窗口打开" @click="emit('open-new', item)" v-html="ICONS.external"></button>
          <button v-if="allowUpload" class="icon-btn" title="重命名" @click="emit('rename', item)" v-html="ICONS.rename"></button>
          <button v-if="allowDelete" class="icon-btn danger" title="删除" @click="emit('delete', item)" v-html="ICONS.trash"></button>
        </div>
      </div>
    </div>
  </div>
</template>
