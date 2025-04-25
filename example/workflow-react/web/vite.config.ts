import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/go': {
        target: 'http://localhost:8080/',
        rewrite: (path) => path.replace(/^\/go/, ''),
        changeOrigin: true,
      },
      '/go/wf': {
        target: 'ws://localhost:8080/',
        rewrite: (path) => path.replace(/^\/go/, ''),
        ws: true,
        rewriteWsOrigin: true,
        changeOrigin: true,
      },
    },
  },
})
