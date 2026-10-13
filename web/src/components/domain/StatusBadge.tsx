import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import { type DcimStatus, dcimStatusStyle } from "./status";

/** DCIM status badge: icon + translated label on the status colour. */
export function StatusBadge({ status, className }: { status: DcimStatus; className?: string }) {
  const { t } = useTranslation("ds");
  const { icon: Icon, badge } = dcimStatusStyle[status];
  return (
    <span
      data-status={status}
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap",
        badge,
        className,
      )}
    >
      <Icon className="size-3.5 shrink-0" aria-hidden data-icon={status} />
      {t(`status.${status}`)}
    </span>
  );
}
