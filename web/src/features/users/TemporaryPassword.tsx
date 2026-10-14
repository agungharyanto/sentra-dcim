import { Eye, EyeOff, KeyRound } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

import { CopyButton } from "@/components/domain/CopyButton";
import { Button } from "@/components/ui/button";

/** Placeholder with the shape of a temporary password (xxxx-xxxx-xxxx-xxxx); the real value is not in the DOM. */
const MASK = "••••-••••-••••-••••";

/**
 * Shows a temporary password exactly once (create / reset): hidden by default, revealed only on request (D-071);
 * the copy button copies it without displaying it. The value only lives in component state of the open dialog; it is
 * never stored, logged or put in a URL.
 */
export function TemporaryPassword({ username, password, onDone }: { username: string; password: string; onDone: () => void }) {
  const { t } = useTranslation(["users", "common"]);
  const [shown, setShown] = useState(false);
  const Icon = shown ? EyeOff : Eye;
  return (
    <div className="space-y-4" data-testid="temporary-password">
      <p className="text-sm text-muted-foreground">{t("temp.forUser", { username })}</p>
      <div className="flex items-center gap-2 rounded-md border bg-muted px-3 py-2">
        <KeyRound className="size-4 shrink-0 text-muted-foreground" aria-hidden />
        <code
          aria-label={t("temp.label")}
          className="flex-1 font-mono text-base tracking-wide select-all"
          data-testid="temporary-password-value"
          data-shown={shown}
        >
          {shown ? password : MASK}
        </code>
        <button
          type="button"
          onClick={() => setShown((s) => !s)}
          aria-label={shown ? t("common:password.hide") : t("common:password.show")}
          aria-pressed={shown}
          data-testid="password-toggle"
          className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-[3px] focus-visible:ring-ring"
        >
          <Icon className="size-4" aria-hidden />
        </button>
        <CopyButton value={password} label={t("temp.copy")} secret />
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
