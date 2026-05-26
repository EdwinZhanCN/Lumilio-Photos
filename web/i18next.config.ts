import { defineConfig } from "i18next-cli";

export default defineConfig({
  locales: ["en", "zh"],
  extract: {
    input: ["src/**/*.{ts,tsx}"],
    output: "src/locales/{{language}}/{{namespace}}.json",
    ignore: ["src/wasm/**", "**/*.wasm.d.ts", "**/*_wasm_bg.wasm.d.ts"],
  },
});
