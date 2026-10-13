import {
  Building2,
  ChartLine,
  Circle,
  CircleHelp,
  DoorClosed,
  IdCard,
  LayoutDashboard,
  type LucideIcon,
  Server,
  Settings,
  Siren,
  Thermometer,
  Users,
  Wrench,
  Zap,
} from "lucide-react";

// Registry icon names (GET /api/v1/ui/bootstrap, lucide kebab-case) → components. Explicit map keeps the bundle
// free of the full icon set; unknown names fall back to a neutral circle.
const icons: Record<string, LucideIcon> = {
  "layout-dashboard": LayoutDashboard,
  siren: Siren,
  zap: Zap,
  thermometer: Thermometer,
  "building-2": Building2,
  server: Server,
  users: Users,
  "door-closed": DoorClosed,
  "id-card": IdCard,
  wrench: Wrench,
  "chart-line": ChartLine,
  "circle-help": CircleHelp,
  settings: Settings,
};

export function menuIcon(name: string): LucideIcon {
  return icons[name] ?? Circle;
}
