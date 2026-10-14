import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import type { AllowlistState } from "./api";

const tx = resources.id.settings.allowlist;

interface Call {
  method: string;
  url: string;
  body: unknown;
}

function api(initial: Partial<AllowlistState> = {}, fail?: Record<string, { status: number; body: unknown }>) {
  const calls: Call[] = [];
  let st: AllowlistState = {
    mode: "off",
    pending_until: null,
    entries: [],
    your_ip: "203.0.113.5",
    your_ip_in_list: false,
    always_public: ["/api/v1/health", "/.well-known/acme-challenge/*"],
    confirm_window_seconds: 120,
    server_time: "2026-10-15T01:00:00Z",
    ...initial,
  };
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      const body = init?.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ method, url, body });
      const f = fail?.[`${method} ${url}`];
      if (f) return json(f.body, f.status);
      if (url.startsWith("/api/v1/auth/me")) return json(testMe({ permissions: allPermissions }));
      if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
      if (url === "/api/v1/settings/allowed-ips" && method === "POST") {
        const cidr = String(body.cidr).includes("/") ? body.cidr : `${body.cidr}/32`;
        st = {
          ...st,
          entries: [...st.entries, { id: `e${st.entries.length}`, cidr, label: body.label, created_at: st.server_time, created_by: null, updated_at: st.server_time }],
          your_ip_in_list: st.your_ip_in_list || cidr === `${st.your_ip}/32`,
        };
        return json(st, 201);
      }
      if (url === "/api/v1/settings/allowed-ips/mode") {
        st = { ...st, mode: body.mode, pending_until: body.mode === "enforce" ? "2026-10-15T01:02:00Z" : null };
        return json(st);
      }
      if (url === "/api/v1/settings/allowed-ips/confirm") {
        st = { ...st, pending_until: null };
        return json(st);
      }
      if (url.startsWith("/api/v1/settings/allowed-ips/") && method === "DELETE") {
        st = { ...st, entries: st.entries.filter((e) => !url.endsWith(e.id)) };
        return json(st);
      }
      if (url === "/api/v1/settings/allowed-ips") return json(st);
      return json({ error: { code: "not_found", message: "x" } }, 404);
    }),
  );
  return calls;
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("settings › allowed IP", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => vi.unstubAllGlobals());

  it("shows the current IP, guards enforce until it is listed, then asks for confirmation", async () => {
    const calls = api();
    renderAt("/settings/allowed-ip");
    const yours = await screen.findByTestId("allowlist-your-ip");
    expect(yours).toHaveTextContent("203.0.113.5");
    expect(yours).toHaveAttribute("data-covered", "false");
    expect(screen.getByTestId("allowlist-always-public")).toHaveTextContent("/.well-known/acme-challenge/*");

    await userEvent.click(screen.getByTestId("allowlist-mode-enforce"));
    expect(screen.getByTestId("allowlist-guard")).toHaveTextContent(tx.guard);
    expect(screen.getByTestId("allowlist-mode-save")).toBeDisabled();

    await userEvent.click(screen.getByTestId("allowlist-add-mine"));
    await waitFor(() => expect(screen.getByTestId("allowlist-your-ip")).toHaveAttribute("data-covered", "true"));
    expect(calls.find((c) => c.method === "POST")?.body).toMatchObject({ cidr: "203.0.113.5" });

    await userEvent.click(screen.getByTestId("allowlist-mode-enforce"));
    await userEvent.click(screen.getByTestId("allowlist-mode-save"));
    const dialog = await screen.findByTestId("confirm-dialog");
    expect(dialog).toHaveTextContent("120 detik");
    await userEvent.click(within(dialog).getByTestId("confirm"));
    await waitFor(() => expect(calls.some((c) => c.method === "PUT" && c.url.endsWith("/mode"))).toBe(true));
    expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ mode: "enforce" });

    // Countdown banner and "Keep".
    const banner = await screen.findByTestId("allowlist-pending");
    expect(banner).toHaveTextContent(/[0-2]:\d\d/);
    await userEvent.click(screen.getByTestId("allowlist-keep"));
    await waitFor(() => expect(calls.some((c) => c.url.endsWith("/confirm"))).toBe(true));
    await waitFor(() => expect(screen.queryByTestId("allowlist-pending")).not.toBeInTheDocument());
  });

  it("monitor and off are saved without the enforce confirmation", async () => {
    const calls = api({ entries: [{ id: "e0", cidr: "192.0.2.0/24", label: "Kantor", created_at: "2026-10-15T01:00:00Z", created_by: null, updated_at: "2026-10-15T01:00:00Z" }] });
    renderAt("/settings/allowed-ip");
    await userEvent.click(await screen.findByTestId("allowlist-mode-monitor"));
    await userEvent.click(screen.getByTestId("allowlist-mode-save"));
    await waitFor(() => expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ mode: "monitor" }));
    expect(screen.queryByTestId("confirm-dialog")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("allowlist-row")[0]).toHaveTextContent("Kantor");
  });

  it("shows validation and lockout errors from the server", async () => {
    api(
      { mode: "enforce", your_ip_in_list: true, entries: [{ id: "e0", cidr: "203.0.113.5/32", label: "", created_at: "2026-10-15T01:00:00Z", created_by: null, updated_at: "2026-10-15T01:00:00Z" }] },
      {
        "POST /api/v1/settings/allowed-ips": { status: 422, body: { error: { code: "validation_failed", message: "x", field: "cidr" } } },
        "DELETE /api/v1/settings/allowed-ips/e0": { status: 409, body: { error: { code: "lockout_risk", message: "x" } } },
      },
    );
    renderAt("/settings/allowed-ip");
    await userEvent.type(await screen.findByTestId("allowlist-cidr"), "10.0.0.5/24");
    await userEvent.click(screen.getByTestId("allowlist-add-submit"));
    expect(await screen.findByRole("alert")).toHaveTextContent(resources.id.errors.code.validation_failed);

    await userEvent.click(screen.getAllByTestId("allowlist-remove")[0] as HTMLElement);
    const dialog = await screen.findByTestId("confirm-dialog");
    expect(dialog).toHaveTextContent(tx.removeBodyEnforce);
    await userEvent.click(within(dialog).getByTestId("confirm"));
    expect(await screen.findByText(resources.id.errors.code.lockout_risk)).toBeInTheDocument();
  });
});
