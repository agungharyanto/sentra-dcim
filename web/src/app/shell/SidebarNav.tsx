import { createElement } from "react";
import { useTranslation } from "react-i18next";
import { NavLink, useMatch } from "react-router";

import { MenuStatusBadge } from "@/components/domain/MenuStatusBadge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { canSeeMenu, usePermissions } from "@/features/auth/permissions";
import type { MenuItem } from "@/features/bootstrap/api";
import { useBootstrap } from "@/features/bootstrap/regional";
import { cn } from "@/lib/utils";

import { menuIcon } from "./icons";
import { groupMenus, useMenuText } from "./menu";

/**
 * Grouped menu from the backend registry, without menus the user has no permission for; shared by the desktop
 * sidebar and the mobile drawer.
 */
export function SidebarNav({ collapsed = false, onNavigate }: { collapsed?: boolean; onNavigate?: () => void }) {
  const boot = useBootstrap();
  const perms = usePermissions();
  const { groupLabel } = useMenuText();

  return (
    <div className="flex flex-col gap-4">
      {groupMenus(
        boot.menu_groups,
        boot.menus.filter((m) => canSeeMenu(m, perms)),
      ).map(({ group, items }) => (
        <div key={group.key} role="group" aria-labelledby={`nav-group-${group.key}`} data-group={group.key}>
          <p
            id={`nav-group-${group.key}`}
            className={cn(
              "px-3 pb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase",
              collapsed && "sr-only",
            )}
          >
            {groupLabel(group.key)}
          </p>
          <ul className="flex flex-col gap-0.5">
            {items.map((m) => (
              <li key={m.key}>
                <MenuLink menu={m} collapsed={collapsed} onNavigate={onNavigate} />
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

const iconProps = { className: "size-4 shrink-0", "aria-hidden": true } as const;

/**
 * One menu entry. className must be a plain string: in collapsed mode the link is a Radix TooltipTrigger (asChild),
 * and Radix Slot stringifies a function className (NavLink's render-prop form), dropping every style.
 */
function MenuLink({
  menu: m,
  collapsed,
  onNavigate,
}: {
  menu: MenuItem;
  collapsed: boolean;
  onNavigate?: (() => void) | undefined;
}) {
  const { t } = useTranslation("shell");
  const { menuLabel } = useMenuText();
  const active = useMatch({ path: m.path, end: false }) !== null;
  const label = menuLabel(m.key);

  const link = (
    <NavLink
      to={m.path}
      onClick={onNavigate}
      data-menu={m.key}
      className={cn(
        "flex items-center gap-3 rounded-md px-3 py-2 text-sm no-underline transition-colors outline-none hover:no-underline focus-visible:ring-[3px] focus-visible:ring-ring",
        active
          ? "bg-accent font-medium text-accent-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-accent-foreground",
        collapsed && "justify-center px-2",
      )}
    >
      {createElement(menuIcon(m.icon), iconProps)}
      <span className={cn("flex-1 truncate", collapsed && "sr-only")}>{label}</span>
      {m.status !== "available" &&
        (collapsed ? (
          <span className="sr-only">{t(`menuStatus.${m.status}`)}</span>
        ) : (
          <MenuStatusBadge status={m.status} className="px-1.5 text-[0.7rem]" />
        ))}
    </NavLink>
  );

  if (!collapsed) return link;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{link}</TooltipTrigger>
      <TooltipContent side="right">{label}</TooltipContent>
    </Tooltip>
  );
}
