/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly API_URL?: string;
  readonly VITE_PLUGIN_REGISTRY_URL?: string;
  readonly VITE_PLUGIN_CDN_ORIGIN?: string;
  readonly VITE_STUDIO_PLUGIN_RUNTIME_ENABLED?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
