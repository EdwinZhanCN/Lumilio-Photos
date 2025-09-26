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
  },
  plugins: [react(), tailwindcss(), wasm(), topLevelAwait()],

  test: {
    projects: [
      {
        test: {
          name: "happy-dom",
          environment: "happy-dom",
          setupFiles: ["./setup.happy-dom.ts", "@vitest/web-worker"],
        },
      },
      {
        test: {
          include: ["**/*.worker.{ts,js}", "**/*.test.{ts,js}"],
          name: "node",
          environment: "node",
          setupFiles: ["./setup.node.ts", "@vitest/web-worker"],
        },
      },
    ],
  },
  optimizeDeps: {
    exclude: ["@immich/justified-layout-wasm"],
  },
});
