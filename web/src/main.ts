import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import { initTheme } from './theme'
import { initLocale } from './i18n'

// Apply the persisted theme + language before mounting so the first paint is
// already correct (no light/dark flash and no wrong-language flash).
initTheme()
initLocale()

createApp(App).mount('#app')
