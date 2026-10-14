import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { App, createQueryClient } from "@/app/App";
import { initI18n } from "@/i18n";
import { resources } from "@/i18n/resources";
import { allPermissions, testBootstrap, testMe } from "@/test/bootstrap";

import type { Role, User } from "./api";

const tx = resources.id.users;
const errors = resources.id.errors;

const user = (o: Partial<User>): User => ({
  id: "u-2",
  username: "teknisi.01",
  display_name: "Teknisi Satu",
  email: null,
  status: "active",
  roles: ["technician"],
  must_change_password: false,
  locked: false,
  last_login_at: null,
  created_at: "2026-10-14T01:00:00Z",
  updated_at: "2026-10-14T01:00:00Z",
  ...o,
});
const me = user({ id: "u-1", username: "operator.satu", display_name: "Operator Satu", roles: ["admin"] });
const roles: Role[] = [
  { key: "admin", system: true, permissions: allPermissions, user_count: 1 },
  { key: "technician", system: true, permissions: [], user_count: 1 },
  { key: "auditor", system: true, permissions: ["roles.read", "users.read"], user_count: 0 },
];
const TEMP = "abcd-efgh-jkmn-pqrs";

interface Call {
  method: string;
  url: string;
  body: unknown;
}

/** API double: users/roles endpoints with recorded calls; `fail` maps "METHOD path-prefix" → error response. */
function api(opts: { perms?: string[]; users?: User[]; fail?: Record<string, { status: number; body: unknown }> } = {}) {
  const calls: Call[] = [];
  const users = opts.users ?? [me, user({})];
  const json = (b: unknown, status = 200) => new Response(JSON.stringify(b), { status });
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method ?? "GET";
    const body = init?.body ? JSON.parse(String(init.body)) : undefined;
    calls.push({ method, url, body });
    for (const [k, v] of Object.entries(opts.fail ?? {})) {
      const [m, p] = k.split(" ");
      if (m === method && url.startsWith(p ?? "")) return json(v.body, v.status);
    }
    if (url.startsWith("/api/v1/auth/me")) return json(testMe({ permissions: opts.perms ?? allPermissions }));
    if (url.startsWith("/api/v1/ui/bootstrap")) return json(testBootstrap());
    if (url.startsWith("/api/v1/roles")) return json({ roles, permissions: allPermissions });
    if (url.startsWith("/api/v1/users?")) return json({ users, total: users.length, limit: 50, offset: 0 });
    if (method === "POST" && url === "/api/v1/users") {
      return json({ user: user({ id: "u-9", username: body.username, display_name: body.display_name, roles: body.roles, must_change_password: true }), temporary_password: TEMP }, 201);
    }
    const m = /^\/api\/v1\/users\/([^/?]+)(\/[a-z-]+)?$/.exec(url);
    if (m) {
      const u = users.find((x) => x.id === m[1]);
      if (!u) return json({ error: { code: "not_found", message: "x" } }, 404);
      if (m[2] === "/password-reset") return json({ temporary_password: TEMP });
      if (m[2] === "/disable") return json({ ...u, status: "disabled" });
      if (m[2] === "/roles") return json({ ...u, roles: body.roles });
      if (method === "PATCH") return json({ ...u, ...body });
      return json(u);
    }
    return json({ error: { code: "not_found", message: "x" } }, 404);
  });
  vi.stubGlobal("fetch", fn);
  return calls;
}

function renderAt(path: string) {
  window.history.pushState({}, "", path);
  return render(<App queryClient={createQueryClient()} />);
}

describe("users & roles", () => {
  beforeEach(async () => {
    await initI18n("id");
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    window.history.pushState({}, "", "/");
  });

  it("lists users with roles, status and the current account marked", async () => {
    api();
    renderAt("/users");
    const page = await screen.findByTestId("users-page");
    const rows = await within(page).findAllByTestId("user-row");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveTextContent(tx.list.you);
    expect(rows[1]).toHaveTextContent(tx.roles.technician);
    expect(rows[1]).toHaveTextContent(tx.status.active);
    expect(within(page).getByTestId("add-user")).toBeInTheDocument();
  });

  it("is read-only with users.read only (Auditor)", async () => {
    api({ perms: ["roles.read", "users.read"] });
    renderAt("/users");
    const [, other] = await screen.findAllByTestId("user-row");
    if (!other) throw new Error("second row missing");
    expect(within(other).getByRole("link", { name: tx.list.viewLabel.replace("{{name}}", "Teknisi Satu") })).toBeInTheDocument();
    expect(screen.queryByTestId("add-user")).not.toBeInTheDocument();
  });

  it("creates a user and shows the temporary password once", async () => {
    const calls = api();
    renderAt("/users");
    await userEvent.click(await screen.findByTestId("add-user"));
    const dialog = await screen.findByTestId("create-user-dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: tx.create.submit }));
    expect(within(dialog).getByRole("alert")).toHaveTextContent(tx.create.required);

    await userEvent.type(within(dialog).getByLabelText(tx.create.username), "teknisi.02");
    await userEvent.type(within(dialog).getByLabelText(tx.create.displayName), "Teknisi Dua");
    await userEvent.click(within(dialog).getByRole("checkbox", { name: new RegExp(tx.roles.technician) }));
    await userEvent.click(within(dialog).getByRole("button", { name: tx.create.submit }));

    expect(await within(dialog).findByTestId("temporary-password-value")).toHaveTextContent(TEMP);
    expect(within(dialog).getByRole("note")).toHaveTextContent(tx.temp.warning);
    const post = calls.find((c) => c.method === "POST" && c.url === "/api/v1/users");
    expect(post?.body).toEqual({ username: "teknisi.02", display_name: "Teknisi Dua", email: "", roles: ["technician"] });

    await userEvent.click(within(dialog).getByRole("button", { name: tx.temp.done }));
    await waitFor(() => expect(screen.queryByTestId("create-user-dialog")).not.toBeInTheDocument());
    expect(screen.queryByText(TEMP)).not.toBeInTheDocument();
    // Re-opening starts with an empty form, never the old password.
    await userEvent.click(screen.getByTestId("add-user"));
    const again = await screen.findByTestId("create-user-dialog");
    expect(within(again).getByLabelText(tx.create.username)).toHaveValue("");
    expect(within(again).queryByTestId("temporary-password")).not.toBeInTheDocument();
  });

  it("shows field messages for server validation errors", async () => {
    api({ fail: { "POST /api/v1/users": { status: 409, body: { error: { code: "username_taken", message: "x", field: "username" } } } } });
    renderAt("/users");
    await userEvent.click(await screen.findByTestId("add-user"));
    const dialog = await screen.findByTestId("create-user-dialog");
    await userEvent.type(within(dialog).getByLabelText(tx.create.username), "teknisi.01");
    await userEvent.type(within(dialog).getByLabelText(tx.create.displayName), "x");
    await userEvent.click(within(dialog).getByRole("checkbox", { name: new RegExp(tx.roles.technician) }));
    await userEvent.click(within(dialog).getByRole("button", { name: tx.create.submit }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(errors.code.username_taken);
    expect(within(dialog).getByLabelText(tx.create.username)).toHaveAttribute("aria-invalid", "true");
  });

  it("protects the own account on the detail page", async () => {
    api();
    renderAt("/users/u-1");
    const page = await screen.findByTestId("user-detail");
    expect(await within(page).findByText(tx.detail.self)).toBeInTheDocument();
    expect(within(page).queryByTestId("toggle-status")).not.toBeInTheDocument();
    expect(within(page).queryByTestId("reset-password")).not.toBeInTheDocument();
    expect(within(page).getByRole("checkbox", { name: new RegExp(tx.roles.admin) })).toBeDisabled();
  });

  it("disables a user after confirmation and resets a password", async () => {
    const calls = api();
    renderAt("/users/u-2");
    await userEvent.click(await screen.findByTestId("toggle-status"));
    const confirm = await screen.findByTestId("confirm-dialog");
    expect(confirm).toHaveTextContent(tx.detail.disableBody);
    await userEvent.click(within(confirm).getByTestId("confirm"));
    await waitFor(() => expect(calls.some((c) => c.method === "POST" && c.url === "/api/v1/users/u-2/disable")).toBe(true));
    expect(await screen.findByTestId("toggle-status")).toHaveTextContent(tx.detail.enable);

    await userEvent.click(screen.getByTestId("reset-password"));
    await userEvent.click(within(await screen.findByTestId("confirm-dialog")).getByTestId("confirm"));
    expect(await screen.findByTestId("temporary-password-value")).toHaveTextContent(TEMP);
  });

  it("saves roles", async () => {
    const calls = api();
    renderAt("/users/u-2");
    await userEvent.click(await screen.findByRole("checkbox", { name: new RegExp(tx.roles.auditor) }));
    await userEvent.click(screen.getByTestId("save-roles"));
    await waitFor(() =>
      expect(calls.find((c) => c.method === "PUT")?.body).toEqual({ roles: ["technician", "auditor"] }),
    );
  });

  it("lists roles with their permissions on the roles tab", async () => {
    api();
    renderAt("/users?tab=roles");
    const table = await screen.findByTestId("roles-table");
    expect(table).toHaveTextContent(tx.permission.users_manage);
    expect(table).toHaveTextContent(tx.rolesTab.noPermissions);
  });

  it("shows not found for an unknown user id", async () => {
    api();
    renderAt("/users/nope");
    expect(await screen.findByText(tx.detail.notFound)).toBeInTheDocument();
  });
});
