import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Plus, Search } from "lucide-react";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAuth } from "@/features/auth/auth-context";
import { usePermissions } from "@/features/auth/permissions";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import {
  fetchRoles,
  fetchUsers,
  PAGE_SIZE,
  PERM_ROLES_READ,
  PERM_USERS_MANAGE,
  rolesKey,
  SYSTEM_ROLES,
  type User,
  type UserFilter,
  usersKey,
} from "./api";
import { CreateUserDialog } from "./CreateUserDialog";
import { useUserText } from "./text";
import { RoleBadges, UserStatusBadges } from "./UserBadges";

const selectClass =
  "h-9 rounded-md border border-input bg-background px-3 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm";

/** Users & Roles: user list (search, filters, paging) and, with roles.read, the roles tab (?tab=roles). */
export function UsersPage() {
  const { t } = useTranslation("users");
  const perms = usePermissions();
  const [params, setParams] = useSearchParams();
  const tab = params.get("tab") === "roles" && perms.has(PERM_ROLES_READ) ? "roles" : "users";
  return (
    <section className="max-w-6xl space-y-4" data-testid="users-page">
      <p className="text-muted-foreground">{t("list.description")}</p>
      <Tabs value={tab} onValueChange={(v) => setParams(v === "roles" ? { tab: "roles" } : {}, { replace: true })}>
        <TabsList>
          <TabsTrigger value="users">{t("tabs.users")}</TabsTrigger>
          {perms.has(PERM_ROLES_READ) && <TabsTrigger value="roles">{t("tabs.roles")}</TabsTrigger>}
        </TabsList>
        <TabsContent value="users" className="pt-2">
          <UserList />
        </TabsContent>
        {perms.has(PERM_ROLES_READ) && (
          <TabsContent value="roles" className="pt-2">
            <RolesTable />
          </TabsContent>
        )}
      </Tabs>
    </section>
  );
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = window.setTimeout(() => setV(value), ms);
    return () => window.clearTimeout(id);
  }, [value, ms]);
  return v;
}

function UserList() {
  const { t } = useTranslation(["users", "errors", "common"]);
  const id = useId();
  const perms = usePermissions();
  const canManage = perms.has(PERM_USERS_MANAGE);
  const { me } = useAuth();
  const fmt = useFormatters();
  const { roleLabel } = useUserText();
  const [q, setQ] = useState("");
  const [status, setStatus] = useState<UserFilter["status"]>("");
  const [role, setRole] = useState("");
  const [offset, setOffset] = useState(0);
  const [creating, setCreating] = useState(false);
  const query = useDebounced(q, 300);
  const filter: UserFilter = { q: query, status, role, offset };
  const list = useQuery({
    queryKey: [...usersKey, "list", filter],
    queryFn: ({ signal }) => fetchUsers(filter, signal),
    placeholderData: keepPreviousData,
  });
  const roles = useQuery({ queryKey: rolesKey, queryFn: ({ signal }) => fetchRoles(signal), enabled: perms.has(PERM_ROLES_READ) });
  const roleKeys = roles.data?.roles.map((r) => r.key) ?? [...SYSTEM_ROLES];

  // Any filter change starts again at the first page.
  const filterBy =
    <T,>(set: (v: T) => void) =>
    (v: T) => {
      set(v);
      setOffset(0);
    };

  const name = (u: User) => u.display_name || u.username;
  const lastLogin = (u: User) => (u.last_login_at ? fmt.dateTime(u.last_login_at) : t("list.never"));
  const action = (u: User) => (
    <Button asChild variant="outline" size="sm">
      <Link to={`/users/${u.id}`} aria-label={t(canManage ? "list.manageLabel" : "list.viewLabel", { name: name(u) })}>
        {canManage ? t("list.manage") : t("list.view")}
      </Link>
    </Button>
  );
  const you = (u: User) =>
    u.id === me?.user.id && (
      <Badge variant="outline" className="ml-2">
        {t("list.you")}
      </Badge>
    );

  const data = list.data;
  const filtered = query.trim() !== "" || status !== "" || role !== "";

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-2 md:flex-row md:items-center">
        <div className="relative md:max-w-xs md:flex-1">
          <Search className="pointer-events-none absolute top-2.5 left-3 size-4 text-muted-foreground" aria-hidden />
          <Input
            type="search"
            className="pl-9"
            placeholder={t("list.search")}
            aria-label={t("list.searchLabel")}
            maxLength={64}
            value={q}
            onChange={(e) => filterBy(setQ)(e.target.value)}
            data-testid="users-search"
          />
        </div>
        <div className="flex gap-2">
          <select
            id={`${id}-status`}
            className={selectClass}
            aria-label={t("list.statusFilter")}
            value={status}
            onChange={(e) => filterBy(setStatus)(e.target.value as UserFilter["status"])}
          >
            <option value="">{t("list.allStatuses")}</option>
            <option value="active">{t("status.active")}</option>
            <option value="disabled">{t("status.disabled")}</option>
          </select>
          <select id={`${id}-role`} className={selectClass} aria-label={t("list.roleFilter")} value={role} onChange={(e) => filterBy(setRole)(e.target.value)}>
            <option value="">{t("list.allRoles")}</option>
            {roleKeys.map((r) => (
              <option key={r} value={r}>
                {roleLabel(r)}
              </option>
            ))}
          </select>
        </div>
        {canManage && (
          <Button className="md:ml-auto" onClick={() => setCreating(true)} data-testid="add-user">
            <Plus aria-hidden />
            {t("list.add")}
          </Button>
        )}
      </div>

      {list.isPending && <Skeleton className="h-40 w-full" />}
      {list.isError && (
        <div role="alert" className="space-y-2">
          <p className="text-sm text-muted-foreground">
            {t(`errors:code.${toErrorCode(list.error instanceof ApiError ? list.error.code : "unknown")}`)}
          </p>
          <Button variant="outline" size="sm" onClick={() => void list.refetch()}>
            {t("common:app.retry")}
          </Button>
        </div>
      )}
      {data && data.users.length === 0 && (
        <p className="rounded-xl border border-dashed p-8 text-center text-muted-foreground" data-testid="users-empty">
          {filtered ? t("list.emptyFiltered") : t("list.empty")}
        </p>
      )}
      {data && data.users.length > 0 && (
        <>
          {/* ≥ md: table */}
          <div className="hidden rounded-xl border md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("list.name")}</TableHead>
                  <TableHead>{t("list.roles")}</TableHead>
                  <TableHead>{t("list.status")}</TableHead>
                  <TableHead>{t("list.lastLogin")}</TableHead>
                  <TableHead className="text-right">
                    <span className="sr-only">{t("list.actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.users.map((u) => (
                  <TableRow key={u.id} data-testid="user-row" data-username={u.username}>
                    <TableCell>
                      <span className="font-medium">{name(u)}</span>
                      {you(u)}
                      <span className="block font-mono text-xs text-muted-foreground">{u.username}</span>
                    </TableCell>
                    <TableCell>
                      <RoleBadges roles={u.roles} />
                    </TableCell>
                    <TableCell>
                      <UserStatusBadges user={u} />
                    </TableCell>
                    <TableCell className="whitespace-nowrap">{lastLogin(u)}</TableCell>
                    <TableCell className="text-right">{action(u)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          {/* < md: cards */}
          <ul className="space-y-3 md:hidden">
            {data.users.map((u) => (
              <li key={u.id} className="space-y-2 rounded-xl border bg-card p-4" data-testid="user-card" data-username={u.username}>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="truncate font-medium">
                      {name(u)}
                      {you(u)}
                    </p>
                    <p className="truncate font-mono text-xs text-muted-foreground">{u.username}</p>
                  </div>
                  {action(u)}
                </div>
                <RoleBadges roles={u.roles} />
                <UserStatusBadges user={u} />
                <p className="text-xs text-muted-foreground">
                  {t("list.lastLogin")}: {lastLogin(u)}
                </p>
              </li>
            ))}
          </ul>
          <nav aria-label={t("list.pagination")} className="flex items-center justify-between gap-2 text-sm">
            <span className="text-muted-foreground" data-testid="users-range">
              {t("list.range", { from: data.offset + 1, to: data.offset + data.users.length, total: data.total })}
            </span>
            <span className="flex gap-2">
              <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
                {t("list.prev")}
              </Button>
              <Button variant="outline" size="sm" disabled={offset + PAGE_SIZE >= data.total} onClick={() => setOffset(offset + PAGE_SIZE)}>
                {t("list.next")}
              </Button>
            </span>
          </nav>
        </>
      )}
      {canManage && <CreateUserDialog open={creating} onOpenChange={setCreating} roles={roleKeys} />}
    </div>
  );
}

function RolesTable() {
  const { t } = useTranslation(["users", "errors", "common"]);
  const { roleLabel, roleDescription, permissionLabel } = useUserText();
  const roles = useQuery({ queryKey: rolesKey, queryFn: ({ signal }) => fetchRoles(signal) });
  if (roles.isPending) return <Skeleton className="h-40 w-full" />;
  if (roles.isError) {
    return (
      <p role="alert" className="text-sm text-muted-foreground">
        {t(`errors:code.${toErrorCode(roles.error instanceof ApiError ? roles.error.code : "unknown")}`)}
      </p>
    );
  }
  const perms = (list: string[]) =>
    list.length === 0 ? (
      <span className="text-muted-foreground">{t("rolesTab.noPermissions")}</span>
    ) : (
      <span className="flex flex-wrap gap-1">
        {list.map((p) => (
          <Badge key={p} variant="outline" title={p}>
            {permissionLabel(p)}
          </Badge>
        ))}
      </span>
    );
  return (
    <div className="space-y-3" data-testid="roles-table">
      <p className="text-sm text-muted-foreground">{t("rolesTab.description")}</p>
      <div className="hidden rounded-xl border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("rolesTab.role")}</TableHead>
              <TableHead>{t("rolesTab.permissions")}</TableHead>
              <TableHead className="text-right">{t("rolesTab.users")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {roles.data.roles.map((r) => (
              <TableRow key={r.key} data-role={r.key}>
                <TableCell className="align-top">
                  <span className="font-medium">{roleLabel(r.key)}</span>
                  <span className="block max-w-xs text-xs whitespace-normal text-muted-foreground">{roleDescription(r.key)}</span>
                </TableCell>
                <TableCell className="align-top whitespace-normal">{perms(r.permissions)}</TableCell>
                <TableCell className="text-right align-top tabular-nums">{r.user_count}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <ul className="space-y-3 md:hidden">
        {roles.data.roles.map((r) => (
          <li key={r.key} className="space-y-2 rounded-xl border bg-card p-4" data-role={r.key}>
            <div className="flex items-baseline justify-between gap-2">
              <span className="font-medium">{roleLabel(r.key)}</span>
              <span className="text-sm text-muted-foreground tabular-nums">
                {t("rolesTab.users")}: {r.user_count}
              </span>
            </div>
            <p className="text-xs text-muted-foreground">{roleDescription(r.key)}</p>
            <div className="text-sm">{perms(r.permissions)}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}
