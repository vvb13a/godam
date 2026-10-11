import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  build: {
    // Keep the committed frontend/dist/.gitkeep: the Go binary will embed this
    // directory with go:embed and needs at least one file to compile on a
    // clean checkout. `make build-web` clears stale output instead.
    emptyOutDir: false,
  },
  server: {
    proxy: {
      // The Go backend will own /api; proxying keeps the browser same-origin
      // during development.
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
