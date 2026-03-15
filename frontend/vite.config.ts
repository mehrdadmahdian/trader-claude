import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

const appPort = process.env.BACKEND_PORT ?? '8080'
const frontendPort = parseInt(process.env.FRONTEND_PORT ?? '5173', 10)

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: frontendPort,
    proxy: {
      '/api': {
        target: `http://backend:${appPort}`,
        changeOrigin: true,
      },
      '/ws': {
        target: `ws://backend:${appPort}`,
        ws: true,
        changeOrigin: true,
      },
    },
  },
})
