import { CircleAlert, CircleCheck, Info, X } from "lucide-react";
import { useSyncExternalStore } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

/**
 * Minimal toast notifications (replaces `sonner`, which injects a <style> element at runtime and cannot carry a CSP
 * nonce — ISS-030, D-065). Styling is plain Tailwind classes from the design tokens; nothing is injected.
 * API: toast("…", { description }), toast.success("…"), toast.error("…").
 */
type Tone = "default" | "success" | "error";

interface ToastItem {
  id: number;
  tone: Tone;
  title: string;
  description?: string;
}

interface Options {
  description?: string;
  /** Milliseconds before the toast closes itself (default 4 s, errors 8 s). */
  duration?: number;
}

const MAX_VISIBLE = 4;
let items: ToastItem[] = [];
let nextId = 1;
const listeners = new Set<() => void>();

function emit() {
  for (const l of listeners) l();
}

function dismiss(id: number) {
  items = items.filter((t) => t.id !== id);
  emit();
}

function push(tone: Tone, title: string, opts: Options = {}) {
  const id = nextId++;
  items = [...items, { id, tone, title, ...(opts.description ? { description: opts.description } : {}) }].slice(-MAX_VISIBLE);
  emit();
  window.setTimeout(() => dismiss(id), opts.duration ?? (tone === "error" ? 8000 : 4000));
  return id;
}

export const toast = Object.assign((title: string, opts?: Options) => push("default", title, opts), {
  success: (title: string, opts?: Options) => push("success", title, opts),
  error: (title: string, opts?: Options) => push("error", title, opts),
  dismiss,
});

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

const icons = { default: Info, success: CircleCheck, error: CircleAlert } as const;

/** Toast host: fixed bottom-right (bottom on phones). Errors are announced assertively, others politely. */
export function Toaster() {
  const { t } = useTranslation();
  const list = useSyncExternalStore(subscribe, () => items, () => items);
  return (
    <section
      aria-label={t("toast.region")}
      className="pointer-events-none fixed inset-x-4 bottom-4 z-[100] flex flex-col items-stretch gap-2 sm:inset-x-auto sm:right-4 sm:w-96"
    >
      {list.map((item) => {
        const Icon = icons[item.tone];
        return (
          <div
            key={item.id}
            role={item.tone === "error" ? "alert" : "status"}
            data-toast={item.tone}
            className="pointer-events-auto flex items-start gap-3 rounded-lg border bg-popover p-4 text-sm text-popover-foreground shadow-lg animate-in fade-in-0 slide-in-from-bottom-2 motion-reduce:animate-none"
          >
            <Icon
              className={cn(
                "mt-0.5 size-4 shrink-0",
                item.tone === "success" && "text-status-normal-solid",
                item.tone === "error" && "text-destructive",
                item.tone === "default" && "text-muted-foreground",
              )}
              aria-hidden
            />
            <div className="min-w-0 flex-1">
              <p className="font-medium">{item.title}</p>
              {item.description && <p className="mt-1 text-muted-foreground">{item.description}</p>}
            </div>
            <button
              type="button"
              onClick={() => dismiss(item.id)}
              aria-label={t("actions.close")}
              className="-m-1 rounded p-1 text-muted-foreground outline-none hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring"
            >
              <X className="size-4" aria-hidden />
            </button>
          </div>
        );
      })}
    </section>
  );
}
