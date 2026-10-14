import { useState } from "react";
import { useTranslation } from "react-i18next";

import { toErrorCode } from "@/i18n/errors";
import type { ApiError } from "@/lib/api";

const fields = ["username", "display_name", "email", "roles"] as const;
type Field = (typeof fields)[number];

/** Form error state: validation errors name a field (ErrorBody.error.field) and get a field-specific message. */
export function useFieldError() {
  const { t } = useTranslation(["users", "errors"]);
  const [error, setError] = useState<string | null>(null);
  const [field, setField] = useState<Field | null>(null);
  return {
    error,
    field,
    setMessage: (msg: string) => {
      setError(msg);
      setField(null);
    },
    setFromError: (err: ApiError) => {
      const f = fields.find((x) => x === err.field) ?? null;
      setField(f);
      if (err.code === "validation_failed" && f) setError(t(`users:field.${f}`));
      else setError(t(`errors:code.${toErrorCode(err.code)}`));
    },
    clear: () => {
      setError(null);
      setField(null);
    },
  };
}
