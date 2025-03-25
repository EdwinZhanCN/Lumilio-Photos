import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import wasm from 'vite-plugin-wasm'
import topLevelAwait from 'vite-plugin-top-level-await'
import wasmPack from 'vite-plugin-wasm-pack'
import tailwindcss from '@tailwindcss/vite'


export default defineConfig({
    plugins: [
        react(),
        tailwindcss(),
        wasm(),
        topLevelAwait(),
        wasmPack(['./thumbnail-wasm']) // Path to your Rust project
    ],
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
            '@wasm': path.resolve(__dirname, './thumbnail-wasm/pkg')
        }
    },
    build: {
        target: 'esnext', // Ensure proper WASM support
    }
})