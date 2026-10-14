import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { useAuth } from "@/features/auth/auth-context";
import { useBootstrap } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { fetchPreferences, type Preferences, preferencesKey, REGIONAL_NAMES, type RegionalName, updatePreferences } from "./api";
import { draftFormat, type RegionalDraft, RegionalFields, RegionalPreview } from "./RegionalFields";

function draftOf(p: Preferences): RegionalDraft {
  const out = {} as RegionalDraft;
  for (const n of REGIONAL_NAMES) out[n] = p.preferences[`regional.${n}`] ?? "";
  return out;
}

/** Account › Preferences (every user): own regional values or "follow the system". Changes are audited. */
export function PreferencesPage() {
  const { t } = useTranslation(["settings", "errors"]);
  const query = useQuery({ queryKey: preferencesKey, queryFn: ({ signal }) => fetchPreferences(signal) });
  if (query.isPending) return <Skeleton className="h-64 w-full" />;
  if (query.isError) {
    return (
      <p role="alert" className="text-sm text-muted-foreground">
        {t(`errors:code.${toErrorCode(query.error instanceof ApiError ? query.error.code : "unknown")}`)}
      </p>
    );
  }
  return <PreferencesForm key={JSON.stringify(query.data.preferences)} prefs={query.data} />;
}

function PreferencesForm({ prefs }: { prefs: Preferences }) {
  const { t } = useTranslation(["settings", "errors"]);
  const qc = useQueryClient();
  const { refresh } = useAuth();
  const boot = useBootstrap();
  const [draft, setDraft] = useState<RegionalDraft>(() => draftOf(prefs));
  const [errors, setErrors] = useState<Partial<Record<RegionalName, string>>>({});

  const save = useMutation({
    mutationFn: (values: Record<string, string | null>) => updatePreferences(values),
    onSuccess: async (p) => {
      qc.setQueryData(preferencesKey, p);
      await refresh(); // effective values (and the language) follow immediately
      toast.success(t("preferences.saved"));
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === "validation_failed" && err.field?.startsWith("regional.")) {
        setErrors({ [err.field.slice("regional.".length)]: t("errors:code.validation_failed") });
      } else {
        toast.error(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`));
      }
    },
  });

  const current = draftOf(prefs);
  const changed = REGIONAL_NAMES.filter((n) => draft[n] !== current[n]);
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setErrors({});
    if (changed.length === 0) {
      toast(t("regional.noChanges"));
      return;
    }
    save.mutate(Object.fromEntries(changed.map((n) => [`regional.${n}`, draft[n] === "" ? null : draft[n]])));
  };
  return (
    <form onSubmit={submit} className="max-w-4xl space-y-4" data-testid="preferences-page" noValidate>
      <p className="text-muted-foreground">{t("preferences.description")}</p>
      <div className="rounded-xl border bg-card p-4">
        <RegionalFields value={draft} onChange={setDraft} languages={boot.supported_locales} follow={prefs.system} errors={errors} />
      </div>
      <RegionalPreview format={draftFormat(draft, prefs.system)} />
      <Button type="submit" disabled={save.isPending} data-testid="preferences-save">
        {t("preferences.save")}
      </Button>
    </form>
  );
}
