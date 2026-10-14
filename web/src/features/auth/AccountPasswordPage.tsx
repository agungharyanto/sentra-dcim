import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "@/components/ui/toast";

import { sessionsQueryKey } from "./api";
import { PasswordForm } from "./PasswordForm";

/** Voluntary password change inside the shell; other sessions are signed out by the server. */
export function AccountPasswordPage() {
  const { t } = useTranslation("auth");
  const qc = useQueryClient();
  return (
    <section className="max-w-md space-y-4" data-testid="account-password-page">
      <p className="text-muted-foreground">{t("password.description")}</p>
      <PasswordForm
        onDone={() => {
          toast.success(t("password.changed"));
          void qc.invalidateQueries({ queryKey: sessionsQueryKey });
        }}
      />
    </section>
  );
}
