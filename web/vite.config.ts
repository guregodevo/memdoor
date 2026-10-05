import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

const gatewayPort = process.env.VITE_GATEWAY_PORT || '18789'
const gateway = `http://localhost:${gatewayPort}`
const gatewayWs = `ws://localhost:${gatewayPort}`

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      // Remote control's relay is a websocket under /api.
      '/api/relay': { target: gatewayWs, ws: true },
      '/api': gateway,
      '/chat/ws': { target: gatewayWs, ws: true },
      '/ws': { target: gatewayWs, ws: true },
    },
  },
})
