import "@testing-library/jest-dom/vitest";

import { cleanup, configure } from "@testing-library/react";
import { afterEach, beforeAll } from "vitest";

import { initI18n } from "@/i18n";

// CI runners are slower than the server: full-app renders (bootstrap + /me + module) can exceed the 1 s default.
configure({ asyncUtilTimeout: 3000 });

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
