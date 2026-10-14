import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

/** Confirmation for consequential actions (disable, enable, reset password). */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  body,
  busy,
  destructive = false,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  title: string;
  body: string;
  busy: boolean;
  destructive?: boolean;
  onConfirm: () => void;
}) {
  const { t } = useTranslation("users");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="confirm-dialog">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{body}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("detail.cancel")}
          </Button>
          <Button variant={destructive ? "destructive" : "default"} disabled={busy} onClick={onConfirm} data-testid="confirm">
            {busy ? t("detail.saving") : t("detail.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
