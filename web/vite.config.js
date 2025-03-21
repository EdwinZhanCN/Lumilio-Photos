import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import tailwindcss from '@tailwindcss/vite'
import wasmPack from '@wasm-tool/wasm-pack-plugin'
import wasm from 'vite-plugin-wasm'


// https://vite.dev/config/
export default defineConfig({
  plugins: [
      react(),
      tailwindcss(),
      wasm(),
      wasmPack({
          crateDirectory: './thumbnail-wasm', // Rust 项目路径
          outDir: './src/wasm' // 输出目录
      })
  ],
    optimizeDeps: {
        exclude: ['@wasm-tool/wasm-pack-plugin']
    },
    //set alias of src folder
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src')
        }
    },
})
