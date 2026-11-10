import { createApp } from 'vue'
import App from './App.vue'
import './styles/index.css'
import { initTheme } from './utils/theme'
import { initLocale } from './locales'

// Apply the persisted theme + language before mounting so the first paint is
// already correct (no light/dark flash and no wrong-language flash).
initTheme()
initLocale()

createApp(App).mount('#app')
