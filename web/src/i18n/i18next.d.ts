import "i18next";

import type { resources } from "./resources";

// Typed keys: t("shell:placeholder.title") is checked against the id (reference) catalog at compile time.
declare module "i18next" {
  interface CustomTypeOptions {
    defaultNS: "common";
    resources: (typeof resources)["id"];
  }
}
