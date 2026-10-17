import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import type { Rack, Site, TreeSite } from "./api";

const tx = resources.id.facility;

const now = "2026-10-17T01:00:00Z";
const site: Site = { id: "s1", code: "DPS1", name: "Site Denpasar", address: "Jl. Raya", status: "active", version: 1, created_at: now, updated_at: now, archived_at: null };
const rack: Rack = {
  id: "k1",
  room_id: "r2",
  room_name: "MMR 1",
  room_type: "mmr",
  floor_id: "f1",
  floor_name: "Lantai 1",
  building_id: "b1",
  building_name: "Gedung A",
  site_id: "s1",
  site_code: "DPS1",
  site_name: "Site Denpasar",
  name: "MMR-01",
  asset_tag: "TAG-1",
  row_label: "A",
  row_position: 1,
  u_height: 42,
  starting_unit: 1,
  desc_units: false,
  width_in: 19,
  depth_mm: 1000,
  max_weight_kg: 900,
  max_power_kw: 7.5,
  status: "active",
  notes: "",
  version: 3,
  created_at: now,
  updated_at: now,
  archived_at: null,
};
const tree: TreeSite[] = [
  {
    id: "s1",
    code: "DPS1",
    name: "Site Denpasar",
    status: "active",
    rack_count: 1,
    buildings: [
      {
        id: "b1",
        code: "GA",
        name: "Gedung A",
        rack_count: 1,
        floors: [
          {
            id: "f1",
            name: "Lantai 1",
            level: 1,
            rack_count: 1,
            rooms: [
              { id: "r1", name: "Ruang Server", room_type: "whitespace", rack_count: 0, racks_by_status: {}, racks: [] },
              { id: "r2", name: "MMR 1", room_type: "mmr", rack_count: 1, racks_by_status: { active: 1 }, racks: [{ id: "k1", name: "MMR-01", status: "active" }] },
            ],
          },
        ],
      },
    ],
  },
];

interface Call {
  method: string;
  url: string;
  body: unknown;
  ifMatch: string | null;
}

function api(perms: string[] = allPermissions, fail?: Record<string, { status: number; body: unknown }>) {
  const calls: Call[] = [];
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const headers = (init?.headers ?? {}) as Record<string, string>;
      calls.push({ method, url, body: init?.body ? JSON.parse(String(init.body)) : undefined, ifMatch: headers["If-Match"] ?? null });
      const f = fail?.[`${method} ${url}`];
      if (f) return json(f.body, f.status);
      if (url.startsWith("/api/v1/auth/me")) return json(testMe({ permissions: perms, global_permissions: perms }));
      if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
      if (url === "/api/v1/facility/tree") return json({ sites: tree });
      if (url === "/api/v1/sites/s1") return json(site);
      if (url.startsWith("/api/v1/sites")) return json({ items: [site], total: 1, limit: 200, offset: 0 });
      if (url.startsWith("/api/v1/rooms?")) return json({ items: [], total: 0, limit: 200, offset: 0 });
      if (url === "/api/v1/racks/k1" && method === "PATCH") return json({ ...rack, name: "MMR-02", version: 4 });
      if (url === "/api/v1/racks/k1") return json(rack);
      if (url.startsWith("/api/v1/racks")) return json({ items: [rack], total: 1, limit: 50, offset: 0 });
      return json({ error: { code: "not_found", message: "x" } }, 404);
    }),
  );
  return calls;
}

describe("facility module (v0.4.1)", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("shows the tree, auto-opens the selection's path and marks the MMR room", async () => {
    api();
    window.history.pushState({}, "", "/facility/hierarchy?sel=rack:k1");
    render(<App queryClient={createQueryClient()} />);
    const treeEl = await screen.findByTestId("facility-tree");
    const item = within(treeEl).getByText("MMR-01").closest("[role=treeitem]");
    expect(item).toHaveAttribute("aria-selected", "true");
    const mmr = within(treeEl).getByText("MMR 1").closest("[role=treeitem]") as HTMLElement;
    expect(mmr.querySelector('[data-icon="mmr"]')).not.toBeNull();
    expect(await screen.findByTestId("facility-detail-name")).toHaveTextContent("MMR-01");
    expect(screen.getByTestId("facility-detail")).toHaveTextContent("DPS1 › Gedung A › Lantai 1 › MMR 1");
    expect(screen.getByText(tx.rackStatus.active)).toBeInTheDocument();
  });

  it("renames a rack with If-Match and shows a reload notice on 412", async () => {
    const calls = api(allPermissions, {
      "PATCH /api/v1/racks/k1": { status: 412, body: { error: { code: "version_conflict", message: "stale" } } },
    });
    window.history.pushState({}, "", "/facility/hierarchy?sel=rack:k1");
    render(<App queryClient={createQueryClient()} />);
    await userEvent.click(await screen.findByTestId("facility-rename"));
    const dialog = await screen.findByTestId("facility-dialog");
    const name = within(dialog).getByLabelText(tx.fields.name);
    await userEvent.clear(name);
    await userEvent.type(name, "MMR-02");
    await userEvent.click(within(dialog).getByTestId("facility-save"));
    expect(await within(dialog).findByTestId("facility-conflict")).toHaveTextContent(tx.conflict.title);
    const patch = calls.find((c) => c.method === "PATCH");
    expect(patch).toMatchObject({ url: "/api/v1/racks/k1", body: { name: "MMR-02" }, ifMatch: '"3"' });
  });

  it("refuses an invalid rack name before calling the API", async () => {
    const calls = api();
    window.history.pushState({}, "", "/facility/hierarchy?sel=rack:k1");
    render(<App queryClient={createQueryClient()} />);
    await userEvent.click(await screen.findByTestId("facility-rename"));
    const dialog = await screen.findByTestId("facility-dialog");
    const name = within(dialog).getByLabelText(tx.fields.name);
    await userEvent.clear(name);
    await userEvent.type(name, "-bad@name");
    await userEvent.click(within(dialog).getByTestId("facility-save"));
    expect(await within(dialog).findByTestId("facility-form-error")).toHaveTextContent(tx.hint.rackName);
    expect(calls.some((c) => c.method === "PATCH")).toBe(false);
  });

  it("read-only roles see no action buttons", async () => {
    api(["facility.read", "rack.read"]);
    window.history.pushState({}, "", "/facility/hierarchy?sel=site:s1");
    render(<App queryClient={createQueryClient()} />);
    expect(await screen.findByTestId("facility-detail-name")).toHaveTextContent("Site Denpasar");
    expect(screen.getByText(tx.readOnly)).toBeInTheDocument();
    for (const id of ["facility-edit", "facility-archive", "facility-add-child", "facility-add-site"]) {
      expect(screen.queryByTestId(id)).not.toBeInTheDocument();
    }
  });

  it("lists racks with filters and status badges", async () => {
    const calls = api();
    window.history.pushState({}, "", "/facility/racks");
    render(<App queryClient={createQueryClient()} />);
    const row = await screen.findByTestId("rack-row");
    expect(row).toHaveTextContent("MMR-01");
    expect(within(row).getByText(tx.rackStatus.active)).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByTestId("rack-status-filter"), "retired");
    await waitFor(() => expect(calls.some((c) => c.url.includes("status=retired"))).toBe(true));
  });
});
