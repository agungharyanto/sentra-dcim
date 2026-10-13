import { Check, Copy } from "lucide-react";
import { type MouseEvent, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

const COPIED_MS = 2000;

/**
 * Icon button that copies `value` with navigator.clipboard (no library). Shows Check for ~2 s and announces
 * "Copied" politely; a missing or failing clipboard shows an error toast. `label` is the accessible name and
 * tooltip (already translated by the caller, e.g. "Copy path /api/v1/health").
 */
export function CopyButton({ value, label, className }: { value: string; label: string; className?: string }) {
  const { t } = useTranslation("ds");
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | undefined>(undefined);

  useEffect(() => () => window.clearTimeout(timer.current), []);

  const copy = async (e: MouseEvent<HTMLButtonElement>) => {
    // Never let the click reach a surrounding disclosure/row handler.
    e.stopPropagation();
    e.preventDefault();
    try {
      if (!navigator.clipboard?.writeText) throw new Error("clipboard unavailable");
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopied(false), COPIED_MS);
    } catch {
      toast.error(t("copy.failed", { value }));
    }
  };

  const Icon = copied ? Check : Copy;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          onClick={copy}
          aria-label={label}
          data-copied={copied}
          className={cn(
            "inline-flex size-7 shrink-0 items-center justify-center rounded-md transition-colors outline-none hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring",
            // The "copied" colour must win over the hover colour, so hover text colour applies only when idle.
            copied ? "text-status-normal-solid" : "text-muted-foreground hover:text-accent-foreground",
            className,
          )}
        >
          <Icon className="size-4" aria-hidden />
          <span className="sr-only" aria-live="polite">
            {copied ? t("copy.copied") : ""}
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent>{copied ? t("copy.copied") : label}</TooltipContent>
    </Tooltip>
  );
}
