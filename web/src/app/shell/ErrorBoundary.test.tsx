import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router";

import { ThemeProvider } from "@/components/domain/ThemeProvider";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BootstrapContext } from "@/features/bootstrap/regional";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { testBootstrap } from "@/test/bootstrap";

import { AppShell } from "./AppShell";

function Boom(): never {
  throw new Error("boom");
}

describe("page error boundary", () => {
  beforeEach(async () => {
    await initI18n("id");
    vi.spyOn(console, "error").mockImplementation(() => {});
  });
  afterEach(() => vi.restoreAllMocks());

  it("shows a translated error page and resets when navigating to another page", async () => {
    render(
      <ThemeProvider>
        <TooltipProvider>
          <BootstrapContext value={testBootstrap()}>
            <MemoryRouter initialEntries={["/alarms"]}>
              <Routes>
                <Route element={<AppShell />}>
                  <Route path="/alarms" element={<Boom />} />
                  <Route path="/dashboard" element={<p>dashboard ok</p>} />
                </Route>
              </Routes>
            </MemoryRouter>
          </BootstrapContext>
        </TooltipProvider>
      </ThemeProvider>,
    );

    expect(await screen.findByTestId("page-error")).toHaveTextContent(resources.id.shell.pages.error.title);
    // The shell survives: navigation still works and clears the error (boundary keyed by pathname).
    const nav = screen.getByRole("navigation", { name: resources.id.shell.layout.mainNav });
    await userEvent.click(within(nav).getByRole("link", { name: resources.id.shell.sidebar.dashboard }));
    expect(await screen.findByText("dashboard ok")).toBeInTheDocument();
    expect(screen.queryByTestId("page-error")).not.toBeInTheDocument();
  });
});
