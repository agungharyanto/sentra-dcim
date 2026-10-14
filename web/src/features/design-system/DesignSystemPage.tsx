import { ChevronDown, Info } from "lucide-react";
import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";
import { toast } from "@/components/ui/toast";

import { LanguageSwitcher } from "@/components/domain/LanguageSwitcher";
import { CopyButton } from "@/components/domain/CopyButton";
import { Logo } from "@/components/domain/Logo";
import { MenuStatusBadge } from "@/components/domain/MenuStatusBadge";
import { dcimStatuses, menuStatuses } from "@/components/domain/status";
import { StatusBadge } from "@/components/domain/StatusBadge";
import { StatusDot } from "@/components/domain/StatusDot";
import { ThemeSwitcher } from "@/components/domain/ThemeSwitcher";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useBootstrap, useFormatters } from "@/features/bootstrap/regional";

// Token names are code identifiers (shown as-is), not UI text.
const tokenGroups: string[][] = [
  ["background", "foreground", "card", "muted", "muted-foreground", "border", "input", "ring"],
  ["primary", "primary-foreground", "secondary", "accent", "destructive", "link", "info", "info-bg"],
  ...dcimStatuses.map((s) => ["solid", "fg", "bg", "border"].map((p) => `status-${s}-${p}`)),
];

// Fictional sample identifier for the copy demo.
const sampleDeviceId = "PDU-A01-02";

// Fictional sample rows for the table demo.
const racks = [
  { rack: "RAK-A01", kw: 3.2, temp: 22.4, status: "normal" },
  { rack: "RAK-A02", kw: 5.8, temp: 27.9, status: "warning" },
  { rack: "RAK-B07", kw: 7.1, temp: 33.5, status: "critical" },
  { rack: "RAK-C03", kw: 0, temp: 0, status: "offline" },
  { rack: "RAK-C04", kw: 1.4, temp: 21.0, status: "maintenance" },
] as const;

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={`ds-${id}`} data-section={id}>
      <Card>
        <CardHeader>
          <CardTitle>
            <h2 id={`ds-${id}`} className="text-base">
              {title}
            </h2>
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-start gap-3">{children}</CardContent>
      </Card>
    </section>
  );
}

/** /design-system — internal developer showcase (not in the menu registry; ISS-029 restricts it from v0.3). */
export function DesignSystemPage() {
  const { t } = useTranslation(["ds", "common"]);
  const boot = useBootstrap();
  const fmt = useFormatters();
  const [density, setDensity] = useState("comfortable");

  return (
    <div className="min-h-dvh bg-background text-foreground">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b px-6 py-4">
        <Logo />
        <div className="flex flex-wrap items-center gap-2">
          <ThemeSwitcher />
          <LanguageSwitcher serverLocales={boot.supported_locales} />
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 p-6">
        <div className="space-y-2">
          <h1 className="text-2xl font-semibold tracking-tight">{t("showcase.title")}</h1>
          <p className="text-muted-foreground">{t("showcase.subtitle")}</p>
          <p className="flex items-center gap-2 text-sm text-info-fg">
            <Info className="size-4 text-info" aria-hidden />
            {t("showcase.statusRule")}
          </p>
        </div>

        <Section id="status" title={t("showcase.sections.status")}>
          <div className="grid w-full gap-4 sm:grid-cols-3">
            <div className="flex flex-col gap-2" data-testid="status-badges">
              <p className="text-sm text-muted-foreground">{t("showcase.status.badge")}</p>
              {dcimStatuses.map((s) => (
                <StatusBadge key={s} status={s} />
              ))}
            </div>
            <div className="flex flex-col gap-2">
              <p className="text-sm text-muted-foreground">{t("showcase.status.dot")}</p>
              {dcimStatuses.map((s) => (
                <StatusDot key={s} status={s} />
              ))}
            </div>
            <div className="flex flex-col gap-2">
              <p className="text-sm text-muted-foreground">{t("showcase.status.compact")}</p>
              <div className="flex gap-3">
                {dcimStatuses.map((s) => (
                  <StatusDot key={s} status={s} hideLabel />
                ))}
              </div>
            </div>
          </div>
        </Section>

        <Section id="menu-status" title={t("showcase.sections.menuStatus")}>
          {menuStatuses.map((s) => (
            <MenuStatusBadge key={s} status={s} />
          ))}
        </Section>

        <Section id="tokens" title={t("showcase.sections.tokens")}>
          <div className="grid w-full gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {tokenGroups.flat().map((name) => (
              <div key={name} className="flex items-center gap-2 text-xs">
                <span
                  className="size-6 shrink-0 rounded border border-input"
                  data-token={name}
                  ref={(el) => el?.style.setProperty("background", `var(--${name})`)}
                />
                <code className="font-mono">{`--${name}`}</code>
              </div>
            ))}
          </div>
        </Section>

        <Section id="typography" title={t("showcase.sections.typography")}>
          <div className="space-y-2">
            <p className="text-2xl font-semibold">{t("showcase.typography.heading")}</p>
            <p>{t("showcase.typography.sans")}</p>
            <p className="font-mono text-sm">{t("showcase.typography.mono")}</p>
          </div>
        </Section>

        <Section id="buttons" title={t("showcase.sections.buttons")}>
          <Button>{t("showcase.buttons.default")}</Button>
          <Button variant="secondary">{t("showcase.buttons.secondary")}</Button>
          <Button variant="outline">{t("showcase.buttons.outline")}</Button>
          <Button variant="ghost">{t("showcase.buttons.ghost")}</Button>
          <Button variant="destructive">{t("showcase.buttons.destructive")}</Button>
          <Button variant="link">{t("showcase.buttons.link")}</Button>
          <Button disabled>{t("showcase.buttons.disabled")}</Button>
        </Section>

        <Section id="badges" title={t("showcase.sections.badges")}>
          <Badge>{t("showcase.badges.default")}</Badge>
          <Badge variant="secondary">{t("showcase.badges.secondary")}</Badge>
          <Badge variant="outline">{t("showcase.badges.outline")}</Badge>
          <Badge variant="destructive">{t("showcase.badges.destructive")}</Badge>
        </Section>

        <Section id="forms" title={t("showcase.sections.forms")}>
          <div className="grid w-full gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <label htmlFor="ds-rack" className="text-sm font-medium">
                {t("showcase.form.rackLabel")}
              </label>
              <Input id="ds-rack" placeholder={t("showcase.form.rackPlaceholder")} />
            </div>
            <div className="grid gap-1.5">
              <label htmlFor="ds-site" className="text-sm font-medium">
                {t("showcase.form.siteLabel")}
              </label>
              <Select>
                <SelectTrigger id="ds-site" className="w-full" data-testid="site-select">
                  <SelectValue placeholder={t("showcase.form.sitePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="main">{t("showcase.form.sites.main")}</SelectItem>
                  <SelectItem value="drc">{t("showcase.form.sites.drc")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        </Section>

        <Section id="tabs" title={t("showcase.sections.tabs")}>
          <Tabs defaultValue="overview" className="w-full">
            <TabsList>
              <TabsTrigger value="overview">{t("showcase.tabs.overview")}</TabsTrigger>
              <TabsTrigger value="power">{t("showcase.tabs.power")}</TabsTrigger>
              <TabsTrigger value="environment">{t("showcase.tabs.environment")}</TabsTrigger>
            </TabsList>
            <TabsContent value="overview">{t("showcase.tabs.overviewBody")}</TabsContent>
            <TabsContent value="power">{t("showcase.tabs.powerBody")}</TabsContent>
            <TabsContent value="environment">{t("showcase.tabs.environmentBody")}</TabsContent>
          </Tabs>
        </Section>

        <Section id="table" title={t("showcase.sections.table")}>
          <Table>
            <TableCaption>{t("showcase.table.caption")}</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead>{t("showcase.table.rack")}</TableHead>
                <TableHead className="text-right">{t("showcase.table.power")}</TableHead>
                <TableHead className="text-right">{t("showcase.table.temperature")}</TableHead>
                <TableHead>{t("showcase.table.status")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {racks.map((r) => (
                <TableRow key={r.rack}>
                  <TableCell className="font-mono">{r.rack}</TableCell>
                  <TableCell className="text-right tabular-nums">
                    {t("showcase.table.kw", { value: fmt.number(r.kw, { minimumFractionDigits: 1 }) })}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {t("showcase.table.celsius", { value: fmt.number(r.temp, { minimumFractionDigits: 1 }) })}
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={r.status} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Section>

        <Section id="overlays" title={t("showcase.sections.overlays")}>
          <Dialog>
            <DialogTrigger asChild>
              <Button variant="outline">{t("showcase.overlays.openDialog")}</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("showcase.overlays.dialogTitle")}</DialogTitle>
                <DialogDescription>{t("showcase.overlays.dialogBody")}</DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <DialogClose asChild>
                  <Button variant="outline">{t("common:actions.cancel")}</Button>
                </DialogClose>
                <DialogClose asChild>
                  <Button>{t("common:actions.confirm")}</Button>
                </DialogClose>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          <Sheet>
            <SheetTrigger asChild>
              <Button variant="outline">{t("showcase.overlays.openSheet")}</Button>
            </SheetTrigger>
            <SheetContent>
              <SheetHeader>
                <SheetTitle>{t("showcase.overlays.sheetTitle")}</SheetTitle>
                <SheetDescription>{t("showcase.overlays.sheetBody")}</SheetDescription>
              </SheetHeader>
              <div className="px-4">
                <StatusBadge status="maintenance" />
              </div>
            </SheetContent>
          </Sheet>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost">{t("showcase.overlays.tooltipTrigger")}</Button>
            </TooltipTrigger>
            <TooltipContent>{t("showcase.overlays.tooltip")}</TooltipContent>
          </Tooltip>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline">
                {t("showcase.overlays.menu")}
                <ChevronDown aria-hidden />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuItem>{t("showcase.overlays.menuItems.ack")}</DropdownMenuItem>
              <DropdownMenuItem>{t("showcase.overlays.menuItems.assign")}</DropdownMenuItem>
              <DropdownMenuItem>{t("showcase.overlays.menuItems.export")}</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuLabel>{t("showcase.overlays.density")}</DropdownMenuLabel>
              <DropdownMenuRadioGroup value={density} onValueChange={setDensity}>
                <DropdownMenuRadioItem value="comfortable">
                  {t("showcase.overlays.densityOptions.comfortable")}
                </DropdownMenuRadioItem>
                <DropdownMenuRadioItem value="compact">
                  {t("showcase.overlays.densityOptions.compact")}
                </DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </Section>

        <Section id="feedback" title={t("showcase.sections.feedback")}>
          <Button
            variant="secondary"
            onClick={() =>
              toast(t("showcase.feedback.toastTitle"), { description: t("showcase.feedback.toastBody") })
            }
          >
            {t("showcase.feedback.showToast")}
          </Button>
          <span className="inline-flex items-center gap-1 rounded-md border px-2 py-1" data-testid="copy-demo">
            <code className="font-mono text-sm">{sampleDeviceId}</code>
            <CopyButton value={sampleDeviceId} label={t("showcase.copyDemo")} />
          </span>
          <div className="w-full space-y-2" aria-label={t("showcase.feedback.skeleton")} role="status">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-24 w-full" />
          </div>
        </Section>
      </main>
    </div>
  );
}
