import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type ReactNode, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { FormMessage } from "@/features/auth/FormMessage";
import { ApiError } from "@/lib/api";

import { createUser, rolesKey, usersKey } from "./api";
import { RoleCheckboxes } from "./RoleCheckboxes";
import { TemporaryPassword } from "./TemporaryPassword";
import { useFieldError } from "./useFieldError";

const empty = { username: "", display_name: "", email: "", roles: [] as string[] };

/** "Add user": form, then the one-time temporary password. Closing clears everything. */
export function CreateUserDialog({ open, onOpenChange, roles }: { open: boolean; onOpenChange: (o: boolean) => void; roles: readonly string[] }) {
  const { t } = useTranslation("users");
  const id = useId();
  const qc = useQueryClient();
  const [form, setForm] = useState(empty);
  const [busy, setBusy] = useState(false);
  const { error, field, setFromError, setMessage, clear } = useFieldError();
  const [created, setCreated] = useState<{ username: string; password: string } | null>(null);

  const close = (o: boolean) => {
    if (!o) {
      setForm(empty);
      setCreated(null);
      clear();
    }
    onOpenChange(o);
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!form.username.trim() || !form.display_name.trim() || form.roles.length === 0) {
      setMessage(t("create.required"));
      return;
    }
    setBusy(true);
    try {
      const res = await createUser(form);
      setCreated({ username: res.user.username, password: res.temporary_password });
      void qc.invalidateQueries({ queryKey: usersKey });
      void qc.invalidateQueries({ queryKey: rolesKey });
    } catch (err) {
      setFromError(err instanceof ApiError ? err : new ApiError("unknown", 0));
    } finally {
      setBusy(false);
    }
  };

  const set = (k: keyof typeof form) => (v: string) => setForm((f) => ({ ...f, [k]: v }));

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl" data-testid="create-user-dialog">
        <DialogHeader>
          <DialogTitle>{created ? t("temp.title") : t("create.title")}</DialogTitle>
          {!created && <DialogDescription>{t("create.description")}</DialogDescription>}
        </DialogHeader>
        {created ? (
          <TemporaryPassword username={created.username} password={created.password} onDone={() => close(false)} />
        ) : (
          <form onSubmit={(e) => void submit(e)} noValidate className="space-y-4">
            {error && (
              <FormMessage tone="error" testId="create-user-error">
                {error}
              </FormMessage>
            )}
            <Field id={`${id}-username`} label={t("create.username")} hint={t("create.usernameHint")}>
              <Input
                id={`${id}-username`}
                autoComplete="off"
                spellCheck={false}
                maxLength={64}
                value={form.username}
                onChange={(e) => set("username")(e.target.value)}
                aria-invalid={field === "username" ? true : undefined}
                aria-describedby={`${id}-username-hint`}
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field id={`${id}-name`} label={t("create.displayName")}>
                <Input
                  id={`${id}-name`}
                  autoComplete="off"
                  maxLength={128}
                  value={form.display_name}
                  onChange={(e) => set("display_name")(e.target.value)}
                  aria-invalid={field === "display_name" ? true : undefined}
                />
              </Field>
              <Field id={`${id}-email`} label={t("create.email")}>
                <Input
                  id={`${id}-email`}
                  type="email"
                  autoComplete="off"
                  maxLength={254}
                  value={form.email}
                  onChange={(e) => set("email")(e.target.value)}
                  aria-invalid={field === "email" ? true : undefined}
                />
              </Field>
            </div>
            <fieldset className="space-y-2">
              <legend className="text-sm font-medium">{t("create.roles")}</legend>
              <p id={`${id}-roles-hint`} className="text-xs text-muted-foreground">
                {t("create.rolesHint")}
              </p>
              <RoleCheckboxes
                roles={roles}
                value={form.roles}
                onChange={(roles) => setForm((f) => ({ ...f, roles }))}
                invalid={field === "roles"}
                describedBy={`${id}-roles-hint`}
              />
            </fieldset>
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="outline" onClick={() => close(false)}>
                {t("create.cancel")}
              </Button>
              <Button type="submit" disabled={busy}>
                {busy ? t("create.submitting") : t("create.submit")}
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Field({ id, label, hint, children }: { id: string; label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      {children}
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
    </div>
  );
}
