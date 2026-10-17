import {
  Archive,
  Bookmark,
  Building,
  Building2,
  Cable,
  CircleCheck,
  CircleOff,
  Clock,
  DoorOpen,
  Layers,
  type LucideIcon,
  Server,
  TriangleAlert,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { Level, RackStatus, RoomType, SiteStatus } from "./api";

const badgeBase = "inline-flex w-fit items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap";

/**
 * Rack lifecycle → design-system status colours (src/styles/tokens.css), always with icon + label (never colour
 * alone): active = normal, decommissioning = warning, reserved = maintenance, planned = "coming soon" style,
 * retired = offline.
 */
export const rackStatusStyle: Record<RackStatus, { icon: LucideIcon; badge: string }> = {
  planned: { icon: Clock, badge: "bg-menu-coming-soon-bg text-menu-coming-soon-fg border-menu-coming-soon-border" },
  active: { icon: CircleCheck, badge: "bg-status-normal-bg text-status-normal-fg border-status-normal-border" },
  reserved: { icon: Bookmark, badge: "bg-status-maintenance-bg text-status-maintenance-fg border-status-maintenance-border" },
  decommissioning: { icon: TriangleAlert, badge: "bg-status-warning-bg text-status-warning-fg border-status-warning-border" },
  retired: { icon: CircleOff, badge: "bg-status-offline-bg text-status-offline-fg border-status-offline-border" },
};

export function RackStatusBadge({ status, className }: { status: RackStatus; className?: string }) {
  const { t } = useTranslation("facility");
  const { icon: Icon, badge } = rackStatusStyle[status];
  return (
    <span data-rack-status={status} className={cn(badgeBase, badge, className)}>
      <Icon className="size-3.5 shrink-0" aria-hidden />
      {t(`rackStatus.${status}`)}
    </span>
  );
}

const siteStatusStyle: Record<SiteStatus, string> = {
  planned: rackStatusStyle.planned.badge,
  active: rackStatusStyle.active.badge,
  inactive: rackStatusStyle.retired.badge,
};

export function SiteStatusBadge({ status }: { status: SiteStatus }) {
  const { t } = useTranslation("facility");
  return (
    <span data-site-status={status} className={cn(badgeBase, siteStatusStyle[status])}>
      {t(`siteStatus.${status}`)}
    </span>
  );
}

export function ArchivedBadge() {
  const { t } = useTranslation("facility");
  return (
    <span data-archived className={cn(badgeBase, rackStatusStyle.retired.badge)}>
      <Archive className="size-3.5 shrink-0" aria-hidden />
      {t("detail.archived")}
    </span>
  );
}

const levelIcons = { site: Building2, building: Building, floor: Layers, room: DoorOpen, rack: Server, mmr: Cable } as const;

/** Icon of a level (static components only; MMR rooms get the cable icon). */
export function LevelIcon({ level, roomType, className }: { level: Level; roomType?: RoomType | undefined; className?: string }) {
  const mmr = level === "room" && roomType === "mmr";
  const key = mmr ? "mmr" : level;
  switch (key) {
    case "site":
      return <levelIcons.site className={className} aria-hidden data-icon={key} />;
    case "building":
      return <levelIcons.building className={className} aria-hidden data-icon={key} />;
    case "floor":
      return <levelIcons.floor className={className} aria-hidden data-icon={key} />;
    case "room":
      return <levelIcons.room className={className} aria-hidden data-icon={key} />;
    case "mmr":
      return <levelIcons.mmr className={className} aria-hidden data-icon={key} />;
    case "rack":
      return <levelIcons.rack className={className} aria-hidden data-icon={key} />;
  }
}
