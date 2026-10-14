import "@/styles/globals.css";

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@/app/App";
import { fallbackLocale, initI18n, storedLocale } from "@/i18n";
import { applyCspNonce } from "@/lib/csp-nonce";
import { applyTheme, readThemePreference, resolveTheme } from "@/lib/theme";

const root = document.getElementById("root");
if (!root) throw new Error("#root element missing in index.html");

// Before any Radix overlay can inject its scroll-lock <style> (CSP style-src nonce, ISS-030).
applyCspNonce();

// Apply the stored theme before the first render (index.html ships class="dark", the default).
applyTheme(resolveTheme(readThemePreference()));

// Start in the browser's saved choice (or id); BootstrapGate applies the server default once it is known.
await initI18n(storedLocale() ?? fallbackLocale);

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
