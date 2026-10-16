import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  // O app é servido em várias rotas (/ e /auth), enquanto os assets ficam
  // expostos pelo servidor em /react/assets/. Caminhos relativos fariam o
  // navegador solicitar /assets/* e receber o fallback HTML do painel.
  base: '/react/',
  plugins: [react()],
  server: {
    host: true,
    port: 5173,
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
