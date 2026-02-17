const RuntimeGlobal = globalThis.__lumilioReactJsxRuntime;

if (!RuntimeGlobal) {
  throw new Error("Lumilio React JSX runtime shim is not initialized");
}

export const Fragment = RuntimeGlobal.Fragment;
export const jsx = RuntimeGlobal.jsx;
export const jsxs = RuntimeGlobal.jsxs;
export const jsxDEV = RuntimeGlobal.jsxDEV;
