import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { initI18n } from "@/i18n";

import { toast, Toaster } from "./toast";

describe("toast", () => {
  beforeEach(async () => {
    await initI18n("id");
  });

  it("shows success politely and errors assertively, closes on click and by itself", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    render(<Toaster />);
    act(() => {
      toast.success("Tersimpan");
      toast.error("Gagal", { description: "Coba lagi" });
    });
    expect(screen.getByRole("status")).toHaveTextContent("Tersimpan");
    expect(screen.getByRole("alert")).toHaveTextContent("GagalCoba lagi");
    await userEvent.click(screen.getAllByRole("button", { name: "Tutup" })[0] as HTMLElement);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    act(() => vi.advanceTimersByTime(8000));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    vi.useRealTimers();
  });

  it("never injects a <style> element (CSP nonce, ISS-030)", () => {
    const before = document.querySelectorAll("style").length;
    render(<Toaster />);
    act(() => {
      toast("Info");
    });
    expect(document.querySelectorAll("style")).toHaveLength(before);
  });
});
