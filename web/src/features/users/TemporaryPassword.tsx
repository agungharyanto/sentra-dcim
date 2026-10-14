import { KeyRound } from "lucide-react";
import { useTranslation } from "react-i18next";

import { CopyButton } from "@/components/domain/CopyButton";
import { Button } from "@/components/ui/button";

/**
 * Shows a temporary password exactly once (create / reset). The value only lives in component state of the open
 * dialog; it is never stored, logged or put in a URL.
 */
export function TemporaryPassword({ username, password, onDone }: { username: string; password: string; onDone: () => void }) {
  const { t } = useTranslation("users");
  return (
    <div className="space-y-4" data-testid="temporary-password">
      <p className="text-sm text-muted-foreground">{t("temp.forUser", { username })}</p>
      <div className="flex items-center gap-2 rounded-md border bg-muted px-3 py-2">
        <KeyRound className="size-4 shrink-0 text-muted-foreground" aria-hidden />
        <code aria-label={t("temp.label")} className="flex-1 font-mono text-base tracking-wide select-all" data-testid="temporary-password-value">
          {password}
        </code>
        <CopyButton value={password} label={t("temp.copy")} />
      </div>
      <p role="note" className="rounded-md border border-status-warning-border bg-status-warning-bg px-3 py-2 text-sm text-status-warning-fg">
        {t("temp.warning")}
      </p>
      <div className="flex justify-end">
        <Button onClick={onDone}>{t("temp.done")}</Button>
      </div>
    </div>
  );
}
