import { useId } from "react";
import { useTranslation } from "react-i18next";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Regional } from "@/features/auth/api";
import { useFormatters } from "@/features/bootstrap/regional";
import { type DateFormat, dateFormats, formatDate, formatDateAndTime, formatTime, type RegionalFormat, type TimeFormat, timeFormats } from "@/lib/format";

import { REGIONAL_NAMES, type RegionalName, timeZones } from "./api";

/** Field values by name; "" means "follow the system" (preferences only). */
export type RegionalDraft = Record<RegionalName, string>;

const selectClass =
  "h-9 w-full rounded-md border border-input bg-background px-3 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring aria-invalid:border-destructive md:text-sm";

/** Labels of regional values in the UI language. */
export function useRegionalText() {
  const { t } = useTranslation("settings");
  const fmt = useFormatters();
  const sample = new Date();
  return (name: RegionalName, value: string): string => {
    switch (name) {
      case "language":
        return t(`value.language.${value as "id" | "en"}`, { defaultValue: value });
      case "time_format":
        return t(`value.time_format.${value as TimeFormat}`, { defaultValue: value });
      case "date_format":
        return `${formatDate(sample, { ...fmt.regional, dateFormat: value as DateFormat })} (${value})`;
      default:
        return value;
    }
  };
}

export function draftFormat(d: RegionalDraft, fallback: Regional): RegionalFormat {
  return {
    locale: d.language || fallback.language,
    timeZone: d.timezone || fallback.timezone,
    timeFormat: (d.time_format || fallback.time_format) as TimeFormat,
    dateFormat: (d.date_format || fallback.date_format) as DateFormat,
  };
}

interface Props {
  value: RegionalDraft;
  onChange: (next: RegionalDraft) => void;
  /** Allowed languages (server); time and date formats are fixed lists. */
  languages: readonly string[];
  /** With `follow`, every field offers "Follow system (<value>)" as its first option (value ""). */
  follow?: Regional;
  errors?: Partial<Record<RegionalName, string>>;
  /** Extra line under a field (default value, reset button). */
  footer?: (name: RegionalName) => React.ReactNode;
}

/** The four regional fields, shared by Settings › Regional (system) and Account › Preferences (own). */
export function RegionalFields({ value, onChange, languages, follow, errors = {}, footer }: Props) {
  const { t } = useTranslation("settings");
  const id = useId();
  const label = useRegionalText();
  const zones = timeZones();
  const options: Record<RegionalName, readonly string[]> = {
    language: languages,
    timezone: value.timezone && !zones.includes(value.timezone) ? [value.timezone, ...zones] : zones,
    time_format: timeFormats,
    date_format: dateFormats,
  };
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      {REGIONAL_NAMES.map((name) => {
        const fieldId = `${id}-${name}`;
        const err = errors[name];
        return (
          <div key={name} className="space-y-1" data-field={name}>
            <label htmlFor={fieldId} className="text-sm font-medium">
              {t(`field.${name}`)}
            </label>
            <select
              id={fieldId}
              className={selectClass}
              value={value[name]}
              aria-invalid={err ? true : undefined}
              aria-describedby={err ? `${fieldId}-err` : name === "timezone" ? `${fieldId}-hint` : undefined}
              onChange={(e) => onChange({ ...value, [name]: e.target.value })}
              data-testid={`regional-${name}`}
            >
              {follow && <option value="">{t("preferences.followSystem", { value: label(name, follow[name]) })}</option>}
              {options[name].map((o) => (
                <option key={o} value={o}>
                  {label(name, o)}
                </option>
              ))}
            </select>
            {name === "timezone" && !err && (
              <p id={`${fieldId}-hint`} className="text-xs text-muted-foreground">
                {t("hint.timezone")}
              </p>
            )}
            {err && (
              <p id={`${fieldId}-err`} role="alert" className="text-xs text-destructive">
                {err}
              </p>
            )}
            {footer?.(name)}
          </div>
        );
      })}
    </div>
  );
}

/** Live preview of a regional configuration (now). */
export function RegionalPreview({ format }: { format: RegionalFormat }) {
  const { t } = useTranslation("settings");
  const now = new Date();
  return (
    <Card data-testid="regional-preview">
      <CardHeader>
        <CardTitle className="text-base">{t("preview.title")}</CardTitle>
      </CardHeader>
      <CardContent>
        <dl className="grid gap-2 text-sm sm:grid-cols-3">
          <div>
            <dt className="text-muted-foreground">{t("preview.dateTime")}</dt>
            <dd className="font-medium tabular-nums" data-testid="preview-datetime">
              {formatDateAndTime(now, format, { zone: true })}
            </dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("preview.date")}</dt>
            <dd className="font-medium tabular-nums">{formatDate(now, format)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">{t("preview.time")}</dt>
            <dd className="font-medium tabular-nums">{formatTime(now, format, { seconds: true })}</dd>
          </div>
        </dl>
      </CardContent>
    </Card>
  );
}
