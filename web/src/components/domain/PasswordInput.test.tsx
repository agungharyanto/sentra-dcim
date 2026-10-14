import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";

import { PasswordInput } from "./PasswordInput";

const tx = resources.id.common.password;

describe("PasswordInput (D-071)", () => {
  beforeEach(async () => {
    await initI18n("id");
  });

  it("is hidden by default, toggles with an accessible button, keeps autocomplete, hides again on submit", async () => {
    render(
      <form onSubmit={(e) => e.preventDefault()} data-testid="f">
        <label htmlFor="pw">Kata sandi</label>
        <PasswordInput id="pw" autoComplete="current-password" defaultValue="rahasia-panjang" />
        <button type="submit">Kirim</button>
      </form>,
    );
    const input = screen.getByLabelText("Kata sandi");
    expect(input).toHaveAttribute("type", "password");
    expect(input).toHaveAttribute("autocomplete", "current-password");

    const toggle = screen.getByRole("button", { name: tx.show });
    expect(toggle).toHaveAttribute("type", "button"); // never submits the form
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    await userEvent.click(toggle);
    expect(input).toHaveAttribute("type", "text");
    expect(input).toHaveAttribute("autocomplete", "current-password");
    expect(screen.getByRole("button", { name: tx.hide })).toHaveAttribute("aria-pressed", "true");

    fireEvent.submit(screen.getByTestId("f"));
    expect(await screen.findByRole("button", { name: tx.show })).toHaveAttribute("aria-pressed", "false");
    expect(input).toHaveAttribute("type", "password");
  });
});
