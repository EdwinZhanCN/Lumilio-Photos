import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import wasm from "vite-plugin-wasm";
import path from "path";
import topLevelAwait from "vite-plugin-top-level-await";

// https://vite.dev/config/
export default defineConfig({
    //@ts-ignore
    test: {
        environment: "jsdom",
        globals: true,
        setupFiles: "./vitest.setup.ts",
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    plugins: [react(), tailwindcss(), wasm(), topLevelAwait()],
});
