import { useQuery } from "@tanstack/react-query";
import { type ReactNode, useEffect } from "react";
import { useTranslation } from "react-i18next";

import { isLocale, storedLocale } from "@/i18n";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { bootstrapQueryKey, fetchBootstrap } from "./api";
import { BootstrapContext } from "./regional";

/** Loads GET /api/v1/ui/bootstrap once, applies the server default locale, then renders children. */
export function BootstrapGate({ children }: { children: ReactNode }) {
  const { t, i18n } = useTranslation(["errors", "common"]);
  const query = useQuery({
    queryKey: bootstrapQueryKey,
    queryFn: ({ signal }) => fetchBootstrap(signal),
    staleTime: Infinity,
  });

  const serverLocale = query.data?.default_locale;
  useEffect(() => {
    // A choice made in this browser wins over the server default.
    if (!storedLocale() && isLocale(serverLocale) && i18n.language !== serverLocale) {
      void i18n.changeLanguage(serverLocale);
    }
  }, [serverLocale, i18n]);

  if (query.isPending) {
    return (
      <p role="status" className="p-8 text-muted-foreground">
        {t("common:app.loading")}
      </p>
    );
  }
  if (query.isError) {
    const code = toErrorCode(query.error instanceof ApiError ? query.error.code : "unknown");
    return (
      <div role="alert" className="mx-auto max-w-xl space-y-3 p-8">
        <h1 className="text-lg font-semibold">{t("bootstrap.title")}</h1>
        <p className="text-muted-foreground">{t("bootstrap.description")}</p>
        <p className="text-sm text-muted-foreground">
          {t(`code.${code}`)}
        </p>
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
  return <BootstrapContext value={query.data}>{children}</BootstrapContext>;
}
