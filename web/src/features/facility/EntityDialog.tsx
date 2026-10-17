import { useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import { type FormEvent, type ReactNode, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { FormMessage } from "@/features/auth/FormMessage";
import { toErrorCode } from "@/i18n/errors";
import { ApiError, getJson } from "@/lib/api";

import {
  createEntity,
  type Entity,
  facilityKey,
  type Level,
  type Page,
  RACK_NAME_RE,
  RACK_STATUSES,
  type Room,
  ROOM_TYPES,
  SITE_STATUSES,
  updateEntity,
} from "./api";

export const selectClass =
  "h-9 w-full rounded-md border border-input bg-background px-3 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm";

type Kind = "text" | "textarea" | "int" | "decimal" | "select" | "checkbox" | "room";
interface FieldSpec {
  name: string;
  kind: Kind;
  required?: boolean;
  optional?: boolean;
  options?: readonly (string | number)[];
  max?: number;
  hint?: string;
  createOnly?: boolean;
  half?: boolean;
}

/** Fields per level (API names; labels: facility:fields.<name>). */
const specs: Record<Level, FieldSpec[]> = {
  site: [
    { name: "code", kind: "text", required: true, max: 16, hint: "siteCode", createOnly: true, half: true },
    { name: "status", kind: "select", options: SITE_STATUSES, half: true },
    { name: "name", kind: "text", required: true, max: 100 },
    { name: "address", kind: "textarea", max: 500, optional: true },
  ],
  building: [
    { name: "name", kind: "text", required: true, max: 100 },
    { name: "code", kind: "text", max: 16, hint: "buildingCode", optional: true },
    { name: "notes", kind: "textarea", max: 2000, optional: true },
  ],
  floor: [
    { name: "name", kind: "text", required: true, max: 100, half: true },
    { name: "level", kind: "int", hint: "level", half: true },
    { name: "notes", kind: "textarea", max: 2000, optional: true },
  ],
  room: [
    { name: "name", kind: "text", required: true, max: 100, half: true },
    { name: "room_type", kind: "select", options: ROOM_TYPES, half: true },
    { name: "notes", kind: "textarea", max: 2000, optional: true },
  ],
  rack: [
    { name: "name", kind: "text", required: true, max: 64, hint: "rackName" },
    { name: "status", kind: "select", options: RACK_STATUSES, half: true },
    { name: "asset_tag", kind: "text", max: 64, optional: true, half: true },
    { name: "room_id", kind: "room", hint: "moveRack" },
    { name: "row_label", kind: "text", max: 16, optional: true, half: true },
    { name: "row_position", kind: "int", optional: true, half: true },
    { name: "u_height", kind: "int", half: true },
    { name: "starting_unit", kind: "int", half: true },
    { name: "width_in", kind: "select", options: [19, 23], half: true },
    { name: "depth_mm", kind: "int", optional: true, half: true },
    { name: "max_weight_kg", kind: "decimal", optional: true, half: true },
    { name: "max_power_kw", kind: "decimal", optional: true, half: true },
    { name: "desc_units", kind: "checkbox" },
    { name: "notes", kind: "textarea", max: 2000, optional: true },
  ],
};

const defaults: Record<Level, Record<string, string | boolean>> = {
  site: { code: "", status: "active", name: "", address: "" },
  building: { name: "", code: "", notes: "" },
  floor: { name: "", level: "0", notes: "" },
  room: { name: "", room_type: "whitespace", notes: "" },
  rack: {
    name: "",
    status: "planned",
    asset_tag: "",
    room_id: "",
    row_label: "",
    row_position: "",
    u_height: "42",
    starting_unit: "1",
    width_in: "19",
    depth_mm: "",
    max_weight_kg: "",
    max_power_kw: "",
    desc_units: false,
    notes: "",
  },
};

type Values = Record<string, string | boolean>;

function valuesOf(level: Level, e: Entity | undefined): Values {
  const out: Values = { ...defaults[level] };
  if (!e) return out;
  const rec = e as unknown as Record<string, unknown>;
  for (const k of Object.keys(out)) {
    const v = rec[k];
    out[k] = typeof v === "boolean" ? v : v === null || v === undefined ? "" : String(v);
  }
  return out;
}

/** Converts form values to the API body: numbers, null for cleared optional fields (edit) or omitted (create). */
function toBody(level: Level, values: Values, initial: Values | undefined, rename: boolean): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const f of specs[level]) {
    if (rename && f.name !== "name") continue;
    if (initial && f.createOnly) continue;
    const v = values[f.name];
    if (initial && v === initial[f.name]) continue;
    if (f.kind === "checkbox") body[f.name] = v === true;
    else if (f.kind === "int" || f.kind === "decimal" || (f.kind === "select" && typeof f.options?.[0] === "number")) {
      const s = String(v).trim();
      if (s === "") {
        if (initial) body[f.name] = null;
      } else body[f.name] = Number(s);
    } else if (f.kind === "room") {
      if (v) body[f.name] = v;
    } else if (f.name === "asset_tag") {
      const s = String(v).trim();
      if (s !== "" || initial) body[f.name] = s === "" ? null : s;
    } else body[f.name] = v;
  }
  return body;
}

export interface DialogTarget {
  level: Level;
  mode: "create" | "edit" | "rename";
  /** Edited object (edit/rename). */
  entity?: Entity;
  /** Parent for create: site_id / building_id / floor_id / room_id. */
  parent?: { field: string; id: string };
  /** Site of the rack (edit): the room picker lists rooms of this site only. */
  siteId?: string;
}

/** Create, edit or rename (racks) one object. A 412 offers a reload instead of overwriting someone else's change. */
export function EntityDialog({
  target,
  onClose,
  onSaved,
}: {
  target: DialogTarget;
  onClose: () => void;
  onSaved: (e: Entity) => void;
}) {
  const { t } = useTranslation(["facility", "errors"]);
  const id = useId();
  const qc = useQueryClient();
  const { level, mode, entity } = target;
  const initial = entity ? valuesOf(level, entity) : undefined;
  const [values, setValues] = useState<Values>(() => initial ?? valuesOf(level, undefined));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ text: string; field?: string } | null>(null);
  const [conflict, setConflict] = useState(false);
  const fields = specs[level].filter((f) => (mode === "rename" ? f.name === "name" : !(mode === "create" && f.kind === "room")));
  const rooms = useQuery({
    queryKey: [...facilityKey, "rooms-of", target.siteId],
    queryFn: ({ signal }) => getJson<Page<Room>>(`/api/v1/rooms?site_id=${encodeURIComponent(target.siteId ?? "")}&limit=200`, signal),
    enabled: level === "rack" && mode === "edit" && !!target.siteId,
  });

  const title =
    mode === "create" ? t(`add.${level}`) : mode === "rename" ? t("detail.rename") : `${t("detail.edit")} · ${entity ? entityName(entity) : ""}`;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    const name = String(values.name ?? "").trim();
    if (!name || (level === "site" && mode === "create" && !String(values.code).trim())) {
      setError({ text: t("errors:code.validation_failed"), field: name ? "code" : "name" });
      return;
    }
    if (level === "rack" && !RACK_NAME_RE.test(name)) {
      setError({ text: t("hint.rackName"), field: "name" });
      return;
    }
    const body = toBody(level, values, initial, mode === "rename");
    if (mode !== "create" && Object.keys(body).length === 0) {
      onClose();
      return;
    }
    setBusy(true);
    try {
      const saved =
        mode === "create" || !entity
          ? await createEntity(level, { ...body, ...(target.parent ? { [target.parent.field]: target.parent.id } : {}) })
          : await updateEntity(level, entity.id, entity.version, body);
      void qc.invalidateQueries({ queryKey: facilityKey });
      toast.success(
        mode === "create"
          ? t("form.created", { name: entityName(saved) })
          : mode === "rename"
            ? t("form.renamed", { name: entityName(saved) })
            : t("form.saved"),
      );
      onSaved(saved);
    } catch (err) {
      const ae = err instanceof ApiError ? err : new ApiError("unknown", 0);
      if (ae.code === "version_conflict") setConflict(true);
      else
        setError({
          text: t(`errors:code.${toErrorCode(ae.code)}`),
          ...(ae.field ? { field: ae.field } : {}),
        });
    } finally {
      setBusy(false);
    }
  };

  const reload = () => {
    void qc.invalidateQueries({ queryKey: facilityKey });
    onClose();
  };

  const set = (k: string, v: string | boolean) => setValues((s) => ({ ...s, [k]: v }));

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl" data-testid="facility-dialog" data-level={level} data-mode={mode}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {level === "site" && mode === "create" && <DialogDescription>{t("hint.siteGlobalOnly")}</DialogDescription>}
        </DialogHeader>
        {conflict ? (
          <ConflictNotice onReload={reload} />
        ) : (
          <form onSubmit={(e) => void submit(e)} noValidate className="grid gap-4 sm:grid-cols-2">
            {error && (
              <div className="sm:col-span-2">
                <FormMessage tone="error" testId="facility-form-error">
                  {error.text}
                </FormMessage>
              </div>
            )}
            {fields.map((f) => {
              const fid = `${id}-${f.name}`;
              const invalid = error?.field === f.name ? true : undefined;
              const label = t(`fields.${f.name as "name"}`);
              const hint = f.hint ? t(`hint.${f.hint as "rackName"}`) : f.optional ? t("hint.optional") : undefined;
              const v = values[f.name];
              let control: ReactNode;
              switch (f.kind) {
                case "textarea":
                  control = (
                    <textarea
                      id={fid}
                      className="min-h-20 w-full rounded-md border border-input bg-background px-3 py-2 text-base shadow-xs outline-none focus-visible:ring-[3px] focus-visible:ring-ring md:text-sm"
                      maxLength={f.max}
                      value={String(v)}
                      onChange={(e) => set(f.name, e.target.value)}
                      aria-invalid={invalid}
                    />
                  );
                  break;
                case "select":
                  control = (
                    <select id={fid} className={selectClass} value={String(v)} onChange={(e) => set(f.name, e.target.value)} aria-invalid={invalid}>
                      {(f.options ?? []).map((o) => (
                        <option key={o} value={String(o)}>
                          {optionLabel(t, level, f.name, o)}
                        </option>
                      ))}
                    </select>
                  );
                  break;
                case "room":
                  control = (
                    <select id={fid} className={selectClass} value={String(v)} onChange={(e) => set(f.name, e.target.value)} aria-invalid={invalid}>
                      {(rooms.data?.items ?? []).map((r) => (
                        <option key={r.id} value={r.id}>
                          {`${r.building_name} › ${r.floor_name} › ${r.name}`}
                        </option>
                      ))}
                    </select>
                  );
                  break;
                case "checkbox":
                  return (
                    <label key={f.name} className="flex items-center gap-2 text-sm sm:col-span-2">
                      <input type="checkbox" className="size-4" checked={v === true} onChange={(e) => set(f.name, e.target.checked)} />
                      {label}
                    </label>
                  );
                default:
                  control = (
                    <Input
                      id={fid}
                      inputMode={f.kind === "int" ? "numeric" : f.kind === "decimal" ? "decimal" : undefined}
                      autoComplete="off"
                      spellCheck={false}
                      maxLength={f.max}
                      value={String(v)}
                      onChange={(e) => set(f.name, e.target.value)}
                      aria-invalid={invalid}
                      aria-describedby={hint ? `${fid}-hint` : undefined}
                      disabled={mode !== "create" && f.createOnly}
                      autoFocus={f.name === "name" && mode !== "create"}
                    />
                  );
              }
              return (
                <div key={f.name} className={f.half && mode !== "rename" ? "space-y-1.5" : "space-y-1.5 sm:col-span-2"}>
                  <label htmlFor={fid} className="text-sm font-medium">
                    {label}
                  </label>
                  {control}
                  {(hint || (mode !== "create" && f.createOnly)) && (
                    <p id={`${fid}-hint`} className="text-xs text-muted-foreground">
                      {mode !== "create" && f.createOnly ? t("hint.codeFixed") : hint}
                    </p>
                  )}
                </div>
              );
            })}
            <div className="flex flex-col-reverse gap-2 sm:col-span-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="outline" onClick={onClose}>
                {t("form.cancel")}
              </Button>
              <Button type="submit" disabled={busy} data-testid="facility-save">
                {busy ? t("form.saving") : mode === "create" ? t("form.create") : t("form.save")}
              </Button>
            </div>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

function optionLabel(
  t: ReturnType<typeof useTranslation<["facility", "errors"]>>["t"],
  level: Level,
  field: string,
  o: string | number,
): string {
  switch (field) {
    case "status":
      return level === "site" ? t(`siteStatus.${o as "inactive"}`) : t(`rackStatus.${o as "planned"}`);
    case "room_type":
      return t(`roomType.${o as "mmr"}`);
    case "width_in":
      return o === 23 ? t("form.width23") : t("form.width19");
    default:
      return String(o);
  }
}

export function entityName(e: Entity): string {
  return e.name;
}

/** 412 version_conflict: someone else saved first. Reload instead of overwriting. */
export function ConflictNotice({ onReload }: { onReload: () => void }) {
  const { t } = useTranslation("facility");
  return (
    <div role="alert" className="space-y-3 rounded-lg border border-status-warning-border bg-status-warning-bg p-4 text-status-warning-fg" data-testid="facility-conflict">
      <p className="font-medium">{t("conflict.title")}</p>
      <p className="text-sm">{t("conflict.body")}</p>
      <Button variant="outline" size="sm" onClick={onReload} data-testid="facility-reload">
        <RefreshCw aria-hidden />
        {t("conflict.reload")}
      </Button>
    </div>
  );
}
