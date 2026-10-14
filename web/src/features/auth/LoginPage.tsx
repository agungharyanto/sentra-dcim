import { type FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useSearchParams } from "react-router";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { pendingStepPath, safeNext, useAuth } from "./auth-context";
import { AuthLayout } from "./AuthLayout";
import { FormMessage } from "./FormMessage";

export function LoginPage() {
  const { t } = useTranslation(["auth", "errors"]);
  const { me, expired, signIn } = useAuth();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const next = safeNext(params.get("next"));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (me && !busy) {
    return <Navigate to={pendingStepPath(me, next) ?? next} replace />;
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      setError(t("login.required"));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await signIn(username.trim(), password);
      setPassword("");
      await navigate(pendingStepPath(res, next) ?? next, { replace: true });
    } catch (err) {
      setPassword("");
      if (err instanceof ApiError && err.code === "invalid_credentials") {
        setError(t("login.invalid"));
      } else if (err instanceof ApiError && err.code === "rate_limited") {
        setError(t("login.rateLimited", { minutes: Math.max(1, Math.ceil((err.retryAfter ?? 60) / 60)) }));
      } else {
        setError(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthLayout title={t("login.title")}>
      <Card data-testid="login-page">
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl">{t("login.title")}</h1>
          </CardTitle>
          <CardDescription>{t("login.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={(e) => void submit(e)} noValidate className="space-y-4">
            {expired && !error && (
              <FormMessage tone="info" testId="login-expired">
                {t("login.expired")}
              </FormMessage>
            )}
            {error && (
              <FormMessage tone="error" testId="login-error">
                {error}
              </FormMessage>
            )}
            <div className="space-y-1.5">
              <label htmlFor="login-username" className="text-sm font-medium">
                {t("login.username")}
              </label>
              <Input
                id="login-username"
                name="username"
                autoComplete="username"
                autoCapitalize="none"
                spellCheck={false}
                autoFocus
                required
                maxLength={64}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                aria-invalid={error ? true : undefined}
                aria-describedby="login-username-hint"
              />
              <p id="login-username-hint" className="text-xs text-muted-foreground">
                {t("login.usernameHint")}
              </p>
            </div>
            <div className="space-y-1.5">
              <label htmlFor="login-password" className="text-sm font-medium">
                {t("login.password")}
              </label>
              <Input
                id="login-password"
                name="password"
                type="password"
                autoComplete="current-password"
                required
                maxLength={512}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                aria-invalid={error ? true : undefined}
              />
            </div>
            <Button type="submit" className="w-full" disabled={busy}>
              {busy ? t("login.submitting") : t("login.submit")}
            </Button>
          </form>
          <p className="mt-4 text-xs text-muted-foreground">{t("login.forgot")}</p>
        </CardContent>
      </Card>
    </AuthLayout>
  );
}
