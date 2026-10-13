import { defineConfig, devices } from "@playwright/test";

// Runs inside mcr.microsoft.com/playwright (see run.sh). BASE_URL and credentials come from the environment.
export default defineConfig({
  testDir: ".",
  testMatch: "*.spec.ts",
  timeout: 30_000,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  outputDir: "/tmp/e2e-results",
  // One API login for the whole run; every spec starts signed in (storageState).
  globalSetup: "./global-setup.ts",
  // Screenshots taken by the specs (helpers.ts artifactsDir) are separate from Playwright failure output.
  use: {
    baseURL: process.env.BASE_URL,
    ignoreHTTPSErrors: process.env.BASE_URL?.startsWith("https://localhost") ?? false,
    trace: "off",
    // Settled colours for axe: the UI honours prefers-reduced-motion (no transitions/animations).
    contextOptions: { reducedMotion: "reduce" },
    screenshot: "off",
    storageState: "/tmp/e2e-state.json",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
