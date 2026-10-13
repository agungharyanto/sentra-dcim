import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { mockApi } from "@/test/bootstrap";

import type { OpenApiDoc } from "./api";
import { ApiReference, groupOperations, resolveRef } from "./ApiReference";

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
        responses: { "200": { description: "OK" } },
      },
    },
    "/api/v1/help/pages/{slug}": {
      get: {
        tags: ["help"],
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
      ["system", ["get /api/v1/health"]],
      ["help", ["get /api/v1/help/pages/{slug}"]],
    ]);
    expect(resolveRef(doc, "#/components/responses/NotFound")).toEqual({ description: "Tidak ada" });
  });

  it("renders method, path, parameters, schemas and status codes — read-only", async () => {
    const fetch = mockApi({ "/api/v1/help/openapi": doc });
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ApiReference />
      </QueryClientProvider>,
    );
    const ref = await screen.findByTestId("api-reference");
    expect(ref).toHaveTextContent("Sentra DCIM API");
    expect(within(ref).getByRole("heading", { name: "system" })).toBeInTheDocument();

    const op = ref.querySelector('[data-operation="GET /api/v1/help/pages/{slug}"]') as HTMLElement;
    await userEvent.click(within(op).getByText("/api/v1/help/pages/{slug}"));
    expect(op).toHaveTextContent("slug");
    expect(op).toHaveTextContent("locale"); // $ref parameter resolved
    expect(op).toHaveTextContent("id | en");
    expect(op.querySelector('[data-status-code="200"]')).toHaveTextContent("Halaman");
    expect(op.querySelector('[data-status-code="404"]')).toHaveTextContent("Tidak ada"); // $ref response resolved
    expect(op).toHaveTextContent(resources.id.help.api.required);
    expect(ref.querySelector('[data-operation="GET /api/v1/health"]')).toHaveTextContent(resources.id.help.api.noAuth);

    // Descriptions are Markdown: inline code becomes <code>, wrapped lines join into one paragraph.
    const health = ref.querySelector('[data-operation="GET /api/v1/health"]') as HTMLElement;
    await userEvent.click(within(health).getByText("/api/v1/health"));
    expect(within(health).getByText("dependensi").tagName).toBe("CODE");
    expect(health).not.toHaveTextContent("`");

    // No way to execute requests: only the spec itself was fetched, and there is no form or send button.
    expect(within(ref).queryAllByRole("button")).toHaveLength(0);
    expect(ref.querySelector("form")).toBeNull();
    expect(fetch).toHaveBeenCalledTimes(1);
  });
});
