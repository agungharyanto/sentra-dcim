import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { helpIndex, mockApi, testBootstrap } from "@/test/bootstrap";

function pageFor(url: string) {
  const u = new URL(url, "http://x");
  const locale = u.searchParams.get("locale") ?? "id";
  const slug = decodeURIComponent(u.pathname.replace("/api/v1/help/pages/", ""));
  if (slug === "nope") return new Response(JSON.stringify({ error: { code: "not_found", message: "x" } }), { status: 404 });
  return { slug, locale, title: slug, audience: "user", order: 1, headings: [], markdown: `# ${slug} (${locale})\n\n[Dasbor](/help/dashboard)` };
}

function renderAt(path: string) {
  const fetch = mockApi({
    "/api/v1/ui/bootstrap": testBootstrap(),
    "/api/v1/help/index": (url: string) => helpIndex(new URL(url, "http://x").searchParams.get("locale") ?? "id"),
    "/api/v1/help/pages/": pageFor,
  });
  window.history.pushState({}, "", path);
  render(<App queryClient={createQueryClient()} />);
  return fetch;
}

const help = resources.id.help;

describe("Help Center", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("shows the page list by audience and the home page at /help", async () => {
    renderAt("/help");
    const nav = await screen.findByRole("navigation", { name: help.pagesNav });
    expect(await within(nav).findByRole("group", { name: help.audience.user })).toBeInTheDocument();
    expect(within(nav).getByRole("group", { name: help.audience.admin })).toHaveTextContent(help.api.title);
    expect(await screen.findByRole("heading", { name: "help (id)" })).toBeInTheDocument();
  });

  it("navigates pages via internal links and the list", async () => {
    renderAt("/help/getting-started");
    await screen.findByRole("heading", { name: "getting-started (id)" });
    await userEvent.click(within(screen.getByTestId("help-markdown")).getByRole("link", { name: "Dasbor" }));
    expect(await screen.findByRole("heading", { name: "dashboard (id)" })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/help/dashboard");
  });

  it("filters the list with search (title + headings)", async () => {
    renderAt("/help");
    const search = await screen.findByTestId("help-search");
    await screen.findByRole("group", { name: help.audience.user });
    await userEvent.type(search, "tema");
    const results = screen.getByTestId("help-results");
    expect(within(results).getAllByRole("link")).toHaveLength(1);
    expect(results).toHaveTextContent("Bahasa, tema, dan jam");
    await userEvent.clear(search);
    await userEvent.type(search, "genset");
    expect(screen.getByTestId("help-results")).toHaveTextContent(help.noResults);
  });

  it("reloads the same page in the other language", async () => {
    const fetch = renderAt("/help/dashboard");
    await screen.findByRole("heading", { name: "dashboard (id)" });
    await userEvent.click(screen.getByRole("button", { name: /English/ }));
    expect(await screen.findByRole("heading", { name: "dashboard (en)" })).toBeInTheDocument();
    expect(fetch.mock.calls.map(([u]) => String(u))).toContain("/api/v1/help/pages/dashboard?locale=en");
  });

  it("shows a not-found state for an unknown help page", async () => {
    renderAt("/help/nope");
    expect(await screen.findByTestId("help-not-found")).toHaveTextContent(help.notFoundTitle);
  });

  it("header ? button opens the help page of the active menu, else the Help Center", async () => {
    renderAt("/dashboard");
    await waitFor(() => expect(screen.getByTestId("help-button")).toHaveAttribute("href", "/help/dashboard"));
    await userEvent.click(screen.getByRole("link", { name: new RegExp(resources.id.shell.sidebar.alarms) }));
    await waitFor(() => expect(screen.getByTestId("help-button")).toHaveAttribute("href", "/help"));
    expect(screen.getByTestId("help-button")).toHaveAccessibleName(help.openHelp);
  });
});
