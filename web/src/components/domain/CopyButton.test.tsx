import { act, fireEvent, render, screen } from "@testing-library/react";
import { toast } from "sonner";

import { CopyButton } from "@/components/domain/CopyButton";
import { TooltipProvider } from "@/components/ui/tooltip";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";

vi.mock("sonner", async (orig) => ({ ...(await orig<typeof import("sonner")>()), toast: { error: vi.fn() } }));

function renderButton(onParentClick = vi.fn()) {
  render(
    <TooltipProvider>
      <div onClick={onParentClick}>
        <CopyButton value="/api/v1/health" label="Salin path /api/v1/health" />
      </div>
    </TooltipProvider>,
  );
  return { button: screen.getByRole("button", { name: "Salin path /api/v1/health" }), onParentClick };
}

/** Clicks and lets the clipboard promise settle. */
async function click(el: HTMLElement) {
  await act(async () => {
    fireEvent.click(el);
  });
}

describe("CopyButton", () => {
  beforeEach(async () => {
    await initI18n("id");
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("copies the plain value, shows Check + 'Disalin' for ~2 s, then reverts", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
    const { button, onParentClick } = renderButton();

    await click(button);
    expect(writeText).toHaveBeenCalledWith("/api/v1/health");
    expect(onParentClick).not.toHaveBeenCalled(); // click never reaches the surrounding row
    expect(button).toHaveAttribute("data-copied", "true");
    expect(button.querySelector(".lucide-check")).not.toBeNull();
    expect(button.className).toContain("text-status-normal-solid");
    expect(button.className).not.toContain("hover:text-accent-foreground"); // hover must not hide the success colour
    expect(button.querySelector('[aria-live="polite"]')).toHaveTextContent(resources.id.ds.copy.copied);

    act(() => vi.advanceTimersByTime(2000));
    expect(button).toHaveAttribute("data-copied", "false");
    expect(button.querySelector(".lucide-copy")).not.toBeNull();
    expect(button.querySelector('[aria-live="polite"]')).toHaveTextContent("");
  });

  it("shows an error toast when the clipboard rejects", async () => {
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText: vi.fn().mockRejectedValue(new Error("denied")) } });
    const { button } = renderButton();
    await click(button);
    expect(toast.error).toHaveBeenCalledWith(expect.stringContaining("/api/v1/health"));
    expect(button).toHaveAttribute("data-copied", "false");
  });

  it("shows an error toast when the clipboard API is unavailable", async () => {
    vi.stubGlobal("navigator", { ...navigator, clipboard: undefined });
    const { button } = renderButton();
    await click(button);
    expect(toast.error).toHaveBeenCalledTimes(1);
  });
});
