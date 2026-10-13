import { useTranslation } from "react-i18next";
import { Navigate, useNavigate, useSearchParams } from "react-router";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { safeNext, useAuth } from "./auth-context";
import { AuthLayout } from "./AuthLayout";
import { FormMessage } from "./FormMessage";
import { PasswordForm } from "./PasswordForm";

/** Shown after login while must_change_password is set; every other API call answers 403 until it is done. */
export function ForcedPasswordPage() {
  const { t } = useTranslation("auth");
  const { me, refresh, signOut } = useAuth();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const next = safeNext(params.get("next"));

  if (me && !me.user.must_change_password) return <Navigate to={next} replace />;

  return (
    <AuthLayout title={t("forced.title")}>
      <Card data-testid="forced-password-page">
        <CardHeader>
          <CardTitle>
            <h1 className="text-xl">{t("forced.title")}</h1>
          </CardTitle>
          <CardDescription>{t("forced.description", { name: me?.user.display_name ?? "" })}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <FormMessage tone="info">{t("forced.why")}</FormMessage>
          <PasswordForm
            onDone={async () => {
              toast.success(t("password.changed"));
              await refresh();
              await navigate(next, { replace: true });
            }}
          />
          <Button variant="link" className="h-auto px-0" onClick={() => void signOut()}>
            {t("user.logout")}
          </Button>
        </CardContent>
      </Card>
    </AuthLayout>
  );
}
