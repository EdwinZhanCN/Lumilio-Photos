import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { docts } from "@edwinzhancn/docts/vite";
import { playwright } from "vite-plus/test/browser/providers/playwright";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const hashPerformanceEnabled = process.env.VITEST_HASH_PERF === "true";
// Headless Chromium falls back to SwiftShader, whose WebGL is disabled on Apple
// Silicon — so the Studio GPU capability tests (develop engine, render worker)
// can only get a real WebGL2 context in a headed browser. STUDIO_GPU=true runs
// the browser project headed so a developer can verify them locally; the default
// stays headless so CI never fails to launch (those tests skip themselves when
// WebGL2 is absent). See vite-plus `hash-performance` for the same env-gated shape.
const studioGpuEnabled = process.env.STUDIO_GPU === "true";
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
    // Components (*.test.tsx) and colocated flow integration (*.spec.tsx) run in
    // real Chromium via vitest-browser-react, so CSS, browser APIs and event
    // handling are the real thing rather than a simulated-DOM approximation. MSW
    // at the HTTP boundary is wired in test/setup.integration.ts.
    extends: true,
    test: {
      name: "integration",
      setupFiles: ["./test/setup.integration.ts"],
      include: ["src/**/*.test.tsx", "src/**/*.spec.tsx"],
      exclude: ["src/**/*.browser.test.ts", "**/node_modules/**"],
      browser: {
        enabled: true,
        provider: playwright(),
        instances: [{ browser: "chromium", headless: true }],
      },
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
        // Headed (STUDIO_GPU=true) uses the machine's real GPU, the only way the
        // Studio WebGL2 capability tests get a context on Apple Silicon. Headless
        // (default) tries ANGLE+SwiftShader and otherwise lets those tests skip —
        // keeping CI launchable where no display or GPU exists.
        instances: [
          {
            browser: "chromium",
            headless: !studioGpuEnabled,
            launchOptions: studioGpuEnabled
              ? undefined
              : {
                  args: [
                    "--use-gl=angle",
                    "--use-angle=swiftshader",
                    "--enable-unsafe-swiftshader",
                  ],
                },
          },
        ],
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
      "@test": path.resolve(__dirname, "./test"),
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
      "public/mockServiceWorker.js",
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
      "public/mockServiceWorker.js",
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
    // Pre-bundle every dependency the browser (integration) and jsdom (unit)
    // test projects end up optimizing, so Vite finishes dep optimization during
    // its initial scan instead of discovering a dep mid-run. A mid-run
    // re-optimize triggers a full page reload that aborts in-flight dynamic
    // test-file imports, which surfaces as flaky "Failed to fetch dynamically
    // imported module" / "Vitest failed to find the runner" failures in CI (see
    // vitest-dev/vitest#8447, #9509). This list is the union of the specifiers
    // captured from node_modules/.vite/vitest/*/deps/_metadata.json after a full
    // `vp test` run; refresh it if the optimizer starts reloading again. Keep
    // wasm packages OUT of this list and in `exclude` above — pre-bundling wasm
    // wedges the optimizer on CI (same reason @immich/justified-layout-wasm is
    // excluded), which hangs `vp test` until the job timeout.
    include: [
      "@microsoft/fetch-event-source",
      "@tanstack/react-query",
      "@vidstack/react",
      "@vidstack/react/player/layouts/default",
      "expect-type",
      "i18next",
      "i18next-browser-languagedetector",
      "immer",
      "leaflet",
      "lucide-react",
      "openapi-fetch",
      "openapi-react-query",
      "react",
      "react-dom",
      "react-error-boundary",
      "react-i18next",
      "react-leaflet",
      "react-router-dom",
      "react/jsx-dev-runtime",
      "react/jsx-runtime",
      "sonner",
      "supercluster",
      "swiper/modules",
      "swiper/react",
      "vite-plus/test",
      "vitest-browser-react",
      "zustand",
      "zustand/middleware",
      "zustand/middleware/immer",
      "zustand/vanilla",
    ],
  },
});
