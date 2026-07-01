import { defineConfig } from "vite-plus";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { docts } from "@edwinzhancn/docts/vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

const browserTestsEnabled = process.env.VITEST_BROWSER === "true";
const browserPreview = browserTestsEnabled
  ? (await import("vite-plus/test/browser-preview")).preview
  : undefined;
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
  ...(browserTestsEnabled
    ? [
        {
          extends: true,
          test: {
            name: "browser",
            include: ["src/workers/*"],
            exclude: ["**/node_modules/**"],
            browser: {
              provider: browserPreview?.(),
              instances: [{ browser: "chrome" }],
              enabled: true,
            },
          },
        },
      ]
    : []),
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
      "src/lib/http-commons/openapi-fetch/**",
      "src/lib/http-commons/openapi-react-query/**",
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
      "src/lib/http-commons/openapi-fetch/**",
      "src/lib/http-commons/openapi-react-query/**",
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
