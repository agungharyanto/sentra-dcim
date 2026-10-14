import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RotateCcw } from "lucide-react";
import { type FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import type { Regional } from "@/features/auth/api";
import { meQueryKey } from "@/features/auth/api";
import { bootstrapQueryKey } from "@/features/bootstrap/api";
import { useBootstrap, useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { REGIONAL_NAMES, type RegionalName, type SettingsGroup, settingsKey, fetchSettings, updateSettings } from "./api";
import { draftFormat, type RegionalDraft, RegionalFields, RegionalPreview, useRegionalText } from "./RegionalFields";

const GROUP = "regional";

function draftOf(g: SettingsGroup): RegionalDraft {
  const out = {} as RegionalDraft;
  for (const n of REGIONAL_NAMES) out[n] = String(g.items.find((i) => i.name === n)?.value ?? "");
  return out;
}

/** Settings › Regional (Admin): system language, time zone, time and date format. Every change is audited. */
export function RegionalSettingsPage() {
  const { t } = useTranslation(["settings", "errors"]);
  const query = useQuery({ queryKey: settingsKey(GROUP), queryFn: ({ signal }) => fetchSettings(GROUP, signal) });
  if (query.isPending) return <Skeleton className="h-64 w-full" />;
  if (query.isError) {
    return (
      <p role="alert" className="text-sm text-muted-foreground">
        {t(`errors:code.${toErrorCode(query.error instanceof ApiError ? query.error.code : "unknown")}`)}
      </p>
    );
  }
  // A new server state (after saving or resetting) starts a fresh draft.
  return <RegionalForm key={JSON.stringify(draftOf(query.data))} group={query.data} />;
}

function RegionalForm({ group }: { group: SettingsGroup }) {
  const { t } = useTranslation(["settings", "errors"]);
  const qc = useQueryClient();
  const boot = useBootstrap();
  const fmt = useFormatters();
  const label = useRegionalText();
  const [draft, setDraft] = useState<RegionalDraft>(() => draftOf(group));
  const [errors, setErrors] = useState<Partial<Record<RegionalName, string>>>({});

  const afterSave = (g: SettingsGroup) => {
    qc.setQueryData(settingsKey(GROUP), g);
    // System values reach the bootstrap document and the effective values of users following the system.
    void qc.invalidateQueries({ queryKey: bootstrapQueryKey });
    void qc.invalidateQueries({ queryKey: meQueryKey });
  };
  const fail = (err: unknown) => {
    if (err instanceof ApiError && err.code === "validation_failed" && err.field) {
      setErrors({ [err.field]: t("errors:code.validation_failed") });
    } else {
      toast.error(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`));
    }
  };
  const save = useMutation({
    mutationFn: (values: Record<string, unknown>) => updateSettings(GROUP, values),
    onSuccess: (g) => {
      afterSave(g);
      toast.success(t("regional.saved"));
    },
    onError: fail,
  });
  const reset = useMutation({
    mutationFn: (name: RegionalName) => updateSettings(GROUP, { [name]: null }),
    onSuccess: (g, name) => {
      afterSave(g);
      toast.success(t("regional.resetDone", { field: t(`field.${name}`) }));
    },
    onError: fail,
  });

  const items = group.items;
  const item = (n: RegionalName) => items.find((i) => i.name === n);
  const current = draftOf(group);
  const changed = REGIONAL_NAMES.filter((n) => draft[n] !== current[n]);
  const system: Regional = {
    language: current.language,
    timezone: current.timezone,
    time_format: current.time_format as Regional["time_format"],
    date_format: current.date_format as Regional["date_format"],
  };
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setErrors({});
    if (changed.length === 0) {
      toast(t("regional.noChanges"));
      return;
    }
    save.mutate(Object.fromEntries(changed.map((n) => [n, draft[n]])));
  };

  return (
    <form onSubmit={submit} className="space-y-4" data-testid="settings-regional" noValidate>
      <p className="text-muted-foreground">{t("regional.description")}</p>
      <div className="rounded-xl border bg-card p-4">
        <RegionalFields
          value={draft}
          onChange={setDraft}
          languages={boot.supported_locales}
          errors={errors}
          footer={(n) => {
            const it = item(n);
            if (!it) return null;
            return (
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span>{t("regional.default", { value: label(n, String(it.default)) })}</span>
                {!it.is_default && (
                  <>
                    <Badge variant="outline">{t("regional.custom")}</Badge>
                    {it.updated_at && <span>{t("regional.lastChanged", { time: fmt.dateTime(it.updated_at) })}</span>}
                    <Button
                      type="button"
                      variant="link"
                      size="sm"
                      className="h-auto px-0 text-xs"
                      disabled={reset.isPending}
                      onClick={() => reset.mutate(n)}
                      aria-label={t("regional.resetLabel", { field: t(`field.${n}`) })}
                      data-testid={`reset-${n}`}
                    >
                      <RotateCcw aria-hidden />
                      {t("regional.reset")}
                    </Button>
                  </>
                )}
              </div>
            );
          }}
        />
      </div>
      <RegionalPreview format={draftFormat(draft, system)} />
      <Button type="submit" disabled={save.isPending} data-testid="settings-save">
        {t("regional.save")}
      </Button>
    </form>
  );
}
