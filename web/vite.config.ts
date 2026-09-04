import { defineConfig, loadEnv, type Plugin } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { docts } from "@edwinzhancn/docts/vite";
import { playwright } from "vite-plus/test/browser/providers/playwright";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Development is a single-origin deployment: the browser only ever talks to the
// Vite dev server, which proxies /api to the Go API. That is what lets `--host`
// actually work — a phone on the LAN reaches one origin instead of needing to
// find a loopback API. `make web-dev` passes API_URL as process-local toolchain
// configuration, so development no longer writes an env file into web/.
const apiProxyTarget =
  process.env.API_URL || loadEnv("development", __dirname, "").API_URL || "http://127.0.0.1:6680";

const entryChunkBudget = 1300 * 1024;
const lazyChunkBudget = 650 * 1024;
const lazyChunkExceptions = [{ prefix: "emacs-lisp-", budget: 800 * 1024 }];

const enforceChunkBudgets: Plugin = {
  name: "enforce-chunk-budgets",
  generateBundle(_options, bundle) {
    for (const output of Object.values(bundle)) {
      if (output.type !== "chunk") continue;

      const exception = lazyChunkExceptions.find(({ prefix }) =>
        output.fileName.startsWith(`assets/${prefix}`),
      );
      const budget = output.isEntry ? entryChunkBudget : (exception?.budget ?? lazyChunkBudget);
      const bytes = new TextEncoder().encode(output.code).byteLength;
      if (bytes > budget) {
        this.error(
          `${output.fileName} is ${(bytes / 1024).toFixed(1)} KiB; ` +
            `its reviewed chunk budget is ${(budget / 1024).toFixed(0)} KiB`,
        );
      }
    }
  },
};

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
      // Vitest browser mode re-optimizes deps mid-run and reloads the page,
      // aborting the dynamic test-file imports that other files have in flight
      // ("Failed to fetch dynamically imported module" / "Vitest failed to find
      // the runner" / "Vite unexpectedly reloaded a test"). Under parallel file
      // execution on CI this races badly enough to wedge the whole run, not
      // just fail a file. Running the browser files serially removes the
      // concurrent in-flight imports the reload was aborting. Keep browser files
      // serial and front-load known dependencies through optimizeDeps.include;
      // do not retry, because a pass after an infrastructure reload is still a
      // nondeterministic required check. See vitest-dev/vitest#8447, #9509.
      fileParallelism: false,
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
      // Same browser-mode reload constraint as the integration project (see there).
      fileParallelism: false,
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
      "node:fs/promises": path.resolve(__dirname, "./src/shims/nodeFsPromises.browser.ts"),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // changeOrigin stays false (the Vite default) on purpose. The API
      // compares the browser's Origin header against the target origin it
      // derives from Host, so rewriting Host to the proxy target would make
      // every session request look cross-origin and get a 403 from
      // trustedSessionOriginMiddleware. Passing Host through also means the
      // server sees the address the browser actually used, which is how a LAN
      // device gets correct passkey-availability answers instead of a
      // loopback-flavoured one.
      "/api": { target: apiProxyTarget, ws: true },
    },
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
  plugins: [react(), tailwindcss(), docts({ root: "src" }), enforceChunkBudgets],

  build: {
    target: "esnext",
    // Generic Vite warnings use the reviewed entry ceiling; enforceChunkBudgets
    // applies tighter limits to every lazy chunk and an explicit syntax-chunk
    // exception, so raising this threshold does not hide future regressions.
    chunkSizeWarningLimit: 1300,
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
    "*.{css,scss,json,md,yml,yaml}": "vp fmt --write",
  },

  optimizeDeps: {
    exclude: ["@immich/justified-layout-wasm"],
    // Pre-bundle the deps the browser (integration) and jsdom (unit) test
    // projects optimize, so Vite gets most dep optimization done in its initial
    // scan instead of discovering deps mid-run. A mid-run re-optimize reloads
    // the page and aborts in-flight dynamic test-file imports (flaky "Failed to
    // fetch dynamically imported module" / "Vitest failed to find the runner"
    // in CI, see vitest-dev/vitest#8447, #9509). Browser files remain serial so
    // an unexpected reload is reported directly rather than hidden by a retry.
    // List captured from
    // node_modules/.vite/vitest/*/deps/_metadata.json after a full `vp test`
    // run. Keep wasm packages OUT of this list and in `exclude` above —
    // pre-bundling wasm wedges the optimizer on CI (same reason
    // @immich/justified-layout-wasm is excluded) and hangs `vp test` until the
    // job timeout.
    include: [
      "@microsoft/fetch-event-source",
      "@tanstack/react-query",
      "@vidstack/react",
      "@vidstack/react/player/layouts/default",
      "i18next",
      "i18next-browser-languagedetector",
      "immer",
      "leaflet",
      "lucide-react",
      "openapi-fetch",
      "openapi-react-query",
      "qrcode",
      "react",
      "react-dom",
      "react-dom/client",
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
