import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { mockBootstrapFetch, testBootstrap } from "@/test/bootstrap";
import registry from "@/test/fixtures/ui-registry.json";

const shell = resources.id.shell;
const en = resources.en.shell;

function renderAt(path: string, boot = testBootstrap()) {
  mockBootstrapFetch(boot);
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

/** The desktop sidebar (the mobile drawer only mounts when opened). */
async function sidebar() {
  return within(await screen.findByRole("navigation", { name: shell.layout.mainNav }));
}

describe("app shell", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("redirects / to /dashboard and shows the empty dashboard", async () => {
    renderAt("/");
    expect(await screen.findByTestId("dashboard-empty")).toHaveTextContent(shell.dashboard.emptyTitle);
    expect(window.location.pathname).toBe("/dashboard");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(shell.sidebar.dashboard);
  });

  it("renders the sidebar from the registry, grouped, with status badges and aria-current", async () => {
    renderAt("/dashboard");
    const nav = await sidebar();
    for (const g of registry.menu_groups) {
      expect(nav.getByRole("group", { name: shell.groups[g.key as keyof typeof shell.groups] })).toBeInTheDocument();
    }
    const links = nav.getAllByRole("link");
    expect(links).toHaveLength(13);
    expect(links.map((l) => l.getAttribute("href"))).toEqual(registry.menus.map((m) => m.path));
    expect(nav.getByRole("link", { name: shell.sidebar.dashboard })).toHaveAttribute("aria-current", "page");
    // Infrastructure group contains exactly its three menus, in order.
    const infra = nav.getByRole("group", { name: shell.groups.infrastructure });
    expect(within(infra).getAllByRole("link").map((l) => l.getAttribute("href"))).toEqual(["/facility", "/assets", "/tenants"]);
    expect(within(nav.getByRole("link", { name: new RegExp(shell.sidebar.alarms) })).getByText(shell.menuStatus.coming_soon)).toBeInTheDocument();
    expect(screen.getByTestId("app-version")).toHaveTextContent("v0.2.3");
  });

  it("navigates to a coming-soon module with breadcrumb and aria-current", async () => {
    renderAt("/dashboard");
    const nav = await sidebar();
    await userEvent.click(nav.getByRole("link", { name: new RegExp(shell.sidebar.facility) }));
    expect(await screen.findByTestId("coming-soon")).toHaveTextContent(shell.menuDescription.facility);
    expect(window.location.pathname).toBe("/facility");
    expect(nav.getByRole("link", { name: new RegExp(shell.sidebar.facility) })).toHaveAttribute("aria-current", "page");
    const crumbs = screen.getByRole("navigation", { name: shell.layout.breadcrumb });
    expect(crumbs).toHaveTextContent(`Sentra DCIM${shell.groups.infrastructure}${shell.sidebar.facility}`);
  });

  it("shows NotConnected for not_connected menus", async () => {
    const boot = testBootstrap();
    boot.menus = boot.menus.map((m) => (m.key === "power" ? { ...m, status: "not_connected" } : m));
    renderAt("/power", boot);
    expect(await screen.findByTestId("not-connected")).toHaveTextContent(shell.pages.notConnected.title);
  });

  it("shows a 404 page inside the shell for unknown paths", async () => {
    renderAt("/does/not/exist");
    expect(await screen.findByTestId("not-found")).toHaveTextContent("/does/not/exist");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(shell.pages.notFound.title);
    expect(await sidebar()).toBeTruthy();
  });

  it("collapses the sidebar, remembers it, and shows names as tooltips", async () => {
    const { unmount } = renderAt("/dashboard");
    await sidebar();
    await userEvent.click(screen.getByRole("button", { name: shell.layout.collapse }));
    expect(document.getElementById("app-sidebar")).toHaveAttribute("data-collapsed", "true");
    expect(window.localStorage.getItem("sentra.sidebar")).toBe("collapsed");
    unmount();

    renderAt("/dashboard");
    const nav = await sidebar();
    expect(document.getElementById("app-sidebar")).toHaveAttribute("data-collapsed", "true");
    expect(screen.getByRole("button", { name: shell.layout.expand })).toHaveAttribute("aria-expanded", "false");
    // Regression: Radix Slot (tooltip trigger) must receive a string className, not NavLink's function form.
    for (const link of nav.getAllByRole("link")) {
      expect(link.className).toContain("justify-center");
      expect(link.className).not.toMatch(/=>|function/);
    }
    await userEvent.hover(nav.getByRole("link", { name: new RegExp(shell.sidebar.assets) }));
    expect(await screen.findByRole("tooltip")).toHaveTextContent(shell.sidebar.assets);
  });

  it("falls back to expanded when storage throws", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    renderAt("/dashboard");
    await sidebar();
    expect(document.getElementById("app-sidebar")).toHaveAttribute("data-collapsed", "false");
    vi.restoreAllMocks();
  });

  it("translates the shell when switching to English", async () => {
    renderAt("/alarms");
    await screen.findByTestId("coming-soon");
    await userEvent.click(screen.getByRole("button", { name: /English/ }));
    await waitFor(() => expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(en.sidebar.alarms));
    expect(screen.getByTestId("coming-soon")).toHaveTextContent(en.pages.comingSoon.title);
  });

  it("offers a skip link to the main content", async () => {
    renderAt("/dashboard");
    await sidebar();
    expect(screen.getByRole("link", { name: shell.layout.skipToContent })).toHaveAttribute("href", "#main-content");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
  });
});
