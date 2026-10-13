import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import type { UiBootstrap } from "@/features/bootstrap/api";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";

const bootstrap = (overrides: Partial<UiBootstrap> = {}): UiBootstrap => ({
  app_name: "Sentra DCIM",
  version: "v0.2.1",
  default_locale: "id",
  supported_locales: ["id", "en"],
  timezone: "Asia/Jakarta",
  menus: [
    { key: "dashboard", path: "/", icon: "layout-dashboard", status: "available", order: 10 },
    { key: "alarms", path: "/alarms", icon: "siren", status: "coming_soon", order: 50 },
  ],
  ...overrides,
});

function mockFetch(body: unknown, status = 200) {
  const fn = vi.fn(async () => new Response(JSON.stringify(body), { status }));
  vi.stubGlobal("fetch", fn);
  return fn;
}

function renderApp() {
  return render(<App queryClient={createQueryClient()} />);
}

describe("placeholder page", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders in Indonesian and switches to English", async () => {
    const fetch = mockFetch(bootstrap());
    renderApp();

    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(resources.id.shell.placeholder.title);
    expect(fetch).toHaveBeenCalledWith("/api/v1/ui/bootstrap", expect.anything());
    expect(screen.getByTestId("version")).toHaveTextContent("v0.2.1");
    expect(screen.getByTestId("timezone")).toHaveTextContent("Asia/Jakarta");
    expect(screen.getByText(resources.id.shell.sidebar.alarms)).toBeInTheDocument();
    expect(screen.getByText(resources.id.shell.menuStatus.coming_soon)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /English/ }));

    expect(await screen.findByRole("heading", { level: 1 })).toHaveTextContent(resources.en.shell.placeholder.title);
    expect(screen.getByText(resources.en.shell.sidebar.alarms)).toBeInTheDocument();
    expect(screen.getByTestId("locale")).toHaveTextContent(resources.en.common.language.en);
    expect(document.documentElement.lang).toBe("en");
  });

  it("applies the server default locale when the user has not chosen one", async () => {
    mockFetch(bootstrap({ default_locale: "en" }));
    renderApp();
    await waitFor(() =>
      expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(resources.en.shell.placeholder.title),
    );
  });

  it("shows a translated error when bootstrap fails", async () => {
    mockFetch({ error: { code: "rate_limited", message: "x" } }, 429);
    renderApp();
    expect(await screen.findByRole("alert", {}, { timeout: 5000 })).toHaveTextContent(
      resources.id.errors.code.rate_limited,
    );
  });
});
