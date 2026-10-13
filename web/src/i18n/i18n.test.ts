import i18n, { initI18n, setLocale, storedLocale } from "@/i18n";
import { toErrorCode } from "@/i18n/errors";
import { namespaces, resources, supportedLocales } from "@/i18n/resources";

describe("i18n", () => {
  afterEach(async () => {
    await initI18n("id");
  });

  it("initialises with id as default and loads every namespace for every locale", () => {
    expect(i18n.isInitialized).toBe(true);
    expect(i18n.language).toBe("id");
    for (const lng of supportedLocales) {
      for (const ns of namespaces) {
        expect(i18n.hasResourceBundle(lng, ns)).toBe(true);
      }
    }
  });

  it("translates namespaced keys in id and en", async () => {
    expect(i18n.t("shell:sidebar.dashboard")).toBe(resources.id.shell.sidebar.dashboard);
    await i18n.changeLanguage("en");
    expect(i18n.t("shell:sidebar.dashboard")).toBe(resources.en.shell.sidebar.dashboard);
    expect(document.documentElement.lang).toBe("en");
  });

  it("remembers the user's choice", async () => {
    expect(storedLocale()).toBeUndefined();
    await setLocale("en");
    expect(storedLocale()).toBe("en");
    expect(i18n.language).toBe("en");
  });

  it("maps unknown API error codes to unknown", () => {
    expect(toErrorCode("rate_limited")).toBe("rate_limited");
    expect(toErrorCode("something_new")).toBe("unknown");
  });
});
