/// <reference types="vitest" />
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wasm from "vite-plugin-wasm";
import path from "path";
import topLevelAwait from "vite-plugin-top-level-await";
import {preview} from "@vitest/browser-preview";

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
  plugins: [react(), tailwindcss(), wasm(), topLevelAwait()],

  worker: {
    format: 'es',
    plugins: () => [wasm(), topLevelAwait()]
  },

  test: {
    projects: [
      {
        extends: true,
        test: {
          name: "happy-dom",
          environment: "happy-dom",
          setupFiles: ["./setup.happy-dom.ts"],
          exclude: ["src/workers/*"],
        }
      },
      {
        extends: true,
        test: {
          name: "browser",
          include: ["src/workers/*"],
          browser: {
            provider: preview(),
            instances: [
              { browser: 'chrome' },
            ],
            enabled: true,
          },
        }
      },
    ],  },

  optimizeDeps: {
    exclude: ["@immich/justified-layout-wasm"],
  },
});