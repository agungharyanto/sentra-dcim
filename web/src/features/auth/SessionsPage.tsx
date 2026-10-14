import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { MonitorSmartphone } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "@/components/ui/toast";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { fetchSessions, revokeSession, type SessionInfo, sessionsQueryKey } from "./api";
import { describeDevice } from "./device";

/** "Sesi saya": the user's own active sessions, with revoke (GET/DELETE /api/v1/auth/sessions). */
export function SessionsPage() {
  const { t } = useTranslation(["auth", "errors", "common"]);
  const fmt = useFormatters();
  const qc = useQueryClient();
  const query = useQuery({ queryKey: sessionsQueryKey, queryFn: ({ signal }) => fetchSessions(signal) });
  const revoke = useMutation({
    mutationFn: (id: string) => revokeSession(id),
    onSuccess: () => {
      toast.success(t("sessions.revoked"));
      void qc.invalidateQueries({ queryKey: sessionsQueryKey });
    },
    onError: (err) => toast.error(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`)),
  });

  const when = (v: string) => fmt.dateTime(v, { dateStyle: "medium", timeStyle: "short" });
  const ends = (s: SessionInfo) => (s.idle_expires_at < s.expires_at ? s.idle_expires_at : s.expires_at);
  const device = (s: SessionInfo) => describeDevice(s.user_agent) || t("sessions.unknownDevice");
  const action = (s: SessionInfo) =>
    s.current ? (
      <Badge variant="secondary">{t("sessions.current")}</Badge>
    ) : (
      <Button
        variant="outline"
        size="sm"
        disabled={revoke.isPending}
        onClick={() => revoke.mutate(s.id)}
        aria-label={t("sessions.revokeLabel", { device: device(s), time: when(s.created_at) })}
      >
        {t("sessions.revoke")}
      </Button>
    );

  return (
    <section className="max-w-5xl space-y-4" data-testid="sessions-page">
      <p className="text-muted-foreground">{t("sessions.description")}</p>
      {query.isPending && <Skeleton className="h-32 w-full" />}
      {query.isError && (
        <div role="alert" className="space-y-2">
          <p className="text-sm text-muted-foreground">
            {t(`errors:code.${toErrorCode(query.error instanceof ApiError ? query.error.code : "unknown")}`)}
          </p>
          <Button variant="outline" size="sm" onClick={() => void query.refetch()}>
            {t("common:app.retry")}
          </Button>
        </div>
      )}
      {query.data && (
        <>
          {/* ≥ md: table */}
          <div className="hidden rounded-xl border md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("sessions.device")}</TableHead>
                  <TableHead>{t("sessions.ip")}</TableHead>
                  <TableHead>{t("sessions.signedIn")}</TableHead>
                  <TableHead>{t("sessions.lastSeen")}</TableHead>
                  <TableHead>{t("sessions.ends")}</TableHead>
                  <TableHead className="text-right">
                    <span className="sr-only">{t("sessions.actions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {query.data.sessions.map((s) => (
                  <TableRow key={s.id} data-testid="session-row" data-current={s.current}>
                    <TableCell className="font-medium" title={s.user_agent}>
                      {device(s)}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{s.ip ?? "—"}</TableCell>
                    <TableCell>{when(s.created_at)}</TableCell>
                    <TableCell>{when(s.last_seen_at)}</TableCell>
                    <TableCell>{when(ends(s))}</TableCell>
                    <TableCell className="text-right">{action(s)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          {/* < md: cards */}
          <ul className="space-y-3 md:hidden">
            {query.data.sessions.map((s) => (
              <li key={s.id} className="rounded-xl border bg-card p-4" data-testid="session-card">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-2">
                    <MonitorSmartphone className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                    <span className="truncate font-medium">{device(s)}</span>
                  </div>
                  {action(s)}
                </div>
                <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
                  <dt className="text-muted-foreground">{t("sessions.ip")}</dt>
                  <dd className="font-mono text-xs leading-5">{s.ip ?? "—"}</dd>
                  <dt className="text-muted-foreground">{t("sessions.signedIn")}</dt>
                  <dd>{when(s.created_at)}</dd>
                  <dt className="text-muted-foreground">{t("sessions.lastSeen")}</dt>
                  <dd>{when(s.last_seen_at)}</dd>
                  <dt className="text-muted-foreground">{t("sessions.ends")}</dt>
                  <dd>{when(ends(s))}</dd>
                </dl>
              </li>
            ))}
          </ul>
          <p className="text-xs text-muted-foreground">{t("sessions.note")}</p>
        </>
      )}
    </section>
  );
}
