import { useTranslation } from "react-i18next";

import type { MenuStatus } from "@/features/bootstrap/api";
import { cn } from "@/lib/utils";

import { menuStatusStyle } from "./status";

/** Menu availability badge (registry status from GET /api/v1/ui/bootstrap). */
export function MenuStatusBadge({ status, className }: { status: MenuStatus; className?: string }) {
  const { t } = useTranslation("shell");
  const { icon: Icon, badge } = menuStatusStyle[status];
  return (
    <span
      data-menu-status={status}
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap",
        badge,
        className,
      )}
    >
      <Icon className="size-3.5 shrink-0" aria-hidden />
      {t(`menuStatus.${status}`)}
    </span>
  );
}
