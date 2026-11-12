// 浏览器全屏（Fullscreen API）的封装。
// 注意与预览弹窗里的“网页内全屏”区分：那个只是把弹窗铺满视口，地址栏仍在；
// 这里是真的让整个标签页进入浏览器全屏。
import { onBeforeUnmount, onMounted, ref } from 'vue'

// 某些浏览器（Safari 老版本、iOS）用带前缀的 API，统一探测。
type FsDocument = Document & {
  webkitFullscreenElement?: Element | null
  webkitExitFullscreen?: () => Promise<void> | void
}
type FsElement = HTMLElement & {
  webkitRequestFullscreen?: () => Promise<void> | void
}

const doc = document as FsDocument

function currentElement(): Element | null {
  return doc.fullscreenElement ?? doc.webkitFullscreenElement ?? null
}

export function useFullscreen() {
  const isFullscreen = ref(!!currentElement())
  // 浏览器可能拒绝（权限策略 / 非用户手势），把失败原因留给调用方提示。
  const supported = ref(
    typeof doc.documentElement.requestFullscreen === 'function' ||
      typeof (doc.documentElement as FsElement).webkitRequestFullscreen === 'function',
  )

  function sync() {
    isFullscreen.value = !!currentElement()
  }

  async function toggle(): Promise<boolean> {
    if (!supported.value) return false
    try {
      if (currentElement()) {
        if (doc.exitFullscreen) await doc.exitFullscreen()
        else if (doc.webkitExitFullscreen) await doc.webkitExitFullscreen()
      } else {
        const el = document.documentElement as FsElement
        if (el.requestFullscreen) await el.requestFullscreen()
        else if (el.webkitRequestFullscreen) await el.webkitRequestFullscreen()
      }
      return true
    } catch {
      sync()
      return false
    }
  }

  onMounted(() => {
    document.addEventListener('fullscreenchange', sync)
    document.addEventListener('webkitfullscreenchange', sync)
  })

  onBeforeUnmount(() => {
    document.removeEventListener('fullscreenchange', sync)
    document.removeEventListener('webkitfullscreenchange', sync)
  })

  return { isFullscreen, supported, toggle }
}
