import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/graph':     'http://localhost:4319',
      '/anomalies': 'http://localhost:4319',
      '/rootcause': 'http://localhost:4319',
    },
  },
})
