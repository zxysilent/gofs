<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { absUrl, readText } from '../api'
import { ICONS } from '../icons'
import { copyText, showToast } from '../dialog'
import { t } from '../i18n'
import { PROJECT_URL } from '../utils'

const emit = defineEmits<{ (e: 'back'): void }>()

// 手册内容由后端提供（Markdown），前端只负责展示。
const AGENT_DOC = '/gofs-agent.md'
const manualUrl = absUrl(AGENT_DOC)
const manual = ref('')
const loading = ref(true)
const error = ref('')

onMounted(async () => {
  try {
    manual.value = await readText(AGENT_DOC)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})

async function copyManual() {
  const ok = await copyText(manual.value)
  showToast(ok ? t('toast.copiedManual') : t('toast.copyFailedManual'), ok ? 'success' : 'error')
}

async function copyUrl() {
  const full = location.origin + manualUrl
  const ok = await copyText(full)
  showToast(
    ok ? t('sidebar.copiedUrl', { url: full }) : t('sidebar.copyUrlFailed', { url: full }),
    ok ? 'success' : 'error',
  )
}

function openManual() {
  window.open(location.origin + manualUrl, '_blank', 'noopener')
}

// 跳到项目主页。
function openProject() {
  window.open(PROJECT_URL, '_blank', 'noopener')
}
</script>

<template>
  <div class="help-page">
    <div class="help-page-head">
      <button class="btn" :title="t('help.backTitle')" @click="emit('back')">
        <span v-html="ICONS.collapse"></span>{{ t('help.back') }}
      </button>
      <span class="help-page-title">{{ t('help.title') }}</span>
      <div class="toolbar-spacer"></div>
      <button class="btn" :title="t('sidebar.rawFileTitle')" @click="openManual">
        <span v-html="ICONS.external"></span>{{ t('sidebar.rawFile') }}
      </button>
      <button class="btn" :title="t('sidebar.copyUrlTitle')" @click="copyUrl">
        <span v-html="ICONS.link"></span>{{ t('sidebar.copyUrl') }}
      </button>
      <button class="btn" :title="t('sidebar.projectUrlTitle')" @click="openProject">
        <span v-html="ICONS.globe"></span>{{ t('sidebar.projectUrl') }}
      </button>
      <button class="btn btn-primary" :title="t('help.copyManualTitle')" @click="copyManual">
        <span v-html="ICONS.copy"></span>{{ t('help.copyManual') }}
      </button>
    </div>
    <div class="help-page-body">
      <div v-if="loading" class="file-loading">
        <span v-html="ICONS.spinner"></span><span>{{ t('common.loading') }}</span>
      </div>
      <div v-else-if="error" class="error-banner">{{ t('common.loadFailed', { msg: error }) }}</div>
      <pre v-else class="help-md">{{ manual }}</pre>
    </div>
  </div>
</template>
