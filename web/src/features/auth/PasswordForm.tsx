import { type FormEvent, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { PasswordInput } from "@/components/domain/PasswordInput";
import { Button } from "@/components/ui/button";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { changePassword, PASSWORD_MAX, PASSWORD_MIN } from "./api";
import { FormMessage } from "./FormMessage";

/** Unicode code points, as counted by the server policy. */
const length = (s: string) => [...s].length;

/** Current + new + confirmation. Validates the policy locally; the server stays the authority. */
export function PasswordForm({ onDone }: { onDone: () => void | Promise<void> }) {
  const { t } = useTranslation(["auth", "errors"]);
  const id = useId();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const validate = (): string | null => {
    if (!current || !next || !confirm) return t("password.required");
    if (length(next) < PASSWORD_MIN || length(next) > PASSWORD_MAX) {
      return t("password.policy", { min: PASSWORD_MIN, max: PASSWORD_MAX });
    }
    if (next === current) return t("password.same");
    if (next !== confirm) return t("password.mismatch");
    return null;
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const problem = validate();
    setError(problem);
    if (problem) return;
    setBusy(true);
    try {
      await changePassword(current, next);
      setCurrent("");
      setNext("");
      setConfirm("");
      await onDone();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "unknown";
      if (code === "invalid_current_password") setError(t("password.wrongCurrent"));
      else if (code === "password_policy") setError(t("password.policy", { min: PASSWORD_MIN, max: PASSWORD_MAX }));
      else setError(t(`errors:code.${toErrorCode(code)}`));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={(e) => void submit(e)} noValidate className="space-y-4" data-testid="password-form">
      {error && (
        <FormMessage tone="error" testId="password-error">
          {error}
        </FormMessage>
      )}
      <PasswordField id={`${id}-current`} label={t("password.current")} value={current} onChange={setCurrent} autoComplete="current-password" invalid={!!error} />
      <PasswordField
        id={`${id}-new`}
        label={t("password.new")}
        hint={t("password.hint", { min: PASSWORD_MIN, max: PASSWORD_MAX })}
        value={next}
        onChange={setNext}
        autoComplete="new-password"
        invalid={!!error}
      />
      <PasswordField id={`${id}-confirm`} label={t("password.confirm")} value={confirm} onChange={setConfirm} autoComplete="new-password" invalid={!!error} />
      <Button type="submit" className="w-full sm:w-auto" disabled={busy}>
        {busy ? t("password.submitting") : t("password.submit")}
      </Button>
    </form>
  );
}

function PasswordField(props: {
  id: string;
  label: string;
  hint?: string;
  value: string;
  onChange: (v: string) => void;
  autoComplete: string;
  invalid: boolean;
}) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={props.id} className="text-sm font-medium">
        {props.label}
      </label>
      <PasswordInput
        id={props.id}
        autoComplete={props.autoComplete}
        required
        maxLength={PASSWORD_MAX * 4}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        aria-invalid={props.invalid ? true : undefined}
        aria-describedby={props.hint ? `${props.id}-hint` : undefined}
      />
      {props.hint && (
        <p id={`${props.id}-hint`} className="text-xs text-muted-foreground">
          {props.hint}
        </p>
      )}
    </div>
  );
}
