import { CircleAlert, Info } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/** Inline form message; errors are announced (role=alert), notices politely (role=status). */
export function FormMessage({ tone, children, testId }: { tone: "error" | "info"; children: ReactNode; testId?: string }) {
  const Icon = tone === "error" ? CircleAlert : Info;
  return (
    <div
      role={tone === "error" ? "alert" : "status"}
      data-testid={testId}
      className={cn(
        "flex items-start gap-2 rounded-md border px-3 py-2 text-sm",
        tone === "error" ? "border-destructive/50 bg-destructive/10 text-foreground" : "bg-muted text-foreground",
      )}
    >
      <Icon className={cn("mt-0.5 size-4 shrink-0", tone === "error" ? "text-destructive" : "text-muted-foreground")} aria-hidden />
      <p>{children}</p>
    </div>
  );
}
