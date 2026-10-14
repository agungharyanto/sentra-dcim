import { useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { isLocale, setLocale } from "@/i18n";
import { onAuthError } from "@/lib/api";

import { type AuthMe, fetchMe, login, logout, meQueryKey } from "./api";
import { AuthContext, type AuthState } from "./auth-context";

/**
 * Owns the session state (GET /api/v1/auth/me). Any API call answering 401 "unauthenticated" signs the user out
 * locally, so the route guard sends them to the login page; 403 "password_change_required" re-reads /me.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const { t, i18n } = useTranslation(["common", "errors"]);
  const qc = useQueryClient();
  const [expired, setExpired] = useState(false);
  const query = useQuery({ queryKey: meQueryKey, queryFn: ({ signal }) => fetchMe(signal), staleTime: Infinity });
  const me = query.data ?? null;

  // The effective language (own preference, else the system language) wins after sign-in and on every reload.
  const language = me?.regional?.language;
  useEffect(() => {
    if (language && isLocale(language) && language !== i18n.resolvedLanguage) void setLocale(language);
  }, [language, i18n]);

  useEffect(
    () =>
      onAuthError((code) => {
        if (code === "unauthenticated") {
          if (qc.getQueryData<AuthMe | null>(meQueryKey)) setExpired(true);
          qc.removeQueries({ predicate: (q) => q.queryKey[0] !== meQueryKey[0] });
          qc.setQueryData(meQueryKey, null);
        } else {
          void qc.invalidateQueries({ queryKey: meQueryKey });
        }
      }),
    [qc],
  );

  const signIn = useCallback(
    async (username: string, password: string) => {
      const res = await login(username, password);
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== meQueryKey[0] });
      qc.setQueryData<AuthMe>(meQueryKey, res);
      setExpired(false);
      return res;
    },
    [qc],
  );

  const signOut = useCallback(async () => {
    try {
      await logout();
    } finally {
      // Local state is cleared even if the server already dropped the session. The /me query itself stays (its
      // observer lives in this provider) and becomes null.
      qc.removeQueries({ predicate: (q) => q.queryKey[0] !== meQueryKey[0] });
      qc.setQueryData(meQueryKey, null);
      setExpired(false);
    }
  }, [qc]);

  const refresh = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: meQueryKey });
  }, [qc]);

  const value = useMemo<AuthState>(() => ({ me, expired, signIn, signOut, refresh }), [me, expired, signIn, signOut, refresh]);

  if (query.isPending) {
    return (
      <p role="status" className="p-8 text-muted-foreground">
        {t("common:app.loading")}
      </p>
    );
  }
  if (query.isError) {
    return (
      <div role="alert" className="mx-auto max-w-xl space-y-3 p-8">
        <p className="text-muted-foreground">{t("errors:code.network")}</p>
        <button
          type="button"
          onClick={() => void query.refetch()}
          className="rounded-md border px-3 py-1.5 text-sm hover:bg-accent"
        >
          {t("common:app.retry")}
        </button>
      </div>
    );
  }
  return <AuthContext value={value}>{children}</AuthContext>;
}
