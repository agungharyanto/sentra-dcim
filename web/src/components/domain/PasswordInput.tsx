import { Eye, EyeOff } from "lucide-react";
import { type ComponentProps, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

/**
 * Password field with a show/hide toggle (D-071). Hidden by default; hidden again when its form is submitted. The
 * `autoComplete` of the caller (current-password / new-password) is kept so browser password managers still work.
 */
export function PasswordInput({ className, ...props }: Omit<ComponentProps<"input">, "type">) {
  const { t } = useTranslation();
  const [shown, setShown] = useState(false);
  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const form = ref.current?.form;
    if (!form) return;
    const hide = () => setShown(false);
    form.addEventListener("submit", hide);
    return () => form.removeEventListener("submit", hide);
  }, []);

  const Icon = shown ? EyeOff : Eye;
  return (
    <div className="relative">
      <Input ref={ref} type={shown ? "text" : "password"} className={cn("pr-10", className)} {...props} />
      <button
        type="button"
        onClick={() => setShown((s) => !s)}
        aria-label={shown ? t("password.hide") : t("password.show")}
        aria-pressed={shown}
        aria-controls={props.id}
        data-testid="password-toggle"
        className="absolute inset-y-0 right-0 flex w-10 items-center justify-center rounded-r-md text-muted-foreground outline-none hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring"
      >
        <Icon className="size-4" aria-hidden />
      </button>
    </div>
  );
}
