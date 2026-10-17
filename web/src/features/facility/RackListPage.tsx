import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";

import { facilityKey, fetchRacks, fetchSites, PAGE_SIZE, RACK_STATUSES, type Rack, type RackFilter } from "./api";
import { ArchivedBadge, LevelIcon, RackStatusBadge } from "./badges";
import { selectClass } from "./EntityDialog";

const sorts = [
  { value: "", label: "list.sortDefault" },
  { value: "name", label: "list.sortName" },
  { value: "-name", label: "list.sortNameDesc" },
  { value: "status", label: "list.sortStatus" },
  { value: "-updated_at", label: "list.sortUpdated" },
] as const;

/** /facility/racks: filterable rack list (site, status, text, archived) with status badges; MMR rooms marked. */
export function RackListPage() {
  const { t } = useTranslation(["facility", "errors", "common"]);
  const id = useId();
  const [f, setF] = useState<RackFilter>({ q: "", site_id: "", status: "", include_archived: false, sort: "", offset: 0 });
  const set = <K extends keyof RackFilter>(k: K, v: RackFilter[K]) => setF((x) => ({ ...x, [k]: v, offset: k === "offset" ? (v as number) : 0 }));
  const sites = useQuery({ queryKey: [...facilityKey, "sites"], queryFn: ({ signal }) => fetchSites(signal) });
  const list = useQuery({
    queryKey: [...facilityKey, "racks", f],
    queryFn: ({ signal }) => fetchRacks(f, signal),
    placeholderData: keepPreviousData,
  });
  const filtered = f.q !== "" || f.site_id !== "" || f.status !== "" || f.include_archived;
  const data = list.data;
  const location = (k: Rack) => [k.site_code, k.building_name, k.floor_name, k.room_name].join(" › ");
  const roomIcon = (k: Rack) => (
    <LevelIcon level="room" roomType={k.room_type} className={k.room_type === "mmr" ? "size-4 shrink-0 text-primary" : "size-4 shrink-0 text-muted-foreground"} />
  );
  const link = (k: Rack) => (
    <Link to={`/facility/hierarchy?sel=rack:${k.id}`} className="font-medium underline-offset-4 hover:underline" data-testid="rack-link">
      {k.name}
    </Link>
  );

  return (
    <div className="space-y-4" data-testid="rack-list">
      <div className="grid gap-2 md:grid-cols-[1fr_auto_auto_auto] md:items-center">
        <Input
          type="search"
          aria-label={t("list.search")}
          placeholder={t("list.search")}
          value={f.q}
          onChange={(e) => set("q", e.target.value)}
          data-testid="rack-search"
        />
        <select id={`${id}-site`} aria-label={t("fields.site_id")} className={selectClass} value={f.site_id} onChange={(e) => set("site_id", e.target.value)}>
          <option value="">{t("list.allSites")}</option>
          {(sites.data?.items ?? []).map((s) => (
            <option key={s.id} value={s.id}>
              {`${s.code} · ${s.name}`}
            </option>
          ))}
        </select>
        <select
          id={`${id}-status`}
          aria-label={t("list.status")}
          className={selectClass}
          value={f.status}
          onChange={(e) => set("status", e.target.value as RackFilter["status"])}
          data-testid="rack-status-filter"
        >
          <option value="">{t("list.allStatuses")}</option>
          {RACK_STATUSES.map((s) => (
            <option key={s} value={s}>
              {t(`rackStatus.${s}`)}
            </option>
          ))}
        </select>
        <select id={`${id}-sort`} aria-label={t("list.sort")} className={selectClass} value={f.sort} onChange={(e) => set("sort", e.target.value)}>
          {sorts.map((s) => (
            <option key={s.value} value={s.value}>
              {t(s.label)}
            </option>
          ))}
        </select>
      </div>
      <label className="flex w-fit items-center gap-2 text-sm">
        <input type="checkbox" className="size-4" checked={f.include_archived} onChange={(e) => set("include_archived", e.target.checked)} />
        {t("list.includeArchived")}
      </label>

      {list.isPending && <Skeleton className="h-40 w-full" />}
      {list.isError && (
        <div role="alert" className="space-y-2">
          <p className="text-sm text-muted-foreground">
            {t(`errors:code.${toErrorCode(list.error instanceof ApiError ? list.error.code : "unknown")}`)}
          </p>
          <Button variant="outline" size="sm" onClick={() => void list.refetch()}>
            {t("common:app.retry")}
          </Button>
        </div>
      )}
      {data && data.items.length === 0 && (
        <p className="rounded-xl border border-dashed p-8 text-center text-muted-foreground" data-testid="racks-empty">
          {filtered ? t("list.emptyFiltered") : t("list.empty")}
        </p>
      )}
      {data && data.items.length > 0 && (
        <>
          <div className="hidden rounded-xl border md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("list.name")}</TableHead>
                  <TableHead>{t("list.location")}</TableHead>
                  <TableHead>{t("list.status")}</TableHead>
                  <TableHead className="text-right">{t("list.units")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((k) => (
                  <TableRow key={k.id} data-testid="rack-row" data-rack={k.name}>
                    <TableCell>
                      {link(k)}
                      {k.asset_tag && <span className="block font-mono text-xs text-muted-foreground">{k.asset_tag}</span>}
                    </TableCell>
                    <TableCell>
                      <span className="flex items-center gap-1.5">
                        {roomIcon(k)}
                        <span className="truncate">{location(k)}</span>
                      </span>
                    </TableCell>
                    <TableCell>
                      <span className="flex flex-wrap gap-1">
                        <RackStatusBadge status={k.status} />
                        {k.archived_at && <ArchivedBadge />}
                      </span>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">{k.u_height}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <ul className="space-y-3 md:hidden">
            {data.items.map((k) => (
              <li key={k.id} className="space-y-2 rounded-xl border bg-card p-4" data-testid="rack-card">
                <div className="flex items-start justify-between gap-2">
                  {link(k)}
                  <span className="text-sm text-muted-foreground tabular-nums">{`${k.u_height}U`}</span>
                </div>
                <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
                  {roomIcon(k)}
                  <span className="min-w-0 truncate">{location(k)}</span>
                </p>
                <span className="flex flex-wrap gap-1">
                  <RackStatusBadge status={k.status} />
                  {k.archived_at && <ArchivedBadge />}
                </span>
              </li>
            ))}
          </ul>
          <nav aria-label={t("list.pagination")} className="flex items-center justify-between gap-2 text-sm">
            <span className="text-muted-foreground" data-testid="racks-range">
              {t("list.range", { from: data.offset + 1, to: data.offset + data.items.length, total: data.total })}
            </span>
            <span className="flex gap-2">
              <Button variant="outline" size="sm" disabled={f.offset === 0} onClick={() => set("offset", Math.max(0, f.offset - PAGE_SIZE))}>
                {t("list.prev")}
              </Button>
              <Button variant="outline" size="sm" disabled={f.offset + PAGE_SIZE >= data.total} onClick={() => set("offset", f.offset + PAGE_SIZE)}>
                {t("list.next")}
              </Button>
            </span>
          </nav>
        </>
      )}
    </div>
  );
}
