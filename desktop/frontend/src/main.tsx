import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";
import { initI18n } from "./lib/i18n";
import { App } from "./app/App";

// The snapshot arrives asynchronously; applyLocale switches the language once
// the persisted host preference is known.
initI18n("en");

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
