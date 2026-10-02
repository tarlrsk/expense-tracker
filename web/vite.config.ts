/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'

// Matches the default shadcn theme (--background in src/index.css).
const themeColor = '#ffffff'

// The Go API the dev server proxies /api to. API_ADDR is the API's own
// listen address (host:port); the Makefile passes it in from .env.
function apiTarget(): string {
  const addr = process.env.API_ADDR
  if (!addr) {
    return 'http://127.0.0.1:8080'
  }
  // ":8080" (no host) means "all interfaces" to the API; reach it on loopback.
  return addr.startsWith(':') ? `http://127.0.0.1${addr}` : `http://${addr}`
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    // Must come before the React plugin.
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      devOptions: { enabled: false },
      // Icons are already precached by globPatterns below.
      includeManifestIcons: false,
      manifest: {
        name: 'Satang',
        short_name: 'Satang',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        theme_color: themeColor,
        background_color: themeColor,
        icons: [
          { src: 'pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          {
            src: 'maskable-icon-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
        ],
      },
      workbox: {
        // App shell only.
        globPatterns: ['**/*.{html,js,css,svg,png,woff2}'],
        navigateFallback: 'index.html',
        // The service worker must never answer or cache /api requests.
        navigateFallbackDenylist: [/^\/api\//],
        runtimeCaching: [],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: apiTarget() },
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
