import "@/styles/globals.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/app/App";
import { fallbackLocale, initI18n, storedLocale } from "@/i18n";

const root = document.getElementById("root");
if (!root) throw new Error("#root element missing in index.html");

// Start in the browser's saved choice (or id); BootstrapGate applies the server default once it is known.
await initI18n(storedLocale() ?? fallbackLocale);

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
