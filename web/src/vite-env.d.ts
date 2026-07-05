/// <reference types="vite-plus/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly API_URL?: string;
  readonly VITE_APP_VERSION?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
