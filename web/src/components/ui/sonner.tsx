import { Toaster as Sonner, type ToasterProps } from "sonner";

import { useTheme } from "@/components/domain/theme-context";

/** Toast host (shadcn "sonner"); colours follow the design tokens and the active theme. */
function Toaster(props: ToasterProps) {
  const { resolved } = useTheme();
  return (
    <Sonner
      theme={resolved}
      className="toaster group"
      toastOptions={{
        classNames: {
          toast: "!bg-popover !text-popover-foreground !border-border",
          description: "!text-muted-foreground",
        },
      }}
      {...props}
    />
  );
}

export { Toaster };
