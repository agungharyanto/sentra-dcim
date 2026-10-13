import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach, beforeAll } from "vitest";

import { initI18n } from "@/i18n";

beforeAll(async () => {
  await initI18n("id");
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});
