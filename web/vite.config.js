import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import wasm from 'vite-plugin-wasm'
import topLevelAwait from 'vite-plugin-top-level-await'
import tailwindcss from '@tailwindcss/vite'


export default defineConfig({
    plugins: [
        react(),
        tailwindcss(),
        wasm(),
        topLevelAwait(),
    ],
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        }
    },
    build: {
        target: 'esnext',
    }
})