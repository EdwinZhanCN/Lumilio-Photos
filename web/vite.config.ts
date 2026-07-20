import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { docts } from "@edwinzhancn/docts/vite";
import { playwright } from "vite-plus/test/browser/providers/playwright";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const hashPerformanceEnabled = process.env.VITEST_HASH_PERF === "true";
const testProjects = [
  {
    // Pure logic in Node: no DOM, window, storage, Testing Library, Router or
    // QueryClient. The missing browser globals are the point — they turn an
    // accidental browser dependency into a failure instead of hiding it.
    extends: true,
    test: {
      name: "unit",
      environment: "node",
      include: ["src/**/*.test.ts"],
      exclude: ["src/**/*.browser.test.ts", "src/workers/**", "**/node_modules/**"],
    },
  },
  {
    // Components (*.test.tsx) and colocated flow integration (*.spec.tsx) share
    // happy-dom, Testing Library and the same setup.
    extends: true,
    test: {
      name: "integration",
      environment: "happy-dom",
      setupFiles: ["./test/setup.integration.ts"],
      include: ["src/**/*.test.tsx", "src/**/*.spec.tsx"],
      exclude: ["src/**/*.browser.test.ts", "**/node_modules/**"],
    },
  },
  {
    extends: true,
    test: {
      name: hashPerformanceEnabled ? "hash-performance" : "browser",
      include: hashPerformanceEnabled
        ? ["src/workers/hash.perf.test.ts"]
        : ["src/**/*.browser.test.ts"],
      exclude: ["**/node_modules/**"],
      testTimeout: 300_000,
      browser: {
        api: {
          host: "127.0.0.1",
        },
        provider: playwright(),
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
