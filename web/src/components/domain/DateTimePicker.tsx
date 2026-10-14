import { CalendarDays, ChevronLeft, ChevronRight, X } from "lucide-react";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useFormatters } from "@/features/bootstrap/regional";
import { dateTimePattern, intlLocale, utcToZonedLocal, zonedLocalToUtc, zoneName } from "@/lib/format";
import { cn } from "@/lib/utils";

interface Wall {
  y: number;
  m: number; // 1–12
  d: number;
  h: number;
  min: number;
}

const pad = (n: number) => String(n).padStart(2, "0");
const wallToLocal = (w: Wall) => `${w.y}-${pad(w.m)}-${pad(w.d)}T${pad(w.h)}:${pad(w.min)}`;

function parseLocal(s: string): Wall | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(s);
  return m ? { y: Number(m[1]), m: Number(m[2]), d: Number(m[3]), h: Number(m[4]), min: Number(m[5]) } : null;
}

const daysIn = (y: number, m: number) => new Date(Date.UTC(y, m, 0)).getUTCDate();
// Monday-first weekday index (0 = Monday) of the 1st of the month.
const firstWeekday = (y: number, m: number) => (new Date(Date.UTC(y, m - 1, 1)).getUTCDay() + 6) % 7;

export interface DateTimePickerProps {
  /** UTC RFC 3339 instant, or "" for empty. */
  value: string;
  onChange: (utc: string) => void;
  id?: string;
  /** Default time of day when a date is picked on an empty field ("start" 00:00 or "end" 23:59). */
  defaultTime?: "start" | "end";
  "aria-describedby"?: string;
}

/**
 * Date & time field of the design system (v0.3.5, ISS-043). Shows and edits the value in the user's effective
 * regional settings (time zone, date pattern, 12/24 h, language) and hands UTC to the caller. Calendar: Monday first,
 * arrow keys move the focused day, Page Up/Down change the month, Enter/Space pick.
 */
export function DateTimePicker({ value, onChange, id, defaultTime = "start", ...rest }: DateTimePickerProps) {
  const { t } = useTranslation("ds");
  const fmt = useFormatters();
  const tz = fmt.timezone;
  const wall = parseLocal(utcToZonedLocal(value, tz));
  const today = parseLocal(utcToZonedLocal(new Date().toISOString(), tz)) as Wall;
  const [open, setOpen] = useState(false);
  const [view, setView] = useState({ y: (wall ?? today).y, m: (wall ?? today).m });
  const [focusDay, setFocusDay] = useState((wall ?? today).d);
  const grid = useRef<HTMLDivElement>(null);

  // The calendar opens on the selected day (or today); value changes while open do not move it.
  const onOpenChange = (next: boolean) => {
    if (next) {
      const w = wall ?? today;
      setView({ y: w.y, m: w.m });
      setFocusDay(w.d);
    }
    setOpen(next);
  };

  useEffect(() => {
    if (open) grid.current?.querySelector<HTMLButtonElement>(`[data-day="${focusDay}"]`)?.focus();
  }, [open, focusDay, view]);

  const emit = (w: Wall) => onChange(zonedLocalToUtc(wallToLocal(w), tz));
  const pick = (d: number) => {
    const base = wall ?? { ...today, h: defaultTime === "end" ? 23 : 0, min: defaultTime === "end" ? 59 : 0 };
    emit({ ...base, y: view.y, m: view.m, d });
  };
  const setTime = (h: number, min: number) => emit({ ...(wall ?? { ...today, d: today.d }), h, min });

  const move = (deltaDays: number) => {
    const dt = new Date(Date.UTC(view.y, view.m - 1, focusDay + deltaDays));
    setView({ y: dt.getUTCFullYear(), m: dt.getUTCMonth() + 1 });
    setFocusDay(dt.getUTCDate());
  };
  const shiftMonth = (delta: number) => {
    const dt = new Date(Date.UTC(view.y, view.m - 1 + delta, 1));
    const y = dt.getUTCFullYear();
    const m = dt.getUTCMonth() + 1;
    setView({ y, m });
    setFocusDay(Math.min(focusDay, daysIn(y, m)));
  };
  const onGridKey = (e: KeyboardEvent) => {
    const keys: Record<string, () => void> = {
      ArrowLeft: () => move(-1),
      ArrowRight: () => move(1),
      ArrowUp: () => move(-7),
      ArrowDown: () => move(7),
      PageUp: () => shiftMonth(-1),
      PageDown: () => shiftMonth(1),
    };
    const fn = keys[e.key];
    if (fn) {
      e.preventDefault();
      fn();
    }
  };

  const locale = intlLocale(fmt.locale);
  const monthLabel = new Intl.DateTimeFormat(locale, { month: "long", year: "numeric", timeZone: "UTC" }).format(
    new Date(Date.UTC(view.y, view.m - 1, 1)),
  );
  // 2026-01-05 is a Monday.
  const weekdays = Array.from({ length: 7 }, (_, i) =>
    new Intl.DateTimeFormat(locale, { weekday: "short", timeZone: "UTC" }).format(new Date(Date.UTC(2026, 0, 5 + i))),
  );
  const dayLabel = (d: number) =>
    new Intl.DateTimeFormat(locale, { dateStyle: "full", timeZone: "UTC" }).format(new Date(Date.UTC(view.y, view.m - 1, d)));
  const cells: (number | null)[] = [
    ...Array.from({ length: firstWeekday(view.y, view.m) }, () => null),
    ...Array.from({ length: daysIn(view.y, view.m) }, (_, i) => i + 1),
  ];
  const twelve = fmt.regional.timeFormat === "12h";
  const h12 = wall ? wall.h % 12 || 12 : 12;
  const pm = wall ? wall.h >= 12 : false;
  const selectClass =
    "h-8 rounded-md border border-input bg-background px-2 text-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring disabled:opacity-50";

  return (
    <div className="flex w-full items-center gap-1">
      <Popover open={open} onOpenChange={onOpenChange}>
        <PopoverTrigger asChild>
          <button
            type="button"
            id={id}
            aria-describedby={rest["aria-describedby"]}
            data-testid="datetime-trigger"
            data-value={value}
            className={cn(
              "flex h-9 w-full min-w-0 items-center gap-2 rounded-md border border-input bg-background px-3 text-left text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm",
              !value && "text-muted-foreground",
            )}
          >
            <CalendarDays className="size-4 shrink-0 text-muted-foreground" aria-hidden />
            <span className="truncate tabular-nums">{value ? fmt.dateTime(value) : dateTimePattern(fmt.regional)}</span>
          </button>
        </PopoverTrigger>
        <PopoverContent
          className="w-72"
          data-testid="datetime-popover"
          // Focus the selected (or today's) day instead of the first button of the popover.
          onOpenAutoFocus={(e) => {
            e.preventDefault();
            grid.current?.querySelector<HTMLButtonElement>('[tabindex="0"]')?.focus();
          }}
        >
          <div className="mb-2 flex items-center justify-between">
            <Button type="button" variant="ghost" size="icon" onClick={() => shiftMonth(-1)} aria-label={t("datetime.prevMonth")}>
              <ChevronLeft aria-hidden />
            </Button>
            <p className="text-sm font-medium" aria-live="polite" data-testid="datetime-month">
              {monthLabel}
            </p>
            <Button type="button" variant="ghost" size="icon" onClick={() => shiftMonth(1)} aria-label={t("datetime.nextMonth")}>
              <ChevronRight aria-hidden />
            </Button>
          </div>
          <div className="grid grid-cols-7 gap-1 text-center text-xs text-muted-foreground" aria-hidden>
            {weekdays.map((w) => (
              <span key={w}>{w}</span>
            ))}
          </div>
          <div ref={grid} className="mt-1 grid grid-cols-7 gap-1" onKeyDown={onGridKey} role="group" aria-label={monthLabel}>
            {cells.map((d, i) =>
              d === null ? (
                <span key={`e${i}`} />
              ) : (
                <button
                  key={d}
                  type="button"
                  data-day={d}
                  tabIndex={d === focusDay ? 0 : -1}
                  aria-label={dayLabel(d)}
                  aria-pressed={wall?.y === view.y && wall.m === view.m && wall.d === d}
                  onClick={() => pick(d)}
                  className={cn(
                    "h-8 rounded-md text-sm tabular-nums outline-none hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring",
                    wall?.y === view.y && wall.m === view.m && wall.d === d && "bg-primary text-primary-foreground hover:bg-primary",
                    today.y === view.y && today.m === view.m && today.d === d && "font-bold underline underline-offset-4",
                  )}
                >
                  {d}
                </button>
              ),
            )}
          </div>
          <fieldset className="mt-3 flex items-center gap-2 border-t pt-3" disabled={!wall}>
            <legend className="sr-only">{t("datetime.time")}</legend>
            <label className="sr-only" htmlFor={`${id ?? "dt"}-hour`}>
              {t("datetime.hour")}
            </label>
            <select
              id={`${id ?? "dt"}-hour`}
              className={selectClass}
              value={twelve ? h12 : (wall?.h ?? 0)}
              onChange={(e) => {
                const v = Number(e.target.value);
                setTime(twelve ? (v % 12) + (pm ? 12 : 0) : v, wall?.min ?? 0);
              }}
            >
              {(twelve ? Array.from({ length: 12 }, (_, i) => i + 1) : Array.from({ length: 24 }, (_, i) => i)).map((h) => (
                <option key={h} value={h}>
                  {twelve ? h : pad(h)}
                </option>
              ))}
            </select>
            <span aria-hidden>{fmt.locale === "id" ? "." : ":"}</span>
            <label className="sr-only" htmlFor={`${id ?? "dt"}-minute`}>
              {t("datetime.minute")}
            </label>
            <select
              id={`${id ?? "dt"}-minute`}
              className={selectClass}
              value={wall?.min ?? 0}
              onChange={(e) => setTime(wall?.h ?? 0, Number(e.target.value))}
            >
              {Array.from({ length: 60 }, (_, i) => i).map((m) => (
                <option key={m} value={m}>
                  {pad(m)}
                </option>
              ))}
            </select>
            {twelve && (
              <>
                <label className="sr-only" htmlFor={`${id ?? "dt"}-ampm`}>
                  {t("datetime.ampm")}
                </label>
                <select
                  id={`${id ?? "dt"}-ampm`}
                  className={selectClass}
                  value={pm ? "PM" : "AM"}
                  onChange={(e) => setTime((h12 % 12) + (e.target.value === "PM" ? 12 : 0), wall?.min ?? 0)}
                >
                  <option value="AM">{t("datetime.am")}</option>
                  <option value="PM">{t("datetime.pm")}</option>
                </select>
              </>
            )}
            <span className="ml-auto text-xs text-muted-foreground" title={tz}>
              {zoneName(new Date(), fmt.regional)}
            </span>
          </fieldset>
          <div className="mt-3 flex justify-end">
            <Button type="button" size="sm" onClick={() => onOpenChange(false)} data-testid="datetime-done">
              {t("datetime.done")}
            </Button>
          </div>
        </PopoverContent>
      </Popover>
      {value && (
        <Button type="button" variant="ghost" size="icon" onClick={() => onChange("")} aria-label={t("datetime.clear")} data-testid="datetime-clear">
          <X aria-hidden />
        </Button>
      )}
    </div>
  );
}
