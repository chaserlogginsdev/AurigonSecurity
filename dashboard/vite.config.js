import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  build: {
    // Output compiled dashboard directly into backend/dist/
    // so the Go backend can serve it as static files
    outDir: '../backend/dist',
    emptyOutDir: true,
  },
  server: {
    // In dev mode, proxy all API calls to the backend
    // so you don't have to change any URLs in the Svelte code
    proxy: {
      '/login':          'http://localhost:8080',
      '/change-password':'http://localhost:8080',
      '/machines':       'http://localhost:8080',
      '/accounts':       'http://localhost:8080',
      '/actions':        'http://localhost:8080',
      '/audit':          'http://localhost:8080',
      '/users':          'http://localhost:8080',
      '/groups':         'http://localhost:8080',
      '/register':       'http://localhost:8080',
      '/inventory':      'http://localhost:8080',
    }
  }
})