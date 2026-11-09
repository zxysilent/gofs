<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { absUrl, readText } from '../api'
import { ICONS } from '../icons'
import { copyText, showToast } from '../dialog'

const emit = defineEmits<{ (e: 'back'): void }>()

// 手册内容由后端提供（Markdown），前端只负责展示。
const manualUrl = absUrl('/gofs-curl.md')
const manual = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    manual.value = await readText('/gofs-curl.md')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

async function copyManual() {
  const ok = await copyText(manual.value)
  showToast(ok ? '已复制使用手册' : '复制失败，请手动选择复制', ok ? 'success' : 'error')
}

async function copyUrl() {
  const full = location.origin + manualUrl
  const ok = await copyText(full)
  showToast(ok ? '已复制手册地址: ' + full : '复制失败: ' + full, ok ? 'success' : 'error')
}

function openManual() {
  window.open(location.origin + manualUrl, '_blank', 'noopener')
}
</script>

<template>
  <div class="help-page">
    <div class="help-page-head">
      <button class="btn" title="返回文件列表" @click="emit('back')">
        <span v-html="ICONS.collapse"></span>返回
      </button>
      <span class="help-page-title">curl 使用手册</span>
      <div class="toolbar-spacer"></div>
      <button class="btn" title="在新标签页打开原始 Markdown" @click="openManual">
        <span v-html="ICONS.external"></span>原始文件
      </button>
      <button class="btn" title="复制 Markdown 地址" @click="copyUrl">
        <span v-html="ICONS.link"></span>复制地址
      </button>
      <button class="btn btn-primary" title="复制使用手册内容" @click="copyManual">
        <span v-html="ICONS.copy"></span>复制手册
      </button>
    </div>
    <div class="help-page-body">
      <div v-if="loading" class="file-loading">
        <span v-html="ICONS.spinner"></span><span>加载中...</span>
      </div>
      <div v-else-if="error" class="error-banner">加载失败：{{ error }}</div>
      <pre v-else class="help-md">{{ manual }}</pre>
    </div>
  </div>
</template>
