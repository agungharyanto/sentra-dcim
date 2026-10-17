import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Archive, Pencil, Plus, TextCursorInput } from "lucide-react";
import { type ReactNode, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "@/components/ui/toast";
import { useGlobalPermissions, usePermissions } from "@/features/auth/permissions";
import { useFormatters } from "@/features/bootstrap/regional";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import {
  archiveEntity,
  type Building,
  type Entity,
  facilityKey,
  fetchEntity,
  type Floor,
  type Level,
  PERM_FACILITY_MANAGE,
  PERM_RACK_MANAGE,
  type Rack,
  type Room,
  type Site,
} from "./api";
import { ArchivedBadge, LevelIcon, RackStatusBadge, SiteStatusBadge } from "./badges";
import { ConflictNotice, type DialogTarget, EntityDialog } from "./EntityDialog";
import type { Node } from "./FacilityTree";

const childLevel: Partial<Record<Level, Level>> = { site: "building", building: "floor", floor: "room", room: "rack" };
const parentField: Record<Level, string> = { site: "site_id", building: "building_id", floor: "floor_id", room: "room_id", rack: "" };

/** Right pane of the hierarchy: details, location, contents and the actions the user may take. */
export function DetailPanel({ level, id, node, onSelect }: { level: Level; id: string; node?: Node | undefined; onSelect: (key: string) => void }) {
  const { t } = useTranslation(["facility", "errors", "common"]);
  const perms = usePermissions();
  const globalPerms = useGlobalPermissions();
  const q = useQuery({ queryKey: [...facilityKey, level, id], queryFn: ({ signal }) => fetchEntity(level, id, signal) });
  const [dialog, setDialog] = useState<DialogTarget | null>(null);
  const [archiving, setArchiving] = useState(false);

  if (q.isPending) return <Skeleton className="h-64 w-full" />;
  if (q.isError) {
    const code = q.error instanceof ApiError ? q.error.code : "unknown";
    return (
      <p role="alert" className="text-sm text-muted-foreground" data-testid="facility-detail-error">
        {code === "not_found" ? t("detail.notFound") : t(`errors:code.${toErrorCode(code)}`)}
      </p>
    );
  }
  const e = q.data;
  const archived = e.archived_at !== null;
  const canManage = perms.has(level === "rack" ? PERM_RACK_MANAGE : PERM_FACILITY_MANAGE) && !archived;
  const child = childLevel[level];
  const canAddChild = !!child && !archived && perms.has(child === "rack" ? PERM_RACK_MANAGE : PERM_FACILITY_MANAGE);
  const roomType = level === "room" ? (e as Room).room_type : undefined;
  const siteId = "site_id" in e ? e.site_id : e.id;

  return (
    <section className="space-y-4" data-testid="facility-detail" data-level={level} data-id={id}>
      <header className="space-y-2">
        <p className="flex items-center gap-1.5 text-xs font-medium tracking-wide text-muted-foreground uppercase">
          <LevelIcon level={level} roomType={roomType} className="size-4" />
          {t(`level.${level}`)}
        </p>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h2 className="text-xl font-semibold break-all" data-testid="facility-detail-name">
            {e.name}
          </h2>
          {level === "site" && <span className="font-mono text-sm text-muted-foreground">{(e as Site).code}</span>}
          {level === "site" && <SiteStatusBadge status={(e as Site).status} />}
          {level === "rack" && <RackStatusBadge status={(e as Rack).status} />}
          {archived && <ArchivedBadge />}
        </div>
        <Location level={level} entity={e} />
      </header>

      {(canManage || canAddChild) && (
        <div className="flex flex-wrap gap-2">
          {canManage && (
            <Button variant="outline" size="sm" onClick={() => setDialog({ level, mode: "edit", entity: e, siteId })} data-testid="facility-edit">
              <Pencil aria-hidden />
              {t("detail.edit")}
            </Button>
          )}
          {canManage && level === "rack" && (
            <Button variant="outline" size="sm" onClick={() => setDialog({ level, mode: "rename", entity: e })} data-testid="facility-rename">
              <TextCursorInput aria-hidden />
              {t("detail.rename")}
            </Button>
          )}
          {canAddChild && child && (
            <Button
              size="sm"
              onClick={() => setDialog({ level: child, mode: "create", parent: { field: parentField[level], id: e.id } })}
              data-testid="facility-add-child"
            >
              <Plus aria-hidden />
              {t(`add.${child}`)}
            </Button>
          )}
          {canManage && (level !== "site" || globalPerms.has(PERM_FACILITY_MANAGE)) && (
            <Button variant="outline" size="sm" className="sm:ml-auto" onClick={() => setArchiving(true)} data-testid="facility-archive">
              <Archive aria-hidden />
              {t("detail.archive")}
            </Button>
          )}
        </div>
      )}

      <Fields level={level} entity={e} />

      {child && node && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium">{t("detail.children")}</h3>
          {node.children.length === 0 ? (
            <p className="text-sm text-muted-foreground">{t("detail.noChildren")}</p>
          ) : (
            <ul className="divide-y rounded-lg border" data-testid="facility-children">
              {node.children.map((c) => {
                return (
                  <li key={c.id}>
                    <button
                      type="button"
                      className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-accent"
                      onClick={() => onSelect(`${c.level}:${c.id}`)}
                    >
                      <LevelIcon level={c.level} roomType={c.roomType} className="size-4 shrink-0 text-muted-foreground" />
                      <span className="min-w-0 flex-1 truncate">{c.name}</span>
                      {c.rackStatus && <RackStatusBadge status={c.rackStatus} />}
                      {c.level !== "rack" && (c.rackCount ?? 0) > 0 && (
                        <span className="text-xs text-muted-foreground tabular-nums">{t("tree.racks", { count: c.rackCount ?? 0 })}</span>
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}

      {dialog && (
        <EntityDialog
          target={dialog}
          onClose={() => setDialog(null)}
          onSaved={(saved) => {
            setDialog(null);
            if (dialog.mode === "create") onSelect(`${dialog.level}:${saved.id}`);
          }}
        />
      )}
      {archiving && <ArchiveDialog level={level} entity={e} onClose={() => setArchiving(false)} />}
    </section>
  );
}

function Location({ level, entity }: { level: Level; entity: Entity }) {
  const { t } = useTranslation("facility");
  const parts: string[] = [];
  if (level === "building") parts.push((entity as Building).site_name);
  if (level === "floor") parts.push((entity as Floor).site_code, (entity as Floor).building_name);
  if (level === "room") parts.push((entity as Room).site_code, (entity as Room).building_name, (entity as Room).floor_name);
  if (level === "rack") {
    const k = entity as Rack;
    parts.push(k.site_code, k.building_name, k.floor_name, k.room_name);
  }
  if (parts.length === 0) return null;
  return (
    <p className="text-sm text-muted-foreground">
      <span className="sr-only">{t("detail.location")}: </span>
      {parts.join(" › ")}
    </p>
  );
}

function Fields({ level, entity }: { level: Level; entity: Entity }) {
  const { t } = useTranslation("facility");
  const fmt = useFormatters();
  const rows: [string, ReactNode][] = [];
  const dash = "—";
  switch (level) {
    case "site": {
      const s = entity as Site;
      rows.push([t("fields.address"), s.address || dash]);
      break;
    }
    case "building": {
      const b = entity as Building;
      rows.push([t("fields.code"), b.code || dash], [t("fields.notes"), b.notes || dash]);
      break;
    }
    case "floor": {
      const f = entity as Floor;
      rows.push([t("fields.level"), fmt.number(f.level)], [t("fields.notes"), f.notes || dash]);
      break;
    }
    case "room": {
      const r = entity as Room;
      rows.push([t("fields.room_type"), t(`roomType.${r.room_type}`)], [t("fields.notes"), r.notes || dash]);
      break;
    }
    case "rack": {
      const k = entity as Rack;
      const opt = (v: number | null, unit = "") => (v === null ? dash : `${fmt.number(v)}${unit}`);
      rows.push(
        [t("fields.asset_tag"), k.asset_tag ?? dash],
        [t("fields.row_label"), k.row_label ? `${k.row_label}${k.row_position ? ` · ${fmt.number(k.row_position)}` : ""}` : dash],
        [t("fields.units"), `${k.u_height}U · U${k.starting_unit}${k.desc_units ? " ↓" : " ↑"}`],
        [t("fields.width_in"), k.width_in === 23 ? t("form.width23") : t("form.width19")],
        [t("fields.depth_mm"), opt(k.depth_mm)],
        [t("fields.max_weight_kg"), opt(k.max_weight_kg)],
        [t("fields.max_power_kw"), opt(k.max_power_kw)],
        [t("fields.notes"), k.notes || dash],
      );
      break;
    }
  }
  rows.push([t("detail.updated"), `${fmt.dateTime(entity.updated_at)} · ${t("detail.version", { version: entity.version })}`]);
  return (
    <dl className="grid gap-x-6 gap-y-3 rounded-lg border p-4 text-sm sm:grid-cols-2" data-testid="facility-fields">
      {rows.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="text-xs text-muted-foreground">{k}</dt>
          <dd className="break-words">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

function ArchiveDialog({ level, entity, onClose }: { level: Level; entity: Entity; onClose: () => void }) {
  const { t } = useTranslation(["facility", "errors"]);
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);
  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      await archiveEntity(level, entity.id, entity.version);
      void qc.invalidateQueries({ queryKey: facilityKey });
      toast.success(t("form.archived", { name: entity.name }));
      onClose();
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "unknown";
      if (code === "version_conflict") setConflict(true);
      else setError(t(`errors:code.${toErrorCode(code)}`));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent data-testid="facility-archive-dialog">
        <DialogHeader>
          <DialogTitle>{t("archive.title", { name: entity.name })}</DialogTitle>
          <DialogDescription>{t("archive.description")}</DialogDescription>
        </DialogHeader>
        {conflict ? (
          <ConflictNotice
            onReload={() => {
              void qc.invalidateQueries({ queryKey: facilityKey });
              onClose();
            }}
          />
        ) : (
          <>
            {error && (
              <p role="alert" className="text-sm text-destructive" data-testid="facility-archive-error">
                {error}
              </p>
            )}
            <DialogFooter>
              <Button variant="outline" onClick={onClose}>
                {t("form.cancel")}
              </Button>
              <Button variant="destructive" disabled={busy} onClick={() => void confirm()} data-testid="facility-archive-confirm">
                {t("archive.confirm")}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
