import { act, render, screen } from "@testing-library/react";

import { BootstrapContext } from "@/features/bootstrap/regional";
import { initI18n } from "@/i18n";
import { testBootstrap } from "@/test/bootstrap";

import { Clock } from "./Clock";

describe("Clock", () => {
  beforeEach(async () => {
    await initI18n("id");
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-13T08:05:09Z"));
  });
  afterEach(() => vi.useRealTimers());

  const renderClock = (timezone: string) =>
    render(
      <BootstrapContext value={testBootstrap({ timezone })}>
        <Clock />
      </BootstrapContext>,
    );

  it("shows the time in the bootstrap time zone and ticks every second", () => {
    renderClock("Asia/Jakarta");
    const clock = screen.getByTestId("clock");
    expect(clock).toHaveTextContent("15.05.09");
    expect(clock).toHaveAttribute("dateTime", "2026-10-13T08:05:09.000Z"); // machine value stays UTC
    act(() => vi.advanceTimersByTime(1000));
    expect(clock).toHaveTextContent("15.05.10");
  });

  it("follows another server time zone", () => {
    renderClock("Asia/Makassar");
    expect(screen.getByTestId("clock")).toHaveTextContent("16.05.09");
  });

  it("is not a live region (never announced every second)", () => {
    renderClock("Asia/Jakarta");
    const clock = screen.getByTestId("clock");
    expect(clock).not.toHaveAttribute("aria-live");
    expect(clock.closest("[aria-live]")).toBeNull();
    expect(clock.closest('[role="timer"],[role="status"],[role="log"]')).toBeNull();
  });
});
