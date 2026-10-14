import { type FormEvent, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useSearchParams } from "react-router";

import { CopyButton } from "@/components/domain/CopyButton";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { confirmMfaEnrolment, type MfaEnrolment, startMfaEnrolment, verifyMfa } from "../api";
import { pendingStepPath, safeNext, useAuth } from "../auth-context";
import { AuthLayout } from "../AuthLayout";
import { FormMessage } from "../FormMessage";
import { CodeField } from "./CodeField";
import { QrCode } from "./QrCode";
import { RecoveryCodes } from "./RecoveryCodes";

/**
 * MFA step after the password (/mfa): enrolment when the account has no MFA yet, otherwise the code (or a recovery
 * code). The session reaches nothing else until this step passes (D-062).
 */
export function MfaPage() {
  const { t } = useTranslation("auth");
  const { me, refresh, signOut } = useAuth();
  const [params] = useSearchParams();
  const next = safeNext(params.get("next"));
  const [codes, setCodes] = useState<string[] | null>(null);

  // Already passed (and not showing the freshly issued recovery codes): continue.
  if (me?.session.mfa_verified && !codes) return <Navigate to={pendingStepPath(me, next) ?? next} replace />;

  // Re-read /me (now mfa_verified); the guard above then continues to the next step or `next`.
  const proceed = async () => {
    await refresh();
    setCodes(null);
  };

  const enrolling = !me?.user.mfa_enabled || codes !== null;
  const title = codes ? t("mfa.recoveryTitle") : enrolling ? t("mfa.enrolTitle") : t("mfa.verifyTitle");
  return (
    <AuthLayout title={title} wide={enrolling}>
      <Card data-testid={enrolling ? "mfa-enrol-page" : "mfa-verify-page"}>
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl">{title}</h1>
          </CardTitle>
          <CardDescription>
            {codes ? t("mfa.recoveryDescription") : enrolling ? t("mfa.enrolDescription") : t("mfa.verifyDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {codes ? (
            <RecoveryCodes codes={codes} onDone={() => void proceed()} doneLabel={t("mfa.continue")} />
          ) : enrolling ? (
            <Enrolment onEnabled={setCodes} />
          ) : (
            <Verify onDone={() => void proceed()} />
          )}
          {!codes && (
            <Button variant="link" className="h-auto px-0" onClick={() => void signOut()}>
              {t("user.logout")}
            </Button>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  );
}

function useCodeError() {
  const { t } = useTranslation(["auth", "errors"]);
  return (err: unknown) => {
    const e = err instanceof ApiError ? err : new ApiError("unknown", 0);
    if (e.code === "invalid_mfa_code") return t("mfa.invalid");
    if (e.code === "rate_limited") return t("login.rateLimited", { minutes: Math.max(1, Math.ceil((e.retryAfter ?? 60) / 60)) });
    return t(`errors:code.${toErrorCode(e.code)}`);
  };
}

function Verify({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation("auth");
  const id = useId();
  const [code, setCode] = useState("");
  const [recovery, setRecovery] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const message = useCodeError();

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) {
      setError(recovery ? t("mfa.recoveryRequired") : t("mfa.codeRequired"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await verifyMfa(code.trim());
      onDone();
    } catch (err) {
      setCode("");
      setError(message(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={(e) => void submit(e)} noValidate className="space-y-4">
      {error && (
        <FormMessage tone="error" testId="mfa-error">
          {error}
        </FormMessage>
      )}
      <CodeField id={`${id}-code`} value={code} onChange={setCode} recovery={recovery} invalid={!!error} autoFocus />
      <Button type="submit" className="w-full" disabled={busy}>
        {busy ? t("mfa.verifying") : t("mfa.verify")}
      </Button>
      <Button
        type="button"
        variant="link"
        className="h-auto px-0"
        onClick={() => {
          setRecovery(!recovery);
          setCode("");
          setError(null);
        }}
        data-testid="mfa-toggle-recovery"
      >
        {recovery ? t("mfa.useApp") : t("mfa.useRecovery")}
      </Button>
      <p className="text-xs text-muted-foreground">{t("mfa.lostPhone")}</p>
    </form>
  );
}

function Enrolment({ onEnabled }: { onEnabled: (codes: string[]) => void }) {
  const { t } = useTranslation("auth");
  const id = useId();
  const [data, setData] = useState<MfaEnrolment | null>(null);
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const message = useCodeError();

  const start = async () => {
    setBusy(true);
    setError(null);
    try {
      setData(await startMfaEnrolment());
    } catch (err) {
      setError(message(err));
    } finally {
      setBusy(false);
    }
  };

  const confirm = async (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) {
      setError(t("mfa.codeRequired"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await confirmMfaEnrolment(code.trim());
      setData(null); // the secret leaves the page as soon as it is no longer needed
      onEnabled(res.recovery_codes);
    } catch (err) {
      setCode("");
      setError(message(err));
    } finally {
      setBusy(false);
    }
  };

  if (!data) {
    return (
      <div className="space-y-4">
        {error && (
          <FormMessage tone="error" testId="mfa-error">
            {error}
          </FormMessage>
        )}
        <ol className="list-decimal space-y-2 pl-5 text-sm">
          <li>{t("mfa.stepInstall")}</li>
          <li>{t("mfa.stepScan")}</li>
          <li>{t("mfa.stepCode")}</li>
        </ol>
        <Button className="w-full" disabled={busy} onClick={() => void start()} data-testid="mfa-start">
          {t("mfa.start")}
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={(e) => void confirm(e)} noValidate className="space-y-4">
      {error && (
        <FormMessage tone="error" testId="mfa-error">
          {error}
        </FormMessage>
      )}
      <div className="flex flex-col items-center gap-3 sm:flex-row sm:items-start">
        <QrCode rows={data.qr} label={t("mfa.qrLabel", { account: data.account })} />
        <div className="w-full min-w-0 space-y-2 text-sm">
          <p>{t("mfa.scanHint")}</p>
          <p className="text-muted-foreground">{t("mfa.manualHint")}</p>
          <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1">
            <dt className="text-muted-foreground">{t("mfa.account")}</dt>
            <dd className="truncate">
              {data.issuer}: {data.account}
            </dd>
            <dt className="text-muted-foreground">{t("mfa.key")}</dt>
            <dd className="flex items-start gap-1">
              <code className="font-mono" data-testid="mfa-secret">
                {data.secret}
              </code>
              <CopyButton value={data.secret.replaceAll(" ", "")} label={t("mfa.copyKey")} />
            </dd>
            <dt className="text-muted-foreground">{t("mfa.type")}</dt>
            <dd>{t("mfa.typeValue")}</dd>
          </dl>
        </div>
      </div>
      <CodeField id={`${id}-code`} value={code} onChange={setCode} invalid={!!error} />
      <Button type="submit" className="w-full" disabled={busy} data-testid="mfa-confirm">
        {busy ? t("mfa.verifying") : t("mfa.activate")}
      </Button>
    </form>
  );
}
