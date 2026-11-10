<script setup lang="ts">
import { ref, watch } from 'vue'
import type { SortKey, SortOrder } from '../types'
import { ICONS } from '../icons'
import { t } from '../i18n'

const props = defineProps<{
  breadcrumbs: { name: string; path: string }[]
  allowUpload: boolean
  allowArchive: boolean
  allowSearch: boolean
  viewMode: 'list' | 'grid'
  search: string
  sort: SortKey
  order: SortOrder
}>()

const emit = defineEmits<{
  (e: 'toggle-sidebar'): void
  (e: 'search', q: string): void
  (e: 'sort-change', k: SortKey, o: SortOrder): void
  (e: 'view-change', v: 'list' | 'grid'): void
  (e: 'upload', files: FileList | null): void
  (e: 'new-folder'): void
  (e: 'new-file'): void
  (e: 'curl-upload'): void
  (e: 'download-archive'): void
  (e: 'refresh'): void
  (e: 'navigate', path: string): void
}>()

const searchInput = ref(props.search)
watch(() => props.search, (v) => { searchInput.value = v })

function onSearchKeyup(e: KeyboardEvent) {
  if (e.key === 'Enter') emit('search', searchInput.value.trim())
  if (e.key === 'Escape') { searchInput.value = ''; emit('search', '') }
}
function clearSearch() {
  searchInput.value = ''
  emit('search', '')
}

function onSort(key: SortKey) {
  if (props.sort === key) {
    emit('sort-change', key, props.order === 'asc' ? 'desc' : 'asc')
  } else {
    emit('sort-change', key, 'asc')
  }
}

const fileInput = ref<HTMLInputElement | null>(null)
function triggerUpload() {
  fileInput.value?.click()
}
function onFileChange(e: Event) {
  const t = e.target as HTMLInputElement
  emit('upload', t.files)
  t.value = ''
}
</script>

<template>
  <div>
    <div class="toolbar-row">
      <button class="icon-btn" :title="t('toolbar.toggleSidebar')" @click="emit('toggle-sidebar')" v-html="ICONS.menu"></button>
      <nav class="breadcrumb">
        <template v-for="(c, i) in breadcrumbs" :key="c.path">
          <span v-if="i > 0" class="crumb-sep">/</span>
          <a
            class="crumb"
            :class="{ current: i === breadcrumbs.length - 1 }"
            :title="c.name"
            @click="emit('navigate', c.path)"
          >{{ c.name }}</a>
        </template>
      </nav>
      <div class="toolbar-spacer"></div>
      <div v-if="allowSearch" class="search-box">
        <span v-html="ICONS.search"></span>
        <input
          v-model="searchInput"
          type="text"
          :placeholder="t('toolbar.search')"
          @keyup="onSearchKeyup"
        />
        <button v-show="searchInput" class="search-clear" @click="clearSearch">×</button>
      </div>
      <button class="icon-btn" :title="t('common.refresh')" @click="emit('refresh')" v-html="ICONS.refresh"></button>
      <div class="view-toggle">
        <button class="icon-btn" :class="{ active: viewMode === 'list' }" :title="t('toolbar.listView')" @click="emit('view-change', 'list')" v-html="ICONS.list"></button>
        <button class="icon-btn" :class="{ active: viewMode === 'grid' }" :title="t('toolbar.gridView')" @click="emit('view-change', 'grid')" v-html="ICONS.grid"></button>
      </div>
      <button v-if="allowUpload" class="btn btn-primary" @click="triggerUpload">
        <span v-html="ICONS.upload"></span>{{ t('toolbar.upload') }}
      </button>
      <input ref="fileInput" type="file" multiple style="display:none" @change="onFileChange" />
    </div>
    <div class="toolbar-row toolbar-secondary">
      <div class="sort-group">
        <span class="sort-label">{{ t('toolbar.sort') }}</span>
        <button class="chip" :class="{ active: sort === 'name' }" @click="onSort('name')">{{ t('toolbar.sortName') }}<span>{{ sort === 'name' ? (order === 'asc' ? '↑' : '↓') : '' }}</span></button>
        <button class="chip" :class="{ active: sort === 'mtime' }" @click="onSort('mtime')">{{ t('toolbar.sortMtime') }}<span>{{ sort === 'mtime' ? (order === 'asc' ? '↑' : '↓') : '' }}</span></button>
        <button class="chip" :class="{ active: sort === 'size' }" @click="onSort('size')">{{ t('toolbar.sortSize') }}<span>{{ sort === 'size' ? (order === 'asc' ? '↑' : '↓') : '' }}</span></button>
      </div>
      <div class="toolbar-spacer"></div>
      <template v-if="allowUpload">
        <button class="btn" @click="emit('new-folder')"><span v-html="ICONS.newFolder"></span>{{ t('toolbar.newFolder') }}</button>
        <button class="btn" @click="emit('new-file')"><span v-html="ICONS.newFile"></span>{{ t('toolbar.newFile') }}</button>
      </template>
      <button class="btn" :title="t('toolbar.curlUploadTitle')" @click="emit('curl-upload')">
        <span v-html="ICONS.terminal"></span>{{ t('toolbar.curlUpload') }}
      </button>
      <button v-if="allowArchive" class="btn" @click="emit('download-archive')">
        <span v-html="ICONS.archive"></span>{{ t('toolbar.archive') }}
      </button>
    </div>
  </div>
</template>
