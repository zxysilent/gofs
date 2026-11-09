<script setup lang="ts">
import type { UploadTask } from '../types'

defineProps<{ tasks: UploadTask[] }>()
</script>

<template>
  <div id="uploads">
    <div v-for="t in tasks" :key="t.id" class="upload-item">
      <div class="upload-info">
        <span class="upload-name" :title="t.name">{{ t.name }}</span>
        <span
          class="upload-status"
          :class="{ done: t.status === 'done', error: t.status === 'error' }"
          :title="t.error"
        >{{ t.status === 'done' ? '✓' : t.status === 'error' ? '✗' : '' }}</span>
      </div>
      <div class="upload-bar">
        <div
          class="upload-bar-fill"
          :class="{ done: t.status === 'done', error: t.status === 'error' }"
          :style="{ width: t.progress + '%' }"
        ></div>
      </div>
      <span class="upload-percent">{{ t.progress }}%</span>
    </div>
  </div>
</template>
