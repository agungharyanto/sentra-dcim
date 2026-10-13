import { useTranslation } from "react-i18next";

// Single replacement point for the brand mark (placeholder until the official logo arrives):
// replace src/assets/brand/logo.svg — it is also the favicon (index.html). See docs/design-system.md.
import logoUrl from "@/assets/brand/logo.svg";
import { cn } from "@/lib/utils";

export function Logo({ className, withName = true }: { className?: string; withName?: boolean }) {
  const { t } = useTranslation();
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <img src={logoUrl} alt={withName ? "" : t("app.name")} className="size-7" width={28} height={28} />
      {withName && <span className="font-semibold">{t("app.name")}</span>}
    </span>
  );
}
