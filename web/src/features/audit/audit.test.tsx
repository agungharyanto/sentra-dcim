import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import { utcToZonedLocal, zonedLocalToUtc } from "@/lib/format";

import { actionPrefixes, type AuditEvent, type AuditVerifyResult } from "./api";

const tx = resources.id.audit;

const ev = (seq: number, o: Partial<AuditEvent> = {}): AuditEvent => ({
  seq,
  occurred_at: "2026-10-14T01:02:03Z",
  actor_type: "user",
  actor_id: "u-1",
  actor_username: "operator.satu",
  action: "auth.login.success",
  target_type: "session",
  target_id: `s-${seq}`,
  result: "success",
  ip: "192.0.2.7",
  user_agent: "Mozilla/5.0",
  request_id: `req-${seq}`,
  metadata: {},
  hash: "ab".repeat(32),
  ...o,
});

const verified = (o: Partial<AuditVerifyResult> = {}): AuditVerifyResult => ({
  ok: true,
  checked: 42,
  from_seq: 1,
  head_seq: 42,
  head_hash: "cd".repeat(32),
  checkpoint_seq: 0,
  anchors_checked: 0,
  broken_seq: 0,
  verified_at: "2026-10-14T02:00:00Z",
  ...o,
});

interface Call {
  method: string;
  url: string;
}

function api(opts: { perms?: string[]; last?: AuditVerifyResult | null; verify?: AuditVerifyResult; pages?: AuditEvent[][] } = {}) {
  const calls: Call[] = [];
  const pages = opts.pages ?? [[ev(3), ev(2, { action: "auth.login.failure", result: "failure", actor_username: "", actor_type: "anonymous", metadata: { reason: "unknown_user" } })]];
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? "GET";
    calls.push({ method, url });
    if (url.startsWith("/api/v1/auth/me")) return json(testMe({ permissions: opts.perms ?? allPermissions }));
    if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
    if (url.startsWith("/api/v1/audit/actions")) return json({ actions: ["auth.login.failure", "auth.login.success", "user.create"] });
    if (url.startsWith("/api/v1/audit/integrity/verify")) return json(opts.verify ?? verified());
    if (url.startsWith("/api/v1/audit/integrity")) return json({ last: opts.last ?? null, running: false });
    if (url.startsWith("/api/v1/audit/events?")) {
      const before = new URL(url, "http://x").searchParams.get("before");
      const i = before === null ? 0 : pages.findIndex((p) => p[0] !== undefined && p[0].seq < Number(before));
      const page = pages[i] ?? [];
      const next = i + 1 < pages.length ? (page.at(-1)?.seq ?? null) : null;
      return json({ events: page, next_before: next, limit: 50 });
    }
    return json({ error: { code: "not_found", message: "x" } }, 404);
  });
  vi.stubGlobal("fetch", fn);
  return calls;
}

/** Element i of a list, failing the test when it is missing. */
function at<T>(list: T[], i: number): T {
  const v = list[i];
  if (v === undefined) throw new Error(`no element ${i}`);
  return v;
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("audit log", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("lists events newest first with WIB times, results and anonymous actors", async () => {
    api();
    renderAt("/audit");
    const rows = await screen.findAllByTestId("audit-row");
    expect(rows.map((r) => r.dataset.seq)).toEqual(["3", "2"]);
    expect(within(at(rows, 0)).getByText("operator.satu")).toBeInTheDocument();
    expect(within(at(rows, 0)).getByText(/08\.02\.03|08:02:03/)).toBeInTheDocument(); // 01:02:03 UTC = 08:02:03 WIB
    expect(within(at(rows, 1)).getByText(tx.actorType.anonymous)).toBeInTheDocument();
    expect(within(at(rows, 1)).getByText(tx.result.failure)).toHaveAttribute("data-result", "failure");
  });

  it("shows the integrity status and verifies on demand", async () => {
    const calls = api({ last: null, verify: verified({ checked: 7, head_seq: 7 }) });
    renderAt("/audit");
    const card = await screen.findByTestId("audit-integrity");
    await waitFor(() => expect(card).toHaveAttribute("data-state", "unknown"));
    await userEvent.click(screen.getByTestId("audit-verify"));
    await waitFor(() => expect(card).toHaveAttribute("data-state", "intact"));
    expect(calls.some((c) => c.method === "POST" && c.url === "/api/v1/audit/integrity/verify")).toBe(true);
    expect(within(card).getByText(/7 event diperiksa/)).toBeInTheDocument();
  });

  it("reports a broken chain with the first bad seq", async () => {
    api({ last: verified({ ok: false, broken_seq: 5, reason: "row content does not match its hash (modified)" }) });
    renderAt("/audit");
    expect(await screen.findByTestId("audit-broken")).toHaveTextContent("seq 5");
    expect(screen.getByTestId("audit-integrity")).toHaveAttribute("data-state", "broken");
  });

  it("loads older pages with the keyset cursor", async () => {
    const calls = api({ pages: [[ev(5), ev(4)], [ev(3), ev(2)], [ev(1)]] });
    renderAt("/audit");
    await screen.findAllByTestId("audit-row");
    await userEvent.click(screen.getByTestId("audit-more"));
    await waitFor(() => expect(screen.getAllByTestId("audit-row")).toHaveLength(4));
    expect(calls.some((c) => c.url.includes("before=4"))).toBe(true);
    await userEvent.click(screen.getByTestId("audit-more"));
    await waitFor(() => expect(screen.getAllByTestId("audit-row")).toHaveLength(5));
    expect(screen.queryByTestId("audit-more")).not.toBeInTheDocument();
    expect(screen.getByText(tx.table.end)).toBeInTheDocument();
  });

  it("applies filters (WIB → UTC, action prefix) and exports with the same filters", async () => {
    const calls = api();
    renderAt("/audit");
    await screen.findAllByTestId("audit-row");
    // Date picker (ISS-043): pick the 14th of the month shown (current month in WIB) → 00:00 WIB in UTC.
    await userEvent.click(screen.getAllByTestId("datetime-trigger")[0] as HTMLElement);
    await userEvent.click((await screen.findByTestId("datetime-popover")).querySelector('[data-day="14"]') as HTMLElement);
    await userEvent.click(screen.getByTestId("datetime-done"));
    const ym = utcToZonedLocal(new Date().toISOString(), "Asia/Jakarta").slice(0, 7);
    await userEvent.type(screen.getByLabelText(tx.filter.actor), "operator");
    await userEvent.selectOptions(screen.getByTestId("audit-action"), "auth");
    await userEvent.selectOptions(screen.getByLabelText(tx.filter.result), "failure");
    await userEvent.click(screen.getByTestId("audit-apply"));
    await waitFor(() => expect(calls.some((c) => c.url.includes("action=auth&"))).toBe(true));
    const evCalls = calls.filter((c) => c.url.startsWith("/api/v1/audit/events?"));
    const last = at(evCalls, evCalls.length - 1);
    const p = new URL(last.url, "http://x").searchParams;
    expect(p.get("since")).toBe(zonedLocalToUtc(`${ym}-14T00:00`, "Asia/Jakarta"));
    expect(p.get("actor")).toBe("operator");
    expect(p.get("result")).toBe("failure");
    const href = screen.getByTestId("audit-export").getAttribute("href") ?? "";
    expect(href.startsWith("/api/v1/audit/events/export.csv?")).toBe(true);
    expect(new URL(href, "http://x").searchParams.get("action")).toBe("auth");
  });

  it("hides the export without audit.export", async () => {
    api({ perms: ["audit.read"] });
    renderAt("/audit");
    await screen.findAllByTestId("audit-row");
    expect(screen.queryByTestId("audit-export")).not.toBeInTheDocument();
  });

  it("shows event details with metadata", async () => {
    api();
    renderAt("/audit");
    const rows = await screen.findAllByTestId("audit-row");
    await userEvent.click(within(at(rows, 1)).getByRole("button", { name: "Detail event seq 2" }));
    const dialog = await screen.findByTestId("audit-detail");
    expect(within(dialog).getByTestId("audit-metadata")).toHaveTextContent('"reason": "unknown_user"');
    expect(within(dialog).getByText("req-2")).toBeInTheDocument();
  });

  it("denies the page without audit.read", async () => {
    api({ perms: ["users.read"] });
    renderAt("/audit");
    expect(await screen.findByText(resources.id.shell.pages.forbidden.title)).toBeInTheDocument();
    expect(screen.queryByTestId("audit-page")).not.toBeInTheDocument();
  });

  it("derives action prefixes", () => {
    expect(actionPrefixes(["auth.login.failure", "auth.logout", "user.create"])).toEqual(["auth", "auth.login", "user"]);
  });
});
