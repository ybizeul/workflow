import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      '/wf': {
        target: 'ws://localhost:8080/',
        ws: true,
        rewriteWsOrigin: true,
        changeOrigin: true,
      },
    },
  },
})
