import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Pencil, Plus, ShieldAlert, ShieldCheck, Trash2 } from "lucide-react";
import { type FormEvent, useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { dcimStatusStyle } from "@/components/domain/status";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { toast } from "@/components/ui/toast";
import { useFormatters } from "@/features/bootstrap/regional";
import { ConfirmDialog } from "@/features/users/ConfirmDialog";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";

import {
  addAllowed,
  ALLOWLIST_MODES,
  type AllowlistEntry,
  allowlistKey,
  type AllowlistMode,
  type AllowlistState,
  confirmEnforce,
  fetchAllowlist,
  relabelAllowed,
  removeAllowed,
  setAllowlistMode,
} from "./api";

function useErrorText() {
  const { t } = useTranslation("errors");
  return (err: unknown) => t(`code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`);
}

/** Settings › Allowed IP (settings.manage, v0.3.6): who may use the web panel, by IP address or network. */
export function AllowedIpPage() {
  const { t } = useTranslation("settings");
  const errorText = useErrorText();
  const query = useQuery({ queryKey: allowlistKey, queryFn: ({ signal }) => fetchAllowlist(signal) });
  if (query.isPending) return <Skeleton className="h-64 w-full" />;
  if (query.isError) {
    return (
      <p role="alert" className="text-sm text-muted-foreground">
        {errorText(query.error)}
      </p>
    );
  }
  return (
    <div className="space-y-4" data-testid="settings-allowlist">
      <p className="text-muted-foreground">{t("allowlist.description")}</p>
      <PendingBanner state={query.data} fetchedAt={query.dataUpdatedAt} />
      <YourIp state={query.data} />
      <ModeCard state={query.data} />
      <Entries state={query.data} />
      <AlwaysPublic paths={query.data.always_public} />
    </div>
  );
}

function useSave() {
  const qc = useQueryClient();
  return (st: AllowlistState) => qc.setQueryData(allowlistKey, st);
}

function YourIp({ state }: { state: AllowlistState }) {
  const { t } = useTranslation("settings");
  const errorText = useErrorText();
  const save = useSave();
  const add = useMutation({
    mutationFn: () => addAllowed(state.your_ip, t("allowlist.myIpLabel")),
    onSuccess: (st) => {
      save(st);
      toast.success(t("allowlist.added", { cidr: state.your_ip }));
    },
    onError: (err) => toast.error(errorText(err)),
  });
  const Icon = state.your_ip_in_list ? ShieldCheck : ShieldAlert;
  return (
    <Card data-testid="allowlist-your-ip" data-covered={state.your_ip_in_list}>
      <CardContent className="flex flex-wrap items-center gap-3">
        <div className="min-w-0">
          <p className="text-sm text-muted-foreground">{t("allowlist.yourIp")}</p>
          <p className="font-mono text-lg font-medium break-all">{state.your_ip || t("allowlist.unknownIp")}</p>
        </div>
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium",
            state.your_ip_in_list ? dcimStatusStyle.normal.badge : dcimStatusStyle.warning.badge,
          )}
        >
          <Icon className="size-3.5" aria-hidden />
          {state.your_ip_in_list ? t("allowlist.covered") : t("allowlist.notCovered")}
        </span>
        {!state.your_ip_in_list && state.your_ip && (
          <Button variant="outline" size="sm" className="sm:ml-auto" disabled={add.isPending} onClick={() => add.mutate()} data-testid="allowlist-add-mine">
            <Plus aria-hidden />
            {t("allowlist.addMine")}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

function ModeCard({ state }: { state: AllowlistState }) {
  const { t } = useTranslation("settings");
  const errorText = useErrorText();
  const save = useSave();
  const id = useId();
  const [mode, setMode] = useState<AllowlistMode>(state.mode);
  const [confirming, setConfirming] = useState(false);
  const change = useMutation({
    mutationFn: (m: AllowlistMode) => setAllowlistMode(m),
    onSuccess: (st) => {
      save(st);
      setConfirming(false);
      toast.success(t("allowlist.modeSaved", { mode: t(`allowlist.mode.${st.mode}`) }));
    },
    onError: (err) => {
      setConfirming(false);
      toast.error(errorText(err));
    },
  });
  const enforceBlocked = mode === "enforce" && !state.your_ip_in_list;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (mode === state.mode) return;
    if (mode === "enforce") setConfirming(true);
    else change.mutate(mode);
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("allowlist.modeTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-3" data-testid="allowlist-mode">
          <fieldset className="space-y-2">
            <legend className="sr-only">{t("allowlist.modeTitle")}</legend>
            {ALLOWLIST_MODES.map((m) => (
              <label
                key={m}
                htmlFor={`${id}-${m}`}
                className={cn("flex cursor-pointer gap-3 rounded-md border p-3", mode === m && "border-primary bg-accent/40")}
              >
                <input
                  id={`${id}-${m}`}
                  type="radio"
                  name={`${id}-mode`}
                  value={m}
                  checked={mode === m}
                  onChange={() => setMode(m)}
                  className="mt-1"
                  data-testid={`allowlist-mode-${m}`}
                />
                <span>
                  <span className="block text-sm font-medium">
                    {t(`allowlist.mode.${m}`)}
                    {state.mode === m && <span className="ml-2 text-xs font-normal text-muted-foreground">{t("allowlist.current")}</span>}
                  </span>
                  <span className="block text-xs text-muted-foreground">{t(`allowlist.modeHelp.${m}`)}</span>
                </span>
              </label>
            ))}
          </fieldset>
          {enforceBlocked && (
            <p role="alert" className="text-sm text-status-warning-fg" data-testid="allowlist-guard">
              {t("allowlist.guard")}
            </p>
          )}
          <Button type="submit" disabled={mode === state.mode || enforceBlocked || change.isPending} data-testid="allowlist-mode-save">
            {t("allowlist.saveMode")}
          </Button>
        </form>
      </CardContent>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("allowlist.enforceTitle")}
        body={t("allowlist.enforceBody", { seconds: state.confirm_window_seconds })}
        busy={change.isPending}
        destructive
        onConfirm={() => change.mutate("enforce")}
      />
    </Card>
  );
}

/** Countdown of an unconfirmed enforce switch with the "Keep" button (auto-rollback to monitor otherwise). */
function PendingBanner({ state, fetchedAt }: { state: AllowlistState; fetchedAt: number }) {
  const { t } = useTranslation("settings");
  const errorText = useErrorText();
  const save = useSave();
  const qc = useQueryClient();
  // Remaining time from the server clock at fetch time, so a skewed client clock does not matter.
  const remainingAtFetch = state.pending_until ? new Date(state.pending_until).getTime() - new Date(state.server_time).getTime() : 0;
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!state.pending_until) return;
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [state.pending_until]);
  const left = Math.max(0, Math.ceil((remainingAtFetch - (now - fetchedAt)) / 1000));
  useEffect(() => {
    if (state.pending_until && left === 0) void qc.invalidateQueries({ queryKey: allowlistKey });
  }, [left, state.pending_until, qc]);
  const keep = useMutation({
    mutationFn: confirmEnforce,
    onSuccess: (st) => {
      save(st);
      toast.success(t("allowlist.kept"));
    },
    onError: (err) => toast.error(errorText(err)),
  });
  if (!state.pending_until || state.mode !== "enforce") return null;
  const mm = Math.floor(left / 60);
  const ss = String(left % 60).padStart(2, "0");
  return (
    <div role="status" className={cn("flex flex-wrap items-center gap-3 rounded-xl border p-4", dcimStatusStyle.warning.badge)} data-testid="allowlist-pending">
      <p className="text-sm">
        {t("allowlist.pending", { time: `${mm}:${ss}` })}
      </p>
      <Button size="sm" className="sm:ml-auto" disabled={keep.isPending || left === 0} onClick={() => keep.mutate()} data-testid="allowlist-keep">
        {t("allowlist.keep")}
      </Button>
    </div>
  );
}

function Entries({ state }: { state: AllowlistState }) {
  const { t } = useTranslation("settings");
  const errorText = useErrorText();
  const fmt = useFormatters();
  const save = useSave();
  const id = useId();
  const [cidr, setCidr] = useState("");
  const [label, setLabel] = useState("");
  const [fieldErr, setFieldErr] = useState("");
  const [editing, setEditing] = useState<AllowlistEntry | null>(null);
  const [removing, setRemoving] = useState<AllowlistEntry | null>(null);
  const add = useMutation({
    mutationFn: () => addAllowed(cidr, label),
    onSuccess: (st) => {
      save(st);
      toast.success(t("allowlist.added", { cidr }));
      setCidr("");
      setLabel("");
      setFieldErr("");
    },
    onError: (err) => {
      if (err instanceof ApiError && (err.code === "validation_failed" || err.code === "duplicate_network")) setFieldErr(errorText(err));
      else toast.error(errorText(err));
    },
  });
  const remove = useMutation({
    mutationFn: (e: AllowlistEntry) => removeAllowed(e.id),
    onSuccess: (st, e) => {
      save(st);
      setRemoving(null);
      toast.success(t("allowlist.removed", { cidr: e.cidr }));
    },
    onError: (err) => {
      setRemoving(null);
      toast.error(errorText(err));
    },
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    add.mutate();
  };
  const actions = (e: AllowlistEntry) => (
    <span className="flex justify-end gap-1">
      <Button variant="ghost" size="icon" onClick={() => setEditing(e)} aria-label={t("allowlist.editLabel", { cidr: e.cidr })}>
        <Pencil aria-hidden />
      </Button>
      <Button variant="ghost" size="icon" onClick={() => setRemoving(e)} aria-label={t("allowlist.removeLabel", { cidr: e.cidr })} data-testid="allowlist-remove">
        <Trash2 aria-hidden />
      </Button>
    </span>
  );
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("allowlist.listTitle", { count: state.entries.length })}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <form onSubmit={submit} className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end" data-testid="allowlist-add" noValidate>
          <div className="space-y-1">
            <label htmlFor={`${id}-cidr`} className="text-sm font-medium">
              {t("allowlist.cidr")}
            </label>
            <Input
              id={`${id}-cidr`}
              value={cidr}
              onChange={(e) => setCidr(e.target.value)}
              placeholder={t("allowlist.cidrPlaceholder")}
              maxLength={64}
              aria-invalid={fieldErr ? true : undefined}
              aria-describedby={`${id}-cidr-hint`}
              className="font-mono"
              data-testid="allowlist-cidr"
            />
          </div>
          <div className="space-y-1">
            <label htmlFor={`${id}-label`} className="text-sm font-medium">
              {t("allowlist.label")}
            </label>
            <Input id={`${id}-label`} value={label} onChange={(e) => setLabel(e.target.value)} maxLength={100} placeholder={t("allowlist.labelPlaceholder")} />
          </div>
          <Button type="submit" disabled={!cidr.trim() || add.isPending} data-testid="allowlist-add-submit">
            <Plus aria-hidden />
            {t("allowlist.add")}
          </Button>
          <p id={`${id}-cidr-hint`} className={cn("text-xs sm:col-span-3", fieldErr ? "text-destructive" : "text-muted-foreground")} role={fieldErr ? "alert" : undefined}>
            {fieldErr || t("allowlist.cidrHint")}
          </p>
        </form>
        {state.entries.length === 0 ? (
          <p className="rounded-xl border border-dashed p-6 text-center text-sm text-muted-foreground" data-testid="allowlist-empty">
            {t("allowlist.empty")}
          </p>
        ) : (
          <>
            <div className="hidden rounded-xl border md:block">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("allowlist.cidr")}</TableHead>
                    <TableHead>{t("allowlist.label")}</TableHead>
                    <TableHead>{t("allowlist.added_at")}</TableHead>
                    <TableHead className="text-right">
                      <span className="sr-only">{t("allowlist.actions")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {state.entries.map((e) => (
                    <TableRow key={e.id} data-testid="allowlist-row" data-cidr={e.cidr}>
                      <TableCell className="font-mono">{e.cidr}</TableCell>
                      <TableCell className="max-w-64 truncate" title={e.label}>
                        {e.label}
                      </TableCell>
                      <TableCell className="whitespace-nowrap">{fmt.dateTime(e.created_at)}</TableCell>
                      <TableCell className="text-right">{actions(e)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <ul className="space-y-3 md:hidden">
              {state.entries.map((e) => (
                <li key={e.id} className="flex items-start justify-between gap-2 rounded-xl border bg-card p-3" data-testid="allowlist-card">
                  <div className="min-w-0">
                    <p className="font-mono break-all">{e.cidr}</p>
                    {e.label && <p className="truncate text-sm text-muted-foreground">{e.label}</p>}
                    <p className="text-xs text-muted-foreground">{fmt.dateTime(e.created_at)}</p>
                  </div>
                  {actions(e)}
                </li>
              ))}
            </ul>
          </>
        )}
      </CardContent>
      <LabelDialog entry={editing} onClose={() => setEditing(null)} />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(o) => !o && setRemoving(null)}
        title={t("allowlist.removeTitle", { cidr: removing?.cidr ?? "" })}
        body={t(state.mode === "enforce" ? "allowlist.removeBodyEnforce" : "allowlist.removeBody")}
        busy={remove.isPending}
        destructive
        onConfirm={() => removing && remove.mutate(removing)}
      />
    </Card>
  );
}

function LabelDialog({ entry, onClose }: { entry: AllowlistEntry | null; onClose: () => void }) {
  const { t } = useTranslation(["settings", "users"]);
  const errorText = useErrorText();
  const save = useSave();
  const id = useId();
  const [label, setLabel] = useState("");
  const [shownFor, setShownFor] = useState<string | null>(null);
  if (entry && shownFor !== entry.id) {
    setShownFor(entry.id);
    setLabel(entry.label);
  }
  const update = useMutation({
    mutationFn: () => relabelAllowed(entry?.id ?? "", label),
    onSuccess: (st) => {
      save(st);
      onClose();
      setShownFor(null);
    },
    onError: (err) => toast.error(errorText(err)),
  });
  return (
    <Dialog
      open={entry !== null}
      onOpenChange={(o) => {
        if (!o) {
          onClose();
          setShownFor(null);
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("allowlist.editTitle", { cidr: entry?.cidr ?? "" })}</DialogTitle>
          <DialogDescription>{t("allowlist.editBody")}</DialogDescription>
        </DialogHeader>
        <label htmlFor={`${id}-l`} className="text-sm font-medium">
          {t("allowlist.label")}
        </label>
        <Input id={`${id}-l`} value={label} onChange={(e) => setLabel(e.target.value)} maxLength={100} />
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {t("users:detail.cancel")}
          </Button>
          <Button disabled={update.isPending} onClick={() => update.mutate()}>
            {t("allowlist.saveLabel")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function AlwaysPublic({ paths }: { paths: string[] }) {
  const { t } = useTranslation("settings");
  return (
    <div className="rounded-xl border p-4 text-sm" data-testid="allowlist-always-public">
      <p className="font-medium">{t("allowlist.alwaysPublic")}</p>
      <ul className="mt-1 list-disc pl-5 text-muted-foreground">
        {paths.map((p) => (
          <li key={p} className="font-mono">
            {p}
          </li>
        ))}
      </ul>
      <p className="mt-2 text-xs text-muted-foreground">{t("allowlist.alwaysPublicHelp")}</p>
    </div>
  );
}
