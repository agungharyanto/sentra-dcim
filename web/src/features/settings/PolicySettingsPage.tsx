import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RotateCcw } from "lucide-react";
import { type FormEvent, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { fetchSettings, type SettingItem, type SettingsGroup, settingsKey, updateSettings } from "./api";

type Group = "session" | "password";

/** Settings › Session & password (settings.manage, v0.3.7): timeouts, expiry, history. Every change is audited. */
export function PolicySettingsPage() {
  const { t } = useTranslation("settings");
  return (
    <div className="space-y-4" data-testid="settings-policy">
      <p className="text-muted-foreground">{t("policy.description")}</p>
      <GroupCard group="session" />
      <GroupCard group="password" />
    </div>
  );
}

function GroupCard({ group }: { group: Group }) {
  const { t } = useTranslation(["settings", "errors"]);
  const query = useQuery({ queryKey: settingsKey(group), queryFn: ({ signal }) => fetchSettings(group, signal) });
  return (
    <Card data-testid={`policy-${group}`}>
      <CardHeader>
        <CardTitle className="text-base">{t(`policy.${group}.title`)}</CardTitle>
        <CardDescription>{t(`policy.${group}.description`)}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <Skeleton className="h-24 w-full" />
        ) : query.isError ? (
          <p role="alert" className="text-sm text-muted-foreground">
            {t(`errors:code.${toErrorCode(query.error instanceof ApiError ? query.error.code : "unknown")}`)}
          </p>
        ) : (
          <GroupForm key={JSON.stringify(query.data.items.map((i) => i.value))} group={group} data={query.data} />
        )}
      </CardContent>
    </Card>
  );
}

function GroupForm({ group, data }: { group: Group; data: SettingsGroup }) {
  const { t } = useTranslation(["settings", "errors"]);
  const qc = useQueryClient();
  const id = useId();
  const [draft, setDraft] = useState<Record<string, string>>(() => Object.fromEntries(data.items.map((i) => [i.name, String(i.value)])));
  const [errors, setErrors] = useState<Record<string, string>>({});
  const fail = (err: unknown) => {
    if (err instanceof ApiError && err.code === "validation_failed" && err.field) setErrors({ [err.field]: t("errors:code.validation_failed") });
    else toast.error(t(`errors:code.${toErrorCode(err instanceof ApiError ? err.code : "unknown")}`));
  };
  const save = useMutation({
    mutationFn: (values: Record<string, unknown>) => updateSettings(group, values),
    onSuccess: (g) => {
      qc.setQueryData(settingsKey(group), g);
      toast.success(t("policy.saved"));
    },
    onError: fail,
  });
  const reset = useMutation({
    mutationFn: (name: string) => updateSettings(group, { [name]: null }),
    onSuccess: (g) => qc.setQueryData(settingsKey(group), g),
    onError: fail,
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setErrors({});
    const changed = data.items.filter((i) => draft[i.name] !== String(i.value));
    if (changed.length === 0) {
      toast(t("regional.noChanges"));
      return;
    }
    const values: Record<string, unknown> = {};
    for (const i of changed) {
      const n = Number(draft[i.name]);
      if (!Number.isInteger(n)) {
        setErrors({ [i.name]: t("errors:code.validation_failed") });
        return;
      }
      values[i.name] = n;
    }
    save.mutate(values);
  };
  const field = (i: SettingItem) => {
    const fid = `${id}-${i.name}`;
    const err = errors[i.name];
    return (
      <div key={i.name} className="space-y-1" data-field={i.name}>
        <label htmlFor={fid} className="text-sm font-medium">
          {t(`policy.${group}.${i.name}` as "policy.session.idle_minutes")}
        </label>
        <Input
          id={fid}
          type="number"
          inputMode="numeric"
          min={i.min}
          max={i.max}
          step={1}
          value={draft[i.name] ?? ""}
          onChange={(e) => setDraft({ ...draft, [i.name]: e.target.value })}
          aria-invalid={err ? true : undefined}
          aria-describedby={`${fid}-hint`}
          className="max-w-40"
          data-testid={`policy-${i.name}`}
        />
        <p id={`${fid}-hint`} className={err ? "text-xs text-destructive" : "text-xs text-muted-foreground"} role={err ? "alert" : undefined}>
          {err ??
            t(`policy.${group}.${i.name}Hint` as "policy.session.idle_minutesHint", { min: i.min, max: i.max, def: String(i.default) })}
        </p>
        {!i.is_default && (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto px-0 text-xs"
            disabled={reset.isPending}
            onClick={() => reset.mutate(i.name)}
            data-testid={`policy-reset-${i.name}`}
          >
            <RotateCcw aria-hidden />
            {t("regional.reset")}
          </Button>
        )}
      </div>
    );
  };
  return (
    <form onSubmit={submit} className="space-y-4" noValidate>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{data.items.map(field)}</div>
      <Button type="submit" disabled={save.isPending} data-testid={`policy-save-${group}`}>
        {t("regional.save")}
      </Button>
    </form>
  );
}
