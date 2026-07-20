import { defineConfig } from "vite-plus";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";
import { mockPanelApi } from "./dev/mock-api.ts";

// In dev the panel talks to the Wails asset server when LUMILIO_PANEL_API is
// set; without it a small in-memory mock serves the /__onb API so the UI can be
// developed and reviewed without the desktop app running.
const apiTarget = process.env.LUMILIO_PANEL_API;

export default defineConfig({
  plugins: [svelte(), tailwindcss(), ...(apiTarget ? [] : [mockPanelApi()])],
  server: apiTarget
    ? { proxy: { "/__onb": { target: apiTarget, changeOrigin: true } } }
    : undefined,
  staged: {
    "*": "vp check --fix",
  },
  fmt: {},
  lint: {
    jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
    rules: { "vite-plus/prefer-vite-plus-imports": "error" },
    options: { typeAware: true, typeCheck: true },
  },
});
