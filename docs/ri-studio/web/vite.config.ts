import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    // Enable SharedArrayBuffer for WASM performance (optional but recommended)
    headers: {
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'require-corp',
    },
  },
  // Optimize WASM loading
  optimizeDeps: {
    exclude: ['wasm.wasm', 'wasm_exec.js'],
  },
  build: {
    // Increase chunk size warning limit for WASM
    chunkSizeWarningLimit: 80000,
    rollupOptions: {
      output: {
        // Ensure WASM files are handled correctly
        assetFileNames: (assetInfo) => {
          if (assetInfo.name === 'wasm.wasm') {
            return 'wasm.wasm';
          }
          if (assetInfo.name === 'wasm_exec.js') {
            return 'wasm_exec.js';
          }
          return 'assets/[name]-[hash][extname]';
        },
      },
    },
  },
})




