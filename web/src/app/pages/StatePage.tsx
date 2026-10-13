import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

/** Standard centred state page (coming soon, not connected, not found, error, empty states). */
export function StatePage({
  icon: Icon,
  title,
  body,
  action,
  testId,
}: {
  icon: LucideIcon;
  title: string;
  body: string;
  action?: ReactNode;
  testId?: string;
}) {
  return (
    <section
      data-testid={testId}
      className="mx-auto flex max-w-lg flex-col items-center gap-3 rounded-xl border border-dashed bg-card px-6 py-12 text-center"
    >
      <Icon className="size-10 text-muted-foreground" aria-hidden />
      <h2 className="text-lg font-semibold">{title}</h2>
      <p className="text-muted-foreground">{body}</p>
      {action}
    </section>
  );
}
