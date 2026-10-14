import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { CopyButton } from "@/components/domain/CopyButton";
import { Button } from "@/components/ui/button";

/** The 10 recovery codes, shown once, with copy-all and an explicit "I saved them" confirmation. */
export function RecoveryCodes({ codes, onDone, doneLabel }: { codes: string[]; onDone: () => void; doneLabel: string }) {
  const { t } = useTranslation("auth");
  const id = useId();
  const [saved, setSaved] = useState(false);
  return (
    <div className="space-y-4" data-testid="recovery-codes">
      <p role="note" className="rounded-md border border-status-warning-border bg-status-warning-bg px-3 py-2 text-sm text-status-warning-fg">
        {t("mfa.recoveryWarning")}
      </p>
      <div className="relative rounded-md border bg-muted p-3">
        <ol className="grid grid-cols-2 gap-x-6 gap-y-1 font-mono text-sm" aria-label={t("mfa.recoveryListLabel")}>
          {codes.map((c) => (
            <li key={c} data-testid="recovery-code">
              {c}
            </li>
          ))}
        </ol>
        <CopyButton value={codes.join("\n")} label={t("mfa.copyRecovery")} className="absolute top-2 right-2" />
      </div>
      <label htmlFor={`${id}-saved`} className="flex cursor-pointer items-start gap-2 text-sm">
        <input
          id={`${id}-saved`}
          type="checkbox"
          className="mt-0.5 size-4 accent-primary"
          checked={saved}
          onChange={(e) => setSaved(e.target.checked)}
          data-testid="recovery-saved"
        />
        {t("mfa.recoverySaved")}
      </label>
      <Button className="w-full" disabled={!saved} onClick={onDone} data-testid="recovery-done">
        {doneLabel}
      </Button>
    </div>
  );
}
