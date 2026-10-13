import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { TooltipProvider } from "@/components/ui/tooltip";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { mockApi } from "@/test/bootstrap";

import type { OpenApiDoc } from "./api";
import { accessOf, ApiReference, groupOperations, resolveRef } from "./ApiReference";

const doc: OpenApiDoc = {
  openapi: "3.1.0",
  info: { title: "Sentra DCIM API", version: "0.2.4" },
  tags: [{ name: "system", description: "Status" }, { name: "help" }],
  paths: {
    "/api/v1/health": {
      get: {
        tags: ["system"],
        summary: "Liveness",
        description: "Tanpa cek `dependensi`;\nbaris yang dibungkus YAML.",
        security: [],
        "x-sentra-access": "public",
        responses: { "200": { description: "OK" } },
      },
    },
    "/api/v1/ready": { get: { tags: ["system"], summary: "Readiness", security: [], "x-sentra-access": "internal", responses: {} } },
    "/api/v1/help/pages/{slug}": {
      get: {
        tags: ["help"],
        "x-sentra-access": "login",
        summary: "Halaman",
        parameters: [{ name: "slug", in: "path", required: true, schema: { type: "string" } }, { $ref: "#/components/parameters/Locale" } as never],
        responses: {
          "200": { description: "Halaman", content: { "application/json": { schema: { $ref: "#/components/schemas/Page" } } } },
          "404": { $ref: "#/components/responses/NotFound" },
        },
      },
    },
  },
  components: {
    parameters: { Locale: { name: "locale", in: "query", schema: { type: "string", enum: ["id", "en"] } } },
    responses: { NotFound: { description: "Tidak ada" } },
    schemas: {
      Page: { type: "object", required: ["slug"], properties: { slug: { type: "string" }, self: { $ref: "#/components/schemas/Page" } } },
    },
  },
};

describe("API reference viewer", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("groups operations by tag in spec order and resolves $ref", () => {
    expect(groupOperations(doc).map(([tag, ops]) => [tag, ops.map((o) => `${o.method} ${o.path}`)])).toEqual([
      ["system", ["get /api/v1/health", "get /api/v1/ready"]],
      ["help", ["get /api/v1/help/pages/{slug}"]],
    ]);
    expect(resolveRef(doc, "#/components/responses/NotFound")).toEqual({ description: "Tidak ada" });
  });

  it("renders method, path, parameters, schemas and status codes — read-only", async () => {
    const fetch = mockApi({ "/api/v1/help/openapi": doc });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <TooltipProvider>
          <ApiReference />
        </TooltipProvider>
      </QueryClientProvider>,
    );
    const ref = await screen.findByTestId("api-reference");
    expect(ref).toHaveTextContent("Sentra DCIM API");
    expect(within(ref).getByRole("heading", { name: "system" })).toBeInTheDocument();

    const op = ref.querySelector('[data-operation="GET /api/v1/help/pages/{slug}"]') as HTMLElement;
    const toggle = within(op).getByTestId("operation-toggle");
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(op).toHaveTextContent("slug");
    expect(op).toHaveTextContent("locale"); // $ref parameter resolved
    expect(op).toHaveTextContent("id | en");
    expect(op.querySelector('[data-status-code="200"]')).toHaveTextContent("Halaman");
    expect(op.querySelector('[data-status-code="404"]')).toHaveTextContent("Tidak ada"); // $ref response resolved
    expect(op).toHaveTextContent(resources.id.help.api.required);
    // Access labels come from x-sentra-access, not from the path.
    const label = (p: string) => ref.querySelector(`[data-operation="${p}"] [data-access]`);
    expect(label("GET /api/v1/health")).toHaveTextContent(resources.id.help.api.access.public);
    expect(label("GET /api/v1/ready")).toHaveTextContent(resources.id.help.api.access.internal);
    expect(label("GET /api/v1/help/pages/{slug}")).toHaveTextContent(resources.id.help.api.access.login);

    // Descriptions are Markdown: inline code becomes <code>, wrapped lines join into one paragraph.
    const health = ref.querySelector('[data-operation="GET /api/v1/health"]') as HTMLElement;
    await userEvent.click(within(health).getByTestId("operation-toggle"));
    expect(within(health).getByText("dependensi").tagName).toBe("CODE");
    expect(health).not.toHaveTextContent("`");

    // No way to execute requests: the only buttons are disclosure toggles and copy buttons, there is no form, and
    // only the spec itself was fetched.
    const buttons = within(ref).getAllByRole("button");
    expect(buttons.every((b) => b.dataset.testid === "operation-toggle" || /^Salin path \//.test(b.getAttribute("aria-label") ?? ""))).toBe(true);
    expect(ref.querySelector("form")).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("copies the plain path without toggling the endpoint", async () => {
    mockApi({ "/api/v1/help/openapi": doc });
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <TooltipProvider>
          <ApiReference />
        </TooltipProvider>
      </QueryClientProvider>,
    );
    const op = (await screen.findByTestId("api-reference")).querySelector('[data-operation="GET /api/v1/health"]') as HTMLElement;
    const copy = within(op).getByRole("button", { name: "Salin path /api/v1/health" });
    await act(async () => {
      fireEvent.click(copy);
    });
    expect(writeText).toHaveBeenCalledWith("/api/v1/health");
    expect(within(op).getByTestId("operation-toggle")).toHaveAttribute("aria-expanded", "false");
    expect(op).toHaveAttribute("data-open", "false");
  });

  it("reads access only from x-sentra-access", () => {
    expect(accessOf({ "x-sentra-access": "internal" })).toBe("internal");
    expect(accessOf({ security: [] })).toBeUndefined();
    expect(accessOf({ "x-sentra-access": "everyone" })).toBeUndefined();
  });
});
