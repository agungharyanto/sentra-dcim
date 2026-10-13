import "@testing-library/jest-dom/vitest";

import { cleanup } from "@testing-library/react";
import { afterEach, beforeAll } from "vitest";

import { initI18n } from "@/i18n";

// jsdom lacks ResizeObserver (used by Radix popper: Tooltip, Select, DropdownMenu).
if (!("ResizeObserver" in globalThis)) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

beforeAll(async () => {
  await initI18n("id");
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});
