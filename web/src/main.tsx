import { createRoot } from "react-dom/client";
import App from "./App.tsx";
import { I18nProvider } from "@/lib/i18n.tsx";

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Failed to find the root element");

createRoot(rootElement).render(
  <I18nProvider>
    <App />
  </I18nProvider>,
);
