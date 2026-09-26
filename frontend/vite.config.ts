import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'

// In dev, forward API and presigned-storage requests to the Nginx entrypoint
// so the app is same-origin exactly like in production.
const backend = process.env.PIXELCLOUD_URL ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    proxy: {
      '/api': { target: backend, changeOrigin: false },
      '/pixelcloud': { target: backend, changeOrigin: false },
    },
  },
})
