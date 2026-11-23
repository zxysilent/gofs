<script setup lang="ts">
import type { UploadTask } from '../types'

defineProps<{ tasks: UploadTask[] }>()
</script>

<template>
  <!-- 必须挂 .upload-manager（position: fixed）：否则这个节点会成为 .app
       （display: flex）的一个子项，在横向布局里占掉一列宽度，把文件列表挤窄。
       以前这里只有 id="uploads"，而样式写在 .upload-manager 上，
       两者对不上，面板就一直以普通流内元素参与布局。 -->
  <div v-if="tasks.length" class="upload-manager">
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
