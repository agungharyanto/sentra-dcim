import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Navigate, NavLink, Route, Routes, useSearchParams } from "react-router";

import { NotFoundPage } from "@/app/pages/StatusPages";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { FormMessage } from "@/features/auth/FormMessage";
import { useGlobalPermissions, usePermissions } from "@/features/auth/permissions";
import { toErrorCode } from "@/i18n/errors";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/utils";

import { fetchTree, LEVELS, type Level, PERM_FACILITY_MANAGE, PERM_RACK_MANAGE, treeKey } from "./api";
import { DetailPanel } from "./DetailPanel";
import { EntityDialog } from "./EntityDialog";
import { FacilityTree, findNode, toNodes } from "./FacilityTree";
import { RackListPage } from "./RackListPage";

const sections = [
  { path: "hierarchy", label: "tabs.hierarchy" },
  { path: "racks", label: "tabs.racks" },
] as const;

/** Facility (/facility, permission facility.read; v0.4.1): hierarchy tree + details, and the rack list. */
export function FacilityModule() {
  const { t } = useTranslation("facility");
  return (
    <div className="space-y-4">
      <nav aria-label={t("title")} className="flex gap-1 border-b">
        {sections.map((s) => (
          <NavLink
            key={s.path}
            to={`/facility/${s.path}`}
            className={({ isActive }) =>
              cn(
                "-mb-px border-b-2 px-3 py-2 text-sm",
                isActive ? "border-primary font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
              )
            }
          >
            {t(s.label)}
          </NavLink>
        ))}
      </nav>
      <Routes>
        <Route index element={<Navigate to="hierarchy" replace />} />
        <Route path="hierarchy" element={<HierarchyPage />} />
        <Route path="racks" element={<RackListPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </div>
  );
}

function parseSel(sel: string | null): { level: Level; id: string } | null {
  if (!sel) return null;
  const [level, id] = sel.split(":");
  if (!level || !id || !(LEVELS as readonly string[]).includes(level)) return null;
  return { level: level as Level, id };
}

/** Tree on the left, details of the selection (?sel=<level>:<id>) on the right; stacked on small screens. */
function HierarchyPage() {
  const { t } = useTranslation(["facility", "errors", "common"]);
  const perms = usePermissions();
  const globalPerms = useGlobalPermissions();
  const [params, setParams] = useSearchParams();
  const [addingSite, setAddingSite] = useState(false);
  const tree = useQuery({ queryKey: treeKey, queryFn: ({ signal }) => fetchTree(signal) });
  const nodes = useMemo(() => toNodes(tree.data?.sites ?? []), [tree.data]);
  const selKey = params.get("sel") ?? "";
  const sel = parseSel(selKey);
  const select = (key: string) => setParams({ sel: key });
  const readOnly = !perms.has(PERM_FACILITY_MANAGE) && !perms.has(PERM_RACK_MANAGE);
  const canAddSite = globalPerms.has(PERM_FACILITY_MANAGE);

  return (
    <div className="space-y-4" data-testid="facility-hierarchy">
      {readOnly && <FormMessage tone="info">{t("readOnly")}</FormMessage>}
      <div className="grid gap-4 lg:grid-cols-[minmax(16rem,22rem)_1fr]">
        <aside className="space-y-3 rounded-xl border bg-card p-3 lg:sticky lg:top-4 lg:max-h-[calc(100dvh-8rem)] lg:self-start lg:overflow-y-auto">
          {canAddSite && (
            <Button size="sm" className="w-full" onClick={() => setAddingSite(true)} data-testid="facility-add-site">
              <Plus aria-hidden />
              {t("add.site")}
            </Button>
          )}
          {tree.isPending && <Skeleton className="h-48 w-full" />}
          {tree.isError && (
            <div role="alert" className="space-y-2 text-sm text-muted-foreground">
              <p>{t(`errors:code.${toErrorCode(tree.error instanceof ApiError ? tree.error.code : "unknown")}`)}</p>
              <Button variant="outline" size="sm" onClick={() => void tree.refetch()}>
                {t("common:app.retry")}
              </Button>
            </div>
          )}
          {tree.data && nodes.length === 0 && (
            <div className="space-y-1 p-2 text-sm" data-testid="facility-tree-empty">
              <p className="font-medium">{t("tree.empty")}</p>
              {canAddSite && <p className="text-muted-foreground">{t("tree.emptyHint")}</p>}
            </div>
          )}
          {nodes.length > 0 && <FacilityTree nodes={nodes} selected={selKey} onSelect={select} />}
        </aside>
        <div className="min-w-0 rounded-xl border bg-card p-4">
          {sel ? (
            <DetailPanel key={selKey} level={sel.level} id={sel.id} node={findNode(nodes, selKey)} onSelect={select} />
          ) : (
            <p className="text-sm text-muted-foreground" data-testid="facility-select-hint">
              {t("detail.select")}
            </p>
          )}
        </div>
      </div>
      {addingSite && (
        <EntityDialog
          target={{ level: "site", mode: "create" }}
          onClose={() => setAddingSite(false)}
          onSaved={(s) => {
            setAddingSite(false);
            select(`site:${s.id}`);
          }}
        />
      )}
    </div>
  );
}
