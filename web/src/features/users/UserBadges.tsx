import { Badge } from "@/components/ui/badge";
import { useTranslation } from "react-i18next";

import type { User } from "./api";
import { useUserText } from "./text";

export function RoleBadges({ roles }: { roles: string[] }) {
  const { roleLabel } = useUserText();
  return (
    <span className="flex flex-wrap gap-1">
      {roles.map((r) => (
        <Badge key={r} variant="secondary" data-role={r}>
          {roleLabel(r)}
        </Badge>
      ))}
    </span>
  );
}

/** Status + lockout/forced-change markers. Text always accompanies colour. */
export function UserStatusBadges({ user }: { user: User }) {
  const { t } = useTranslation("users");
  return (
    <span className="flex flex-wrap gap-1" data-status={user.status}>
      <Badge
        variant="outline"
        className={
          user.status === "active"
            ? "border-status-normal-border bg-status-normal-bg text-status-normal-fg"
            : "border-status-offline-border bg-status-offline-bg text-status-offline-fg"
        }
      >
        {t(`status.${user.status}`)}
      </Badge>
      {user.locked && (
        <Badge variant="outline" className="border-status-warning-border bg-status-warning-bg text-status-warning-fg">
          {t("status.locked")}
        </Badge>
      )}
      {user.must_change_password && <Badge variant="outline">{t("status.mustChange")}</Badge>}
      {!user.mfa_enabled && (
        <Badge variant="outline" className="border-status-warning-border bg-status-warning-bg text-status-warning-fg" data-mfa="off">
          {t("status.mfaOff")}
        </Badge>
      )}
    </span>
  );
}
