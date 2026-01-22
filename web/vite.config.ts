/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wasm from "vite-plugin-wasm";
import path from "path";
import topLevelAwait from "vite-plugin-top-level-await";

// https://vite.dev/config/
export default defineConfig({
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    headers: {
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'credentialless'
    }
  },
  preview: {
    headers: {
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "credentialless",
    },
  },
  worker: {
    plugins: () => [wasm(), topLevelAwait()],
    format: 'es',
  },
  plugins: [react(), tailwindcss(), wasm(), topLevelAwait()],

  test: {
    projects: [
      {
        test: {
          name: "happy-dom",
          environment: "happy-dom",
          setupFiles: ["./setup.happy-dom.ts", "@vitest/web-worker"],
          exclude: ["src/workers/hash.test.ts"],
        },
      },
      {
        test: {
          name: "browser",
          include: ["src/workers/hash.test.ts"],
          browser: {
            enabled: true,
            provider: 'preview', 
            instances: [
              { browser: 'chrome' }
            ],
          },
        },
      },
    ],
  },
  optimizeDeps: {
    exclude: ["@immich/justified-layout-wasm"],
  },
});
