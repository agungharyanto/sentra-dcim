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
  use: {
    baseURL: process.env.BASE_URL,
    ignoreHTTPSErrors: process.env.BASE_URL?.startsWith("https://localhost") ?? false,
    trace: "off",
    screenshot: "off",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
