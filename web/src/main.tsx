import { createRoot } from "react-dom/client";
import App from "@/app/App";
import { I18nProvider } from "@/lib/i18n.tsx";
import { installRandomUUIDCompatibility } from "@/lib/uuid";
import RootErrorBoundary from "@/app/errors/RootErrorBoundary";

installRandomUUIDCompatibility();

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Failed to find the root element");

createRoot(rootElement).render(
  <I18nProvider>
    <RootErrorBoundary>
      <App />
    </RootErrorBoundary>
  </I18nProvider>,
);
