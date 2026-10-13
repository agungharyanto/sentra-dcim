import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import { fallbackLocale, isLocale, type Locale, namespaces, resources, supportedLocales } from "./resources";

export { fallbackLocale, isLocale, type Locale, supportedLocales } from "./resources";

const STORAGE_KEY = "sentra.locale";

/** The locale the user picked in this browser, if any; it overrides the server default. */
export function storedLocale(): Locale | undefined {
  try {
    const value = window.localStorage.getItem(STORAGE_KEY);
    return isLocale(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

function syncDocumentLang(lng: string) {
  document.documentElement.lang = lng;
}

export async function initI18n(lng: Locale = fallbackLocale) {
  if (i18n.isInitialized) {
    await i18n.changeLanguage(lng);
    return i18n;
  }
  i18n.on("languageChanged", syncDocumentLang);
  await i18n.use(initReactI18next).init({
    resources,
    lng,
    fallbackLng: fallbackLocale,
    supportedLngs: [...supportedLocales],
    ns: [...namespaces],
    defaultNS: "common",
    interpolation: { escapeValue: false }, // React already escapes
  });
  syncDocumentLang(lng);
  return i18n;
}

/** Switches the UI language and remembers the choice in this browser. */
export async function setLocale(lng: Locale) {
  try {
    window.localStorage.setItem(STORAGE_KEY, lng);
  } catch {
    // Storage may be disabled (private mode); the switch still applies to this tab.
  }
  await i18n.changeLanguage(lng);
}

export default i18n;
