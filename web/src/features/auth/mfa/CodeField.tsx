import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";

/** 6-digit authenticator code, or a recovery code when `recovery` is set. */
export function CodeField({
  id,
  value,
  onChange,
  recovery = false,
  invalid,
  autoFocus,
}: {
  id: string;
  value: string;
  onChange: (v: string) => void;
  recovery?: boolean;
  invalid: boolean;
  autoFocus?: boolean;
}) {
  const { t } = useTranslation("auth");
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {recovery ? t("mfa.recoveryLabel") : t("mfa.codeLabel")}
      </label>
      <Input
        id={id}
        value={value}
        onChange={(e) => onChange(recovery ? e.target.value : e.target.value.replace(/[^0-9 ]/g, ""))}
        inputMode={recovery ? "text" : "numeric"}
        autoComplete="one-time-code"
        autoCapitalize="off"
        spellCheck={false}
        maxLength={recovery ? 16 : 7}
        className="font-mono text-lg tracking-widest"
        aria-invalid={invalid ? true : undefined}
        aria-describedby={`${id}-hint`}
        autoFocus={autoFocus}
      />
      <p id={`${id}-hint`} className="text-xs text-muted-foreground">
        {recovery ? t("mfa.recoveryHint") : t("mfa.codeHint")}
      </p>
    </div>
  );
}
