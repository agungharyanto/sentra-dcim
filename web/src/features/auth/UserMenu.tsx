import { KeyRound, LogOut, MonitorSmartphone, ShieldCheck, SlidersHorizontal, UserRound } from "lucide-react";
import { useContext } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { AuthContext } from "./auth-context";

/** Signed-in user (display name) with links to own sessions / password and sign-out. */
export function UserMenu() {
  const { t } = useTranslation("auth");
  // Tolerates a missing provider (shell rendered alone in tests): no session, no menu.
  const auth = useContext(AuthContext);
  if (!auth?.me) return null;
  const { me, signOut } = auth;
  const name = me.user.display_name || me.user.username;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" className="max-w-48 gap-2 px-2.5" aria-label={t("user.menuLabel", { name })} data-testid="user-menu">
          <UserRound aria-hidden />
          <span className="truncate" data-testid="user-name">
            {name}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="space-y-0.5">
          <span className="block truncate font-medium">{name}</span>
          <span className="block truncate text-xs font-normal text-muted-foreground">{me.user.username}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/account/sessions">
            <MonitorSmartphone aria-hidden />
            {t("sessions.title")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/account/security">
            <ShieldCheck aria-hidden />
            {t("security.title")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/account/preferences">
            <SlidersHorizontal aria-hidden />
            {t("preferences.title")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to="/account/password">
            <KeyRound aria-hidden />
            {t("password.title")}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          data-testid="logout"
          // The route guard sends the signed-out user to /login (with this page as next).
          onSelect={() => void signOut()}
        >
          <LogOut aria-hidden />
          {t("user.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
