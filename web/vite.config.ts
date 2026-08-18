import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    // Proxy the Go API so the browser sees one origin in development.
    // Without this every request is cross-origin and depends on the backend's
    // CORS_ALLOWED_ORIGINS being right — a needless thing to debug locally.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
