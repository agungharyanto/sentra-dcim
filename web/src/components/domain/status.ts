import { CircleCheck, CircleOff, Clock, type LucideIcon, OctagonAlert, TriangleAlert, Unplug, Wrench } from "lucide-react";

import type { MenuStatus } from "@/features/bootstrap/api";

/** DCIM equipment/sensor status. Colour + icon + translated label always go together (never colour alone). */
export const dcimStatuses = ["normal", "warning", "critical", "offline", "maintenance"] as const;
export type DcimStatus = (typeof dcimStatuses)[number];

// Full class strings (Tailwind only sees literal class names). Tokens: src/styles/tokens.css.
export const dcimStatusStyle: Record<DcimStatus, { icon: LucideIcon; badge: string; solid: string }> = {
  normal: {
    icon: CircleCheck,
    badge: "bg-status-normal-bg text-status-normal-fg border-status-normal-border",
    solid: "text-status-normal-solid",
  },
  warning: {
    icon: TriangleAlert,
    badge: "bg-status-warning-bg text-status-warning-fg border-status-warning-border",
    solid: "text-status-warning-solid",
  },
  critical: {
    icon: OctagonAlert,
    badge: "bg-status-critical-bg text-status-critical-fg border-status-critical-border",
    solid: "text-status-critical-solid",
  },
  offline: {
    icon: CircleOff,
    badge: "bg-status-offline-bg text-status-offline-fg border-status-offline-border",
    solid: "text-status-offline-solid",
  },
  maintenance: {
    icon: Wrench,
    badge: "bg-status-maintenance-bg text-status-maintenance-fg border-status-maintenance-border",
    solid: "text-status-maintenance-solid",
  },
};

export const menuStatuses = ["available", "coming_soon", "not_connected"] as const satisfies readonly MenuStatus[];

export const menuStatusStyle: Record<MenuStatus, { icon: LucideIcon; badge: string }> = {
  available: {
    icon: CircleCheck,
    badge: "bg-menu-available-bg text-menu-available-fg border-menu-available-border",
  },
  coming_soon: {
    icon: Clock,
    badge: "bg-menu-coming-soon-bg text-menu-coming-soon-fg border-menu-coming-soon-border",
  },
  not_connected: {
    icon: Unplug,
    badge: "bg-menu-not-connected-bg text-menu-not-connected-fg border-menu-not-connected-border border-dashed",
  },
};
