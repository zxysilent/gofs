<script setup lang="ts">
// 通用浮动右键菜单：按视口边界自适应定位，支持键盘与点击外部关闭。
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ICONS } from '../utils/icons'

export interface MenuItem {
  key: string
  label: string
  /** ICONS 里的图标名，缺省则不显示图标。 */
  icon?: string
  danger?: boolean
  disabled?: boolean
  /** 在该项之前画一条分隔线。 */
  dividerBefore?: boolean
}

const props = defineProps<{ x: number; y: number; items: MenuItem[] }>()
const emit = defineEmits<{
  (e: 'select', key: string): void
  (e: 'close'): void
}>()

const rootRef = ref<HTMLElement | null>(null)
const pos = ref({ left: props.x, top: props.y })
// 键盘高亮项；-1 表示仅鼠标操作。
const activeIndex = ref(-1)

const enabled = computed(() => props.items.filter((i) => !i.disabled))

async function place() {
  await nextTick()
  const el = rootRef.value
  if (!el) return
  const { width, height } = el.getBoundingClientRect()
  const pad = 8
  let left = props.x
  let top = props.y
  // 靠近右/下边缘时向内翻转，避免菜单被裁切
  if (left + width + pad > window.innerWidth) left = Math.max(pad, props.x - width)
  if (top + height + pad > window.innerHeight) top = Math.max(pad, props.y - height)
  pos.value = { left, top }
  el.focus()
}

function move(step: number) {
  const list = enabled.value
  if (!list.length) return
  const cur = activeIndex.value
  const idx = cur < 0 ? (step > 0 ? 0 : list.length - 1) : (cur + step + list.length) % list.length
  activeIndex.value = idx
}

function choose(item: MenuItem) {
  if (item.disabled) return
  emit('select', item.key)
}

function onKeydown(ev: KeyboardEvent) {
  switch (ev.key) {
    case 'ArrowDown':
      ev.preventDefault()
      move(1)
      break
    case 'ArrowUp':
      ev.preventDefault()
      move(-1)
      break
    case 'Enter':
    case ' ': {
      ev.preventDefault()
      const item = enabled.value[activeIndex.value]
      if (item) choose(item)
      break
    }
    case 'Escape':
      ev.preventDefault()
      emit('close')
      break
    case 'Tab':
      ev.preventDefault()
      emit('close')
      break
  }
}

// 点击菜单外或按 Esc 关闭；捕获阶段处理，避免被行内 @click 抢先。
function onDocPointerDown(ev: PointerEvent) {
  if (!rootRef.value?.contains(ev.target as Node)) emit('close')
}

function onDocKeydown(ev: KeyboardEvent) {
  if (ev.key === 'Escape') emit('close')
}

// 滚动/缩放时菜单会脱离锚点，直接关闭。
function onViewportChange() {
  emit('close')
}

onMounted(() => {
  void place()
  document.addEventListener('pointerdown', onDocPointerDown, true)
  document.addEventListener('keydown', onDocKeydown)
  window.addEventListener('resize', onViewportChange)
  window.addEventListener('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointerDown, true)
  document.removeEventListener('keydown', onDocKeydown)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})

watch(() => [props.x, props.y, props.items], () => void place())
</script>

<template>
  <div
    ref="rootRef"
    class="ctx-menu"
    role="menu"
    tabindex="-1"
    :style="{ left: pos.left + 'px', top: pos.top + 'px' }"
    @keydown="onKeydown"
    @contextmenu.prevent
  >
    <template v-for="(item, i) in items" :key="item.key">
      <div v-if="item.dividerBefore && i > 0" class="ctx-menu-sep"></div>
      <button
        class="ctx-menu-item"
        :class="{
          danger: item.danger,
          disabled: item.disabled,
          active: enabled[activeIndex]?.key === item.key,
        }"
        role="menuitem"
        :disabled="item.disabled"
        @mouseenter="activeIndex = enabled.findIndex((e) => e.key === item.key)"
        @click="choose(item)"
      >
        <span class="ctx-menu-icon" v-html="item.icon ? ICONS[item.icon] : ''"></span>
        <span class="ctx-menu-label">{{ item.label }}</span>
      </button>
    </template>
  </div>
</template>
