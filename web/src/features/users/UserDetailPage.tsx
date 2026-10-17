import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import { type FormEvent, type ReactNode, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { toast } from "@/components/ui/toast";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useAuth } from "@/features/auth/auth-context";
import { FormMessage } from "@/features/auth/FormMessage";
import { usePermissions } from "@/features/auth/permissions";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import {
  setKiosk,
  fetchRoles,
  fetchUser,
  PERM_ROLES_READ,
  PERM_USERS_MANAGE,
  resetMfa,
  resetPassword,
  rolesKey,
  setRoles,
  setStatus,
  SYSTEM_ROLES,
  updateProfile,
  type User,
  usersKey,
} from "./api";
import { ConfirmDialog } from "./ConfirmDialog";
import { RoleCheckboxes } from "./RoleCheckboxes";
import { SiteRolesCard } from "./SiteRolesCard";
import { TemporaryPassword } from "./TemporaryPassword";
import { RoleBadges, UserStatusBadges } from "./UserBadges";
import { useFieldError } from "./useFieldError";

const errText = (err: unknown) => (err instanceof ApiError ? err : new ApiError("unknown", 0));

/** /users/:id — profile, roles and account status of one user; editable with users.manage. */
export function UserDetailPage() {
  const { t } = useTranslation(["users", "errors", "common"]);
  const { id = "" } = useParams();
  const user = useQuery({ queryKey: [...usersKey, "detail", id], queryFn: ({ signal }) => fetchUser(id, signal) });

  return (
    <section className="max-w-5xl space-y-4" data-testid="user-detail">
      <Button asChild variant="ghost" size="sm" className="-ml-2">
        <Link to="/users">
          <ArrowLeft aria-hidden />
          {t("detail.back")}
        </Link>
      </Button>
      {user.isPending && <Skeleton className="h-64 w-full" />}
      {user.isError && (
        <p role="alert" className="text-muted-foreground">
          {user.error instanceof ApiError && user.error.code === "not_found"
            ? t("detail.notFound")
            : t(`errors:code.${toErrorCode(user.error instanceof ApiError ? user.error.code : "unknown")}`)}
        </p>
      )}
      {user.data && <UserDetail user={user.data} />}
    </section>
  );
}

function UserDetail({ user }: { user: User }) {
  const { t } = useTranslation("users");
  const perms = usePermissions();
  const { me } = useAuth();
  const canManage = perms.has(PERM_USERS_MANAGE);
  const self = me?.user.id === user.id;
  const name = user.display_name || user.username;
  return (
    <>
      <header className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h2 className="text-xl font-semibold">{name}</h2>
        <span className="font-mono text-sm text-muted-foreground">{user.username}</span>
        <UserStatusBadges user={user} />
      </header>
      {!canManage && <FormMessage tone="info">{t("detail.readOnly")}</FormMessage>}
      {canManage && self && <FormMessage tone="info">{t("detail.self")}</FormMessage>}
      <div className="grid gap-4 lg:grid-cols-2">
        {/* Keyed by the saved values: a saved change (here or elsewhere) re-initialises the form state. */}
        <ProfileCard key={`${user.display_name}|${user.email ?? ""}`} user={user} canManage={canManage} />
        <AccountCard user={user} canManage={canManage} self={self} />
        <div className="lg:col-span-2">
          <RolesCard key={user.roles.join()} user={user} canManage={canManage} self={self} />
        </div>
        <div className="lg:col-span-2">
          <SiteRolesCard key={(user.site_roles ?? []).map((r) => `${r.role}@${r.site_id}`).join()} user={user} canManage={canManage} />
        </div>
        {(user.kiosk || (user.roles.length === 1 && user.roles[0] === "viewer")) && (
          <div className="lg:col-span-2">
            <KioskCard user={user} canManage={canManage} />
          </div>
        )}
      </div>
    </>
  );
}

/** Updates both the detail and the list caches after a change. */
function useApply(user: User) {
  const qc = useQueryClient();
  return (next: User) => {
    qc.setQueryData([...usersKey, "detail", user.id], next);
    void qc.invalidateQueries({ queryKey: [...usersKey, "list"] });
    void qc.invalidateQueries({ queryKey: rolesKey });
  };
}

function ProfileCard({ user, canManage }: { user: User; canManage: boolean }) {
  const { t } = useTranslation("users");
  const id = useId();
  const apply = useApply(user);
  const [displayName, setDisplayName] = useState(user.display_name);
  const [email, setEmail] = useState(user.email ?? "");
  const { error, field, setFromError, clear } = useFieldError();
  const save = useMutation({
    mutationFn: () => updateProfile(user.id, { display_name: displayName, email }),
    onSuccess: (u) => {
      clear();
      apply(u);
      toast.success(t("detail.profileSaved"));
    },
    onError: (err) => setFromError(errText(err)),
  });
  const dirty = displayName !== user.display_name || email !== (user.email ?? "");
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate();
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("detail.profile")}</CardTitle>
        <CardDescription>{t("detail.profileDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} noValidate className="space-y-4" data-testid="profile-form">
          {error && <FormMessage tone="error">{error}</FormMessage>}
          <Row id={`${id}-username`} label={t("detail.username")}>
            <Input id={`${id}-username`} value={user.username} readOnly disabled className="font-mono" />
          </Row>
          <Row id={`${id}-name`} label={t("detail.displayName")}>
            <Input
              id={`${id}-name`}
              value={displayName}
              maxLength={128}
              disabled={!canManage}
              onChange={(e) => setDisplayName(e.target.value)}
              aria-invalid={field === "display_name" ? true : undefined}
            />
          </Row>
          <Row id={`${id}-email`} label={t("detail.email")}>
            <Input
              id={`${id}-email`}
              type="email"
              value={email}
              maxLength={254}
              disabled={!canManage}
              onChange={(e) => setEmail(e.target.value)}
              aria-invalid={field === "email" ? true : undefined}
            />
          </Row>
          {canManage && (
            <Button type="submit" disabled={!dirty || save.isPending}>
              {save.isPending ? t("detail.saving") : t("detail.saveProfile")}
            </Button>
          )}
        </form>
      </CardContent>
    </Card>
  );
}

function RolesCard({ user, canManage, self }: { user: User; canManage: boolean; self: boolean }) {
  const { t } = useTranslation("users");
  const perms = usePermissions();
  const apply = useApply(user);
  const [roles, setLocal] = useState(user.roles);
  const { error, setFromError, clear } = useFieldError();
  const all = useQuery({ queryKey: rolesKey, queryFn: ({ signal }) => fetchRoles(signal), enabled: perms.has(PERM_ROLES_READ) });
  const keys = all.data?.roles.map((r) => r.key) ?? [...SYSTEM_ROLES];
  const save = useMutation({
    mutationFn: () => setRoles(user.id, roles),
    onSuccess: (u) => {
      clear();
      apply(u);
      toast.success(t("detail.rolesSaved"));
    },
    onError: (err) => setFromError(errText(err)),
  });
  const dirty = [...roles].sort().join() !== [...user.roles].sort().join();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("detail.rolesTitle")}</CardTitle>
        <CardDescription>{t("detail.rolesDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && <FormMessage tone="error">{error}</FormMessage>}
        {canManage ? (
          <>
            {/* Own Admin role is locked: removing it would lock the caller out (the API refuses it too). */}
            <RoleCheckboxes roles={keys} value={roles} onChange={setLocal} locked={self && user.roles.includes("admin") ? ["admin"] : []} />
            <Button
              disabled={!dirty || (roles.length === 0 && (user.site_roles ?? []).length === 0) || save.isPending}
              onClick={() => save.mutate()}
              data-testid="save-roles"
            >
              {save.isPending ? t("detail.saving") : t("detail.saveRoles")}
            </Button>
          </>
        ) : (
          <RoleBadges roles={user.roles} />
        )}
      </CardContent>
    </Card>
  );
}

function AccountCard({ user, canManage, self }: { user: User; canManage: boolean; self: boolean }) {
  const { t } = useTranslation(["users", "errors"]);
  const fmt = useFormatters();
  const apply = useApply(user);
  const qc = useQueryClient();
  const [confirm, setConfirm] = useState<null | "status" | "reset" | "mfa">(null);
  const [temp, setTemp] = useState<string | null>(null);
  const name = user.display_name || user.username;
  const disabling = user.status === "active";
  const status = useMutation({
    mutationFn: () => setStatus(user.id, disabling ? "disabled" : "active"),
    onSuccess: (u) => {
      apply(u);
      setConfirm(null);
      toast.success(disabling ? t("detail.disabled") : t("detail.enabled"));
    },
    onError: (err) => {
      setConfirm(null);
      toast.error(t(`errors:code.${toErrorCode(errText(err).code)}`));
    },
  });
  const reset = useMutation({
    mutationFn: () => resetPassword(user.id),
    onSuccess: (res) => {
      setConfirm(null);
      setTemp(res.temporary_password);
      // Reset sets must_change_password and clears the lockout: refresh the badges.
      void qc.invalidateQueries({ queryKey: usersKey });
    },
    onError: (err) => {
      setConfirm(null);
      toast.error(t(`errors:code.${toErrorCode(errText(err).code)}`));
    },
  });
  const mfa = useMutation({
    mutationFn: () => resetMfa(user.id),
    onSuccess: () => {
      setConfirm(null);
      toast.success(t("detail.mfaResetDone"));
      void qc.invalidateQueries({ queryKey: usersKey });
    },
    onError: (err) => {
      setConfirm(null);
      toast.error(t(`errors:code.${toErrorCode(errText(err).code)}`));
    },
  });
  const when = (v: string | null) => (v ? fmt.dateTime(v) : t("list.never"));

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("detail.account")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="text-muted-foreground">{t("list.status")}</dt>
          <dd>
            <UserStatusBadges user={user} />
          </dd>
          <dt className="text-muted-foreground">{t("detail.mfa")}</dt>
          <dd data-testid="user-mfa">{user.mfa_enabled ? t("detail.mfaOn") : t("status.mfaOff")}</dd>
          <dt className="text-muted-foreground">{t("detail.lastLogin")}</dt>
          <dd>{when(user.last_login_at)}</dd>
          <dt className="text-muted-foreground">{t("detail.created")}</dt>
          <dd>{when(user.created_at)}</dd>
          <dt className="text-muted-foreground">{t("detail.updated")}</dt>
          <dd>{when(user.updated_at)}</dd>
        </dl>
        {canManage && !self && (
          <div className="flex flex-wrap gap-2">
            <Button variant={disabling ? "destructive" : "default"} onClick={() => setConfirm("status")} data-testid="toggle-status">
              {disabling ? t("detail.disable") : t("detail.enable")}
            </Button>
            <Button variant="outline" onClick={() => setConfirm("reset")} data-testid="reset-password">
              {t("detail.reset")}
            </Button>
            {user.mfa_enabled && (
              <Button variant="outline" onClick={() => setConfirm("mfa")} data-testid="reset-mfa">
                {t("detail.mfaReset")}
              </Button>
            )}
          </div>
        )}
      </CardContent>
      <ConfirmDialog
        open={confirm === "status"}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={disabling ? t("detail.disableTitle", { name }) : t("detail.enableTitle", { name })}
        body={disabling ? t("detail.disableBody") : t("detail.enableBody")}
        busy={status.isPending}
        destructive={disabling}
        onConfirm={() => status.mutate()}
      />
      <ConfirmDialog
        open={confirm === "reset"}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t("detail.resetTitle", { name })}
        body={t("detail.resetBody")}
        busy={reset.isPending}
        onConfirm={() => reset.mutate()}
      />
      <ConfirmDialog
        open={confirm === "mfa"}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={t("detail.mfaResetTitle", { name })}
        body={t("detail.mfaResetBody")}
        busy={mfa.isPending}
        destructive
        onConfirm={() => mfa.mutate()}
      />
      <Dialog open={temp !== null} onOpenChange={(o) => !o && setTemp(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("temp.title")}</DialogTitle>
          </DialogHeader>
          {temp && <TemporaryPassword username={user.username} password={temp} onDone={() => setTemp(null)} />}
        </DialogContent>
      </Dialog>
    </Card>
  );
}

function Row({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children}
    </div>
  );
}

/** Monitoring/kiosk mode of a Viewer-only account (v0.3.7, D-068). */
function KioskCard({ user, canManage }: { user: User; canManage: boolean }) {
  const { t } = useTranslation(["users", "errors"]);
  const apply = useApply(user);
  const [confirming, setConfirming] = useState(false);
  const change = useMutation({
    mutationFn: (on: boolean) => setKiosk(user.id, on),
    onSuccess: (u) => {
      apply(u);
      setConfirming(false);
      toast.success(u.kiosk ? t("kiosk.enabled") : t("kiosk.disabled"));
    },
    onError: (err) => {
      setConfirming(false);
      toast.error(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`));
    },
  });
  return (
    <Card data-testid="kiosk-card" data-kiosk={user.kiosk === true}>
      <CardHeader>
        <CardTitle>{t("kiosk.title")}</CardTitle>
        <CardDescription>{t("kiosk.description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-center gap-3">
        <span className="text-sm font-medium" data-testid="kiosk-state">
          {user.kiosk ? t("kiosk.on") : t("kiosk.off")}
        </span>
        {canManage && (
          <Button
            variant={user.kiosk ? "outline" : "default"}
            size="sm"
            className="sm:ml-auto"
            disabled={change.isPending}
            onClick={() => setConfirming(true)}
            data-testid="kiosk-toggle"
          >
            {user.kiosk ? t("kiosk.turnOff") : t("kiosk.turnOn")}
          </Button>
        )}
      </CardContent>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={user.kiosk ? t("kiosk.turnOff") : t("kiosk.turnOn")}
        body={user.kiosk ? t("kiosk.confirmOff") : t("kiosk.confirmOn")}
        busy={change.isPending}
        onConfirm={() => change.mutate(!user.kiosk)}
      />
    </Card>
  );
}
