import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { useFormatters } from "@/features/bootstrap/regional";

const format: Intl.DateTimeFormatOptions = {
  weekday: "short",
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  timeZoneName: "short",
};

/**
 * Live clock in the server display time zone. Owns its own state, so the 1 s tick re-renders only this component,
 * not the layout. Deliberately no aria-live: screen readers must not announce it every second.
 */
export function Clock() {
  const { t } = useTranslation("shell");
  const fmt = useFormatters();
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <time
      dateTime={now.toISOString()}
      title={t("layout.clockLabel", { timezone: fmt.timezone })}
      data-testid="clock"
      className="font-mono text-xs whitespace-nowrap text-muted-foreground tabular-nums sm:text-sm"
    >
      {fmt.dateTime(now, format)}
    </time>
  );
}
