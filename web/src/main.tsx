import * as React from "react";
import * as ReactJsxRuntime from "react/jsx-runtime";
import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import { I18nProvider } from "@/lib/i18n.tsx";

declare global {
  interface Window {
    __lumilioReact?: typeof React;
    __lumilioReactJsxRuntime?: typeof ReactJsxRuntime;
  }
}

window.__lumilioReact = React;
window.__lumilioReactJsxRuntime = ReactJsxRuntime;

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Failed to find the root element");

createRoot(rootElement).render(
  <I18nProvider>
    <App />
  </I18nProvider>,
);
