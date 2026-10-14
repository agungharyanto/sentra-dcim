import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";

import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";

import { Markdown, safeUrl } from "./Markdown";

const renderMd = (md: string) =>
  render(
    <MemoryRouter>
      <Markdown>{md}</Markdown>
    </MemoryRouter>,
  );

describe("help Markdown renderer", () => {
  beforeEach(async () => {
    await initI18n("id");
  });

  it("never renders raw HTML: <script> and <img onerror> stay text", () => {
    const { container } = renderMd('Halo <script>alert(1)</script> dan <img src=x onerror="alert(2)"> selesai.');
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("[onerror]")).toBeNull();
    expect(screen.getByTestId("help-markdown")).toHaveTextContent("<script>alert(1)</script>");
    expect(screen.getByTestId("help-markdown")).toHaveTextContent('<img src=x onerror="alert(2)">');
  });

  it("renders only help-asset images with alt text; drops external images and empty alt", () => {
    const { container } = renderMd(
      "![Langkah pertama di HP](/api/v1/help/assets/mfa-phone-add.svg)\n\n![logo](https://example.org/x.png)\n\n![](/api/v1/help/assets/a.svg)",
    );
    const imgs = container.querySelectorAll("img");
    expect(imgs).toHaveLength(1);
    expect(imgs[0]).toHaveAttribute("src", "/api/v1/help/assets/mfa-phone-add.svg");
    expect(imgs[0]).toHaveAttribute("alt", "Langkah pertama di HP");
  });

  it("routes internal links through the router", () => {
    renderMd("[Dasbor](/help/dashboard) dan [Referensi](/help/api/reference)");
    const a = screen.getByRole("link", { name: "Dasbor" });
    expect(a).toHaveAttribute("href", "/help/dashboard");
    expect(a).not.toHaveAttribute("target");
  });

  it("opens external https links in a new tab with noopener noreferrer", () => {
    renderMd("[Contoh](https://example.org/docs)");
    const a = screen.getByRole("link", { name: new RegExp("Contoh") });
    expect(a).toHaveAttribute("href", "https://example.org/docs");
    expect(a).toHaveAttribute("target", "_blank");
    expect(a).toHaveAttribute("rel", "noopener noreferrer");
    expect(a).toHaveTextContent(resources.id.help.newTab);
  });

  it.each(["javascript:alert(1)", "JAVASCRIPT:alert(1)", "data:text/html,x", "http://example.org", "//evil.example", "vbscript:x"])(
    "neutralises the unsafe link %s",
    (href) => {
      expect(safeUrl(href)).toBe("");
      const { container } = renderMd(`[klik](${href})`);
      expect(container.querySelector("a")).toBeNull();
      expect(screen.getByText("klik")).toBeInTheDocument();
    },
  );

  it("renders GFM tables and code", () => {
    const { container } = renderMd("| A | B |\n|---|---|\n| 1 | `<user>` |\n");
    expect(container.querySelector("table")).not.toBeNull();
    expect(container.querySelector("code")).toHaveTextContent("<user>");
  });
});
