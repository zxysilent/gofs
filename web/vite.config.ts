import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The built assets are served by the Go server under a dedicated prefix so
// they never collide with real files. We use a unique placeholder that the
// server replaces at runtime with `uri_prefix + assets_prefix`.
export default defineConfig({
  base: '/__GOFS_ASSETS__/',
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'es2020',
    sourcemap: false,
  },
})
