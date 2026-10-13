import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { type DcimStatus, dcimStatusStyle } from "./status";

/**
 * Compact status indicator: the status icon in the solid status colour plus its label. With hideLabel the label
 * is still announced (sr-only) and shown as a native tooltip, so colour is never the only cue.
 */
export function StatusDot({
  status,
  hideLabel = false,
  className,
}: {
  status: DcimStatus;
  hideLabel?: boolean;
  className?: string;
}) {
  const { t } = useTranslation("ds");
  const { icon: Icon, solid } = dcimStatusStyle[status];
  const label = t(`status.${status}`);
  return (
    <span
      data-status={status}
      title={hideLabel ? label : undefined}
      className={cn("inline-flex items-center gap-1.5 text-sm", className)}
    >
      <Icon className={cn("size-4 shrink-0", solid)} aria-hidden data-icon={status} />
      <span className={hideLabel ? "sr-only" : undefined}>{label}</span>
    </span>
  );
}
