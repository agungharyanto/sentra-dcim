import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ShieldCheck } from "lucide-react";
import { type FormEvent, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { fetchMfaStatus, mfaStatusKey, regenerateRecoveryCodes } from "./api";
import { FormMessage } from "./FormMessage";
import { CodeField } from "./mfa/CodeField";
import { RecoveryCodes } from "./mfa/RecoveryCodes";

/** "Keamanan akun" (/account/security): MFA status and new recovery codes (needs a current code). */
export function SecurityPage() {
  const { t } = useTranslation(["auth", "errors", "common"]);
  const fmt = useFormatters();
  const status = useQuery({ queryKey: mfaStatusKey, queryFn: ({ signal }) => fetchMfaStatus(signal) });
  const [open, setOpen] = useState(false);
  const since = (v: string) => fmt.dateTime(v, { dateStyle: "medium", timeStyle: "short" });

  return (
    <section className="max-w-2xl space-y-4" data-testid="security-page">
      <p className="text-muted-foreground">{t("security.description")}</p>
      {status.isPending && <Skeleton className="h-40 w-full" />}
      {status.isError && (
        <p role="alert" className="text-sm text-muted-foreground">
          {t(`errors:code.${toErrorCode(status.error instanceof ApiError ? status.error.code : "unknown")}`)}
        </p>
      )}
      {status.data && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ShieldCheck className="size-5 text-status-normal-solid" aria-hidden />
              {t("security.mfaTitle")}
            </CardTitle>
            <CardDescription>{t("security.mfaDescription")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
              <dt className="text-muted-foreground">{t("security.status")}</dt>
              <dd>
                <Badge
                  variant="outline"
                  className="border-status-normal-border bg-status-normal-bg text-status-normal-fg"
                  data-testid="mfa-status"
                >
                  {status.data.enabled ? t("security.enabled") : t("security.disabled")}
                </Badge>
              </dd>
              {status.data.enabled_at && (
                <>
                  <dt className="text-muted-foreground">{t("security.since")}</dt>
                  <dd>{since(status.data.enabled_at)}</dd>
                </>
              )}
              <dt className="text-muted-foreground">{t("security.recoveryRemaining")}</dt>
              <dd data-testid="recovery-remaining">{t("security.recoveryCount", { count: status.data.recovery_codes_remaining })}</dd>
            </dl>
            {status.data.recovery_codes_remaining <= 3 && (
              <FormMessage tone="info">{t("security.recoveryLow")}</FormMessage>
            )}
            <Button variant="outline" onClick={() => setOpen(true)} data-testid="regenerate-recovery">
              {t("security.regenerate")}
            </Button>
            <p className="text-xs text-muted-foreground">{t("security.lostPhone")}</p>
          </CardContent>
        </Card>
      )}
      <RegenerateDialog open={open} onOpenChange={setOpen} />
    </section>
  );
}

function RegenerateDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { t } = useTranslation(["auth", "errors"]);
  const id = useId();
  const qc = useQueryClient();
  const [code, setCode] = useState("");
  const [codes, setCodes] = useState<string[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const regen = useMutation({
    mutationFn: () => regenerateRecoveryCodes(code.trim()),
    onSuccess: (res) => {
      setCodes(res.recovery_codes);
      void qc.invalidateQueries({ queryKey: mfaStatusKey });
    },
    onError: (err) => {
      setCode("");
      const c = err instanceof ApiError ? err.code : "unknown";
      setError(c === "invalid_mfa_code" ? t("mfa.invalid") : t(`errors:code.${toErrorCode(c)}`));
    },
  });
  const close = (o: boolean) => {
    if (!o) {
      setCode("");
      setCodes(null);
      setError(null);
    }
    onOpenChange(o);
  };
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) {
      setError(t("mfa.codeRequired"));
      return;
    }
    setError(null);
    regen.mutate();
  };
  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent data-testid="regenerate-dialog">
        <DialogHeader>
          <DialogTitle>{codes ? t("mfa.recoveryTitle") : t("security.regenerate")}</DialogTitle>
          <DialogDescription>{codes ? t("mfa.recoveryDescription") : t("security.regenerateDescription")}</DialogDescription>
        </DialogHeader>
        {codes ? (
          <RecoveryCodes codes={codes} onDone={() => close(false)} doneLabel={t("security.done")} />
        ) : (
          <form onSubmit={submit} noValidate className="space-y-4">
            {error && <FormMessage tone="error">{error}</FormMessage>}
            <CodeField id={`${id}-code`} value={code} onChange={setCode} invalid={!!error} />
            <Button type="submit" className="w-full" disabled={regen.isPending}>
              {regen.isPending ? t("mfa.verifying") : t("security.regenerateSubmit")}
            </Button>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
