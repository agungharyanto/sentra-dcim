import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Download, RotateCcw, ShieldAlert, ShieldCheck, ShieldQuestion } from "lucide-react";
import { type FormEvent, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { dcimStatusStyle } from "@/components/domain/status";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { toast } from "@/components/ui/toast";
import { usePermissions } from "@/features/auth/permissions";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";
import { zonedLocalToUtc } from "@/lib/format";
import { cn } from "@/lib/utils";

import {
  actionPrefixes,
  type AuditEvent,
  type AuditFilter,
  type AuditResult,
  auditKey,
  DETAIL_TIME,
  emptyFilter,
  exportUrl,
  fetchActions,
  fetchEvents,
  fetchIntegrity,
  PERM_AUDIT_EXPORT,
  RESULTS,
  verifyNow,
} from "./api";

const selectClass =
  "h-9 w-full rounded-md border border-input bg-background px-3 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm";

const resultStyle: Record<AuditResult, string> = {
  success: dcimStatusStyle.normal.badge,
  failure: dcimStatusStyle.warning.badge,
  denied: dcimStatusStyle.critical.badge,
};

function ResultBadge({ result }: { result: AuditResult }) {
  const { t } = useTranslation("audit");
  return (
    <span
      data-result={result}
      className={cn("inline-flex w-fit items-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap", resultStyle[result])}
    >
      {t(`result.${result}`)}
    </span>
  );
}

function errorText(t: (k: `errors:code.${ReturnType<typeof toErrorCode>}`) => string, err: unknown) {
  return t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`);
}

/** Audit Log: integrity status, filters, keyset-paginated events (newest first), details and CSV export. */
export function AuditPage() {
  const { t } = useTranslation("audit");
  const [filter, setFilter] = useState<AuditFilter>(emptyFilter);
  const [selected, setSelected] = useState<AuditEvent | null>(null);
  return (
    <section className="max-w-6xl space-y-4" data-testid="audit-page">
      <p className="text-muted-foreground">{t("description")}</p>
      <IntegrityCard />
      <Filters onApply={setFilter} current={filter} />
      <EventList filter={filter} onSelect={setSelected} />
      <EventDetail event={selected} onClose={() => setSelected(null)} />
    </section>
  );
}

function IntegrityCard() {
  const { t } = useTranslation(["audit", "errors"]);
  const fmt = useFormatters();
  const qc = useQueryClient();
  const integrity = useQuery({ queryKey: [...auditKey, "integrity"], queryFn: ({ signal }) => fetchIntegrity(signal) });
  const verify = useMutation({
    mutationFn: verifyNow,
    onSuccess: (res) => {
      qc.setQueryData([...auditKey, "integrity"], { last: res, running: false });
      void qc.invalidateQueries({ queryKey: [...auditKey, "events"] }); // the verification itself is an event
      if (res.ok) toast.success(t("integrity.verifyDone"));
      else toast.error(t("integrity.verifyBroken"));
    },
    onError: (err) => {
      toast.error(err instanceof ApiError && err.code === "verification_running" ? t("integrity.busy") : errorText(t, err));
    },
  });
  const last = integrity.data?.last ?? null;
  const running = verify.isPending || integrity.data?.running === true;
  const state = last === null ? "unknown" : last.ok ? "intact" : "broken";
  const Icon = state === "intact" ? ShieldCheck : state === "broken" ? ShieldAlert : ShieldQuestion;
  const tone =
    state === "intact" ? dcimStatusStyle.normal.badge : state === "broken" ? dcimStatusStyle.critical.badge : dcimStatusStyle.offline.badge;
  return (
    <Card data-testid="audit-integrity" data-state={integrity.isPending ? "loading" : state}>
      <CardHeader className="flex flex-row flex-wrap items-center justify-between gap-2">
        <CardTitle className="text-base">{t("integrity.title")}</CardTitle>
        <Button variant="outline" size="sm" disabled={running} onClick={() => verify.mutate()} data-testid="audit-verify">
          {running ? t("integrity.running") : t("integrity.verify")}
        </Button>
      </CardHeader>
      <CardContent className="space-y-1 text-sm">
        {integrity.isPending ? (
          <Skeleton className="h-10 w-full" />
        ) : integrity.isError ? (
          <p role="alert" className="text-muted-foreground">
            {errorText(t, integrity.error)}
          </p>
        ) : (
          <>
            <span className={cn("inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium", tone)}>
              <Icon className="size-3.5" aria-hidden />
              {t(`integrity.${state}`)}
            </span>
            {last === null ? (
              <p className="text-muted-foreground">{t("integrity.unknownBody")}</p>
            ) : (
              <>
                <p className="text-muted-foreground">
                  {t("integrity.verifiedAt", { time: fmt.dateTime(last.verified_at, { dateStyle: "medium", timeStyle: "medium" }) })}
                  {" · "}
                  {t("integrity.checked", { count: last.checked, head: last.head_seq })}
                </p>
                {last.checkpoint_seq > 0 && <p className="text-muted-foreground">{t("integrity.checkpoint", { seq: last.checkpoint_seq })}</p>}
                {!last.ok && (
                  <p role="alert" className="font-medium text-status-critical-fg" data-testid="audit-broken">
                    {t("integrity.brokenAt", { seq: last.broken_seq, reason: last.reason ?? "" })}
                  </p>
                )}
              </>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function Field({ htmlFor, label, children }: { htmlFor: string; label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label htmlFor={htmlFor} className="text-sm font-medium">
        {label}
      </label>
      {children}
    </div>
  );
}

interface Draft {
  since: string;
  until: string;
  actor: string;
  action: string;
  result: AuditFilter["result"];
  target: string;
}

const emptyDraft: Draft = { since: "", until: "", actor: "", action: "", result: "", target: "" };

function Filters({ current, onApply }: { current: AuditFilter; onApply: (f: AuditFilter) => void }) {
  const { t } = useTranslation("audit");
  const id = useId();
  const fmt = useFormatters();
  const perms = usePermissions();
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const actions = useQuery({ queryKey: [...auditKey, "actions"], queryFn: ({ signal }) => fetchActions(signal) });
  const list = actions.data?.actions ?? [];
  const prefixes = actionPrefixes(list);
  const set = (k: keyof Draft) => (e: { target: { value: string } }) => setDraft({ ...draft, [k]: e.target.value });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    onApply({
      since: zonedLocalToUtc(draft.since, fmt.timezone),
      until: zonedLocalToUtc(draft.until, fmt.timezone),
      actor: draft.actor,
      action: draft.action,
      result: draft.result,
      target: draft.target,
    });
  };
  return (
    <form onSubmit={submit} className="space-y-3 rounded-xl border bg-card p-4" aria-label={t("filter.title")} data-testid="audit-filters">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <Field htmlFor={`${id}-since`} label={t("filter.since")}>
          <Input id={`${id}-since`} type="datetime-local" value={draft.since} onChange={set("since")} />
        </Field>
        <Field htmlFor={`${id}-until`} label={t("filter.until")}>
          <Input id={`${id}-until`} type="datetime-local" value={draft.until} onChange={set("until")} />
        </Field>
        <Field htmlFor={`${id}-actor`} label={t("filter.actor")}>
          <Input id={`${id}-actor`} maxLength={128} placeholder={t("filter.actorPlaceholder")} value={draft.actor} onChange={set("actor")} />
        </Field>
        <Field htmlFor={`${id}-action`} label={t("filter.action")}>
          <select id={`${id}-action`} className={selectClass} value={draft.action} onChange={set("action")} data-testid="audit-action">
            <option value="">{t("filter.allActions")}</option>
            {prefixes.map((p) => (
              <option key={`p:${p}`} value={p}>
                {t("filter.prefixOption", { prefix: p })}
              </option>
            ))}
            {list.map((a) => (
              <option key={a} value={a}>
                {a}
              </option>
            ))}
          </select>
        </Field>
        <Field htmlFor={`${id}-result`} label={t("filter.result")}>
          <select id={`${id}-result`} className={selectClass} value={draft.result} onChange={set("result")}>
            <option value="">{t("filter.allResults")}</option>
            {RESULTS.map((r) => (
              <option key={r} value={r}>
                {t(`result.${r}`)}
              </option>
            ))}
          </select>
        </Field>
        <Field htmlFor={`${id}-target`} label={t("filter.target")}>
          <Input id={`${id}-target`} maxLength={128} placeholder={t("filter.targetPlaceholder")} value={draft.target} onChange={set("target")} />
        </Field>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="submit" data-testid="audit-apply">
          {t("filter.apply")}
        </Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            setDraft(emptyDraft);
            onApply(emptyFilter);
          }}
        >
          <RotateCcw aria-hidden />
          {t("filter.reset")}
        </Button>
        {perms.has(PERM_AUDIT_EXPORT) && (
          <Button asChild variant="outline" className="sm:ml-auto">
            <a href={exportUrl(current)} download title={t("filter.exportHint")} data-testid="audit-export">
              <Download aria-hidden />
              {t("filter.export")}
            </a>
          </Button>
        )}
      </div>
    </form>
  );
}

function EventList({ filter, onSelect }: { filter: AuditFilter; onSelect: (e: AuditEvent) => void }) {
  const { t } = useTranslation(["audit", "errors", "common"]);
  const fmt = useFormatters();
  const events = useInfiniteQuery({
    queryKey: [...auditKey, "events", filter],
    queryFn: ({ pageParam, signal }) => fetchEvents(filter, pageParam, signal),
    initialPageParam: undefined as number | undefined,
    getNextPageParam: (last) => last.next_before ?? undefined,
  });
  const rows = events.data?.pages.flatMap((p) => p.events) ?? [];
  const filtered = Object.values(filter).some((v) => v !== "");
  const time = (e: AuditEvent) => fmt.dateTime(e.occurred_at, { dateStyle: "short", timeStyle: "medium" });
  const actor = (e: AuditEvent) => e.actor_username || t(`actorType.${e.actor_type}`);
  const target = (e: AuditEvent) => (e.target_type || e.target_id ? [e.target_type, e.target_id].filter(Boolean).join(": ") : t("none"));
  const details = (e: AuditEvent) => (
    <Button variant="ghost" size="sm" onClick={() => onSelect(e)} aria-label={t("table.detailsLabel", { seq: e.seq })}>
      {t("table.details")}
    </Button>
  );

  if (events.isPending) return <Skeleton className="h-60 w-full" />;
  if (events.isError) {
    return (
      <div role="alert" className="space-y-2">
        <p className="text-sm text-muted-foreground">{errorText(t, events.error)}</p>
        <Button variant="outline" size="sm" onClick={() => void events.refetch()}>
          {t("common:app.retry")}
        </Button>
      </div>
    );
  }
  if (rows.length === 0) {
    return (
      <p className="rounded-xl border border-dashed p-8 text-center text-muted-foreground" data-testid="audit-empty">
        {filtered ? t("table.emptyFiltered") : t("table.empty")}
      </p>
    );
  }
  return (
    <div className="space-y-3">
      {/* ≥ md: table */}
      <div className="hidden rounded-xl border md:block">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("table.time")}</TableHead>
              <TableHead>{t("table.actor")}</TableHead>
              <TableHead>{t("table.action")}</TableHead>
              <TableHead>{t("table.result")}</TableHead>
              <TableHead>{t("table.target")}</TableHead>
              <TableHead>{t("table.ip")}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t("table.details")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((e) => (
              <TableRow key={e.seq} data-testid="audit-row" data-seq={e.seq} data-action={e.action}>
                <TableCell className="whitespace-nowrap tabular-nums">{time(e)}</TableCell>
                <TableCell className="max-w-40 truncate" title={actor(e)}>
                  {actor(e)}
                </TableCell>
                <TableCell className="font-mono text-xs">{e.action}</TableCell>
                <TableCell>
                  <ResultBadge result={e.result} />
                </TableCell>
                <TableCell className="max-w-56 truncate font-mono text-xs" title={target(e)}>
                  {target(e)}
                </TableCell>
                <TableCell className="font-mono text-xs">{e.ip || t("none")}</TableCell>
                <TableCell className="text-right">{details(e)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      {/* < md: cards */}
      <ul className="space-y-3 md:hidden">
        {rows.map((e) => (
          <li key={e.seq} className="space-y-1.5 rounded-xl border bg-card p-4" data-testid="audit-card" data-seq={e.seq}>
            <div className="flex items-start justify-between gap-2">
              <span className="font-mono text-xs break-all">{e.action}</span>
              <ResultBadge result={e.result} />
            </div>
            <p className="text-sm">
              <span className="font-medium">{actor(e)}</span>
              <span className="text-muted-foreground"> · {time(e)}</span>
            </p>
            <p className="truncate font-mono text-xs text-muted-foreground">{target(e)}</p>
            <div className="flex items-center justify-between">
              <span className="font-mono text-xs text-muted-foreground">{e.ip}</span>
              {details(e)}
            </div>
          </li>
        ))}
      </ul>
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="text-muted-foreground" data-testid="audit-shown">
          {t("table.shown", { count: rows.length })}
        </span>
        {events.hasNextPage ? (
          <Button variant="outline" size="sm" disabled={events.isFetchingNextPage} onClick={() => void events.fetchNextPage()} data-testid="audit-more">
            {t("table.loadMore")}
          </Button>
        ) : (
          <span className="text-muted-foreground">{t("table.end")}</span>
        )}
      </div>
    </div>
  );
}

function EventDetail({ event, onClose }: { event: AuditEvent | null; onClose: () => void }) {
  const { t } = useTranslation("audit");
  const fmt = useFormatters();
  const e = event;
  const meta = e && Object.keys(e.metadata).length > 0 ? JSON.stringify(e.metadata, null, 2) : null;
  const row = (label: string, value: React.ReactNode, mono = false) => (
    <div className="grid gap-0.5 sm:grid-cols-[9rem_1fr] sm:gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={cn("min-w-0 break-words", mono && "font-mono text-xs break-all")}>{value || t("none")}</dd>
    </div>
  );
  return (
    <Dialog open={e !== null} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl" data-testid="audit-detail">
        {e && (
          <>
            <DialogHeader>
              <DialogTitle>{t("detail.title")}</DialogTitle>
              <DialogDescription>{t("detail.description", { seq: e.seq })}</DialogDescription>
            </DialogHeader>
            <dl className="space-y-2 text-sm">
              {row(t("detail.seq"), String(e.seq), true)}
              {row(t("detail.time"), fmt.dateTime(e.occurred_at, DETAIL_TIME))}
              {row(t("detail.timeUtc"), e.occurred_at, true)}
              {row(t("detail.actor"), `${e.actor_username || t("none")} (${t(`actorType.${e.actor_type}`)})`)}
              {row(t("detail.actorId"), e.actor_id ?? "", true)}
              {row(t("detail.action"), e.action, true)}
              {row(t("detail.result"), <ResultBadge result={e.result} />)}
              {row(t("detail.target"), [e.target_type, e.target_id].filter(Boolean).join(": "), true)}
              {row(t("detail.ip"), e.ip, true)}
              {row(t("detail.userAgent"), e.user_agent)}
              {row(t("detail.requestId"), e.request_id, true)}
              {row(
                t("detail.metadata"),
                meta ? (
                  <pre className="max-h-64 overflow-auto rounded-md bg-muted p-2 font-mono text-xs whitespace-pre-wrap" data-testid="audit-metadata">
                    {meta}
                  </pre>
                ) : (
                  <span className="text-muted-foreground">{t("detail.noMetadata")}</span>
                ),
              )}
              {row(t("detail.hash"), e.hash, true)}
            </dl>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
