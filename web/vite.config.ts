import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { docts } from "@edwinzhancn/docts/vite";
import { playwright } from "vite-plus/test/browser/providers/playwright";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const productionSmokeEnabled = process.env.PRODUCTION_SMOKE === "true";
const hashPerformanceEnabled = process.env.VITEST_HASH_PERF === "true";
const testProjects = [
  {
    extends: true,
    test: {
      name: "happy-dom",
      environment: "happy-dom",
      setupFiles: ["./setup.happy-dom.ts"],
      include: ["src/**/*.{test,spec}.{ts,tsx}"],
      exclude: ["src/workers/*", "**/node_modules/**"],
    },
  },
  {
    extends: true,
    test: {
      name: hashPerformanceEnabled ? "hash-performance" : "hash-contract",
      include: [
        hashPerformanceEnabled ? "src/workers/hash.perf.test.ts" : "src/workers/hash.test.ts",
      ],
      exclude: ["**/node_modules/**"],
      testTimeout: 300_000,
      browser: {
        api: {
          host: "127.0.0.1",
        },
        provider: playwright({
          launchOptions: {
            channel: process.env.PLAYWRIGHT_CHANNEL ?? "chrome",
          },
        }),
        instances: [{ browser: "chromium", headless: true }],
        enabled: true,
      },
    },
  },
];

export default defineConfig({
  define: {
    "process.env.DRAGGABLE_DEBUG": "false",
  },

  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    headers: {
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "credentialless",
    },
  },
  preview: {
    headers: {
      "Cross-Origin-Opener-Policy": "same-origin",
      "Cross-Origin-Embedder-Policy": "credentialless",
    },
  },
  plugins: [react(), tailwindcss(), docts({ root: "src" })],

  build: {
    target: "esnext",
    rollupOptions: productionSmokeEnabled
      ? {
          input: {
            app: path.resolve(__dirname, "index.html"),
            "production-smoke": path.resolve(__dirname, "production-smoke.html"),
          },
        }
      : undefined,
  },

  worker: {
    format: "es",
  },

  test: {
    projects: testProjects as any,
  },

  lint: {
    ignorePatterns: [
      "dist/**",
      "coverage/**",
      "src/features/*/doc.md",
      "src/wasm/**",
      "src/lib/http-commons/schema.d.ts",
    ],
    jsPlugins: ["@edwinzhancn/docts/oxlint"],
    rules: {
      "docts/link-needs-import": "error",
    },
    overrides: [
      {
        // doc.ts imports back {@link} references and nothing else — tsc counts
        // a {@link} as a use, the linter's no-unused-vars does not.
        files: ["**/doc.ts"],
        rules: {
          "no-unused-vars": "off",
          "eslint/no-unused-vars": "off",
        },
      },
    ],
    options: {
      typeAware: true,
      typeCheck: true,
    },
  },

  fmt: {
    ignorePatterns: [
      "dist/**",
      "coverage/**",
      "src/features/*/doc.md",
      "src/wasm/**",
      "src/lib/http-commons/schema.d.ts",
    ],
    semi: true,
    singleQuote: false,
  },

  staged: {
    "*.{js,jsx,ts,tsx}": "vp check --fix",
    "*.{css,scss,json,md,yml,yaml}": "vp fmt . --write",
  },

  optimizeDeps: {
    exclude: ["@immich/justified-layout-wasm"],
  },
});
