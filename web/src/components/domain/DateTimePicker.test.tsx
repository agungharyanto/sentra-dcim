import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";

import type { Regional } from "@/features/auth/api";
import { AuthContext, type AuthState } from "@/features/auth/auth-context";
import { BootstrapContext } from "@/features/bootstrap/regional";
import { initI18n } from "@/i18n";
import { utcToZonedLocal, zonedLocalToUtc } from "@/lib/format";
import { testBootstrap, testMe } from "@/test/bootstrap";

import { DateTimePicker } from "./DateTimePicker";

function Harness({ regional, initial = "", onValue }: { regional: Regional; initial?: string; onValue: (v: string) => void }) {
  const [v, setV] = useState(initial);
  const auth: AuthState = {
    me: { ...testMe(), regional },
    expired: false,
    signIn: async () => testMe(),
    signOut: async () => undefined,
    refresh: async () => undefined,
  };
  return (
    <BootstrapContext.Provider value={testBootstrap()}>
      <AuthContext.Provider value={auth}>
        <DateTimePicker
          id="dt"
          value={v}
          onChange={(x) => {
            setV(x);
            onValue(x);
          }}
        />
      </AuthContext.Provider>
    </BootstrapContext.Provider>
  );
}

const wib24: Regional = { language: "id", timezone: "Asia/Jakarta", time_format: "24h", date_format: "DD/MM/YYYY" };

describe("DateTimePicker", () => {
  beforeEach(async () => {
    await initI18n("id");
  });

  it("shows the effective pattern when empty and the formatted value otherwise", () => {
    const { unmount } = render(<Harness regional={wib24} onValue={() => undefined} />);
    expect(screen.getByTestId("datetime-trigger")).toHaveTextContent("DD/MM/YYYY HH.mm");
    unmount();
    render(<Harness regional={{ ...wib24, date_format: "YYYY-MM-DD" }} initial="2026-10-14T00:30:00Z" onValue={() => undefined} />);
    expect(screen.getByTestId("datetime-trigger")).toHaveTextContent("2026-10-14 07.30");
  });

  it("picks a day and a 12 h time in the user's zone and returns UTC", async () => {
    await initI18n("en"); // the UI language is the effective language (AuthProvider keeps them in sync)
    const onValue = vi.fn();
    render(
      <Harness regional={{ language: "en", timezone: "Asia/Makassar", time_format: "12h", date_format: "DD MMM YYYY" }} initial="2026-10-14T00:30:00Z" onValue={onValue} />,
    );
    expect(screen.getByTestId("datetime-trigger")).toHaveTextContent("14 Oct 2026 8:30 AM");
    await userEvent.click(screen.getByTestId("datetime-trigger"));
    const pop = await screen.findByTestId("datetime-popover");
    await userEvent.click(pop.querySelector('[data-day="20"]') as HTMLElement);
    expect(onValue).toHaveBeenLastCalledWith(zonedLocalToUtc("2026-10-20T08:30", "Asia/Makassar"));
    await userEvent.selectOptions(within(pop).getByLabelText(/AM\/PM/), "PM");
    expect(onValue).toHaveBeenLastCalledWith("2026-10-20T12:30:00Z"); // 20.30 WITA
    await userEvent.selectOptions(within(pop).getByLabelText(/hour|jam/i), "12");
    expect(utcToZonedLocal(onValue.mock.lastCall?.[0] as string, "Asia/Makassar")).toBe("2026-10-20T12:30");
  });

  it("moves the focused day with the arrow keys and changes month with Page Down", async () => {
    render(<Harness regional={wib24} initial="2026-10-31T03:00:00Z" onValue={() => undefined} />);
    await userEvent.click(screen.getByTestId("datetime-trigger"));
    const pop = await screen.findByTestId("datetime-popover");
    expect(document.activeElement).toHaveAttribute("data-day", "31");
    await userEvent.keyboard("{ArrowRight}");
    expect(document.activeElement).toHaveAttribute("data-day", "1");
    expect(within(pop).getByTestId("datetime-month")).toHaveTextContent(/November 2026/);
    await userEvent.keyboard("{PageDown}");
    expect(within(pop).getByTestId("datetime-month")).toHaveTextContent(/Desember 2026/);
  });

  it("clears the value", async () => {
    const onValue = vi.fn();
    render(<Harness regional={wib24} initial="2026-10-14T00:30:00Z" onValue={onValue} />);
    await userEvent.click(screen.getByTestId("datetime-clear"));
    expect(onValue).toHaveBeenLastCalledWith("");
    expect(screen.getByTestId("datetime-trigger")).toHaveTextContent("DD/MM/YYYY HH.mm");
  });
});
