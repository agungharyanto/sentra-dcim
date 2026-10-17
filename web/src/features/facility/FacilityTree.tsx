import { ChevronRight } from "lucide-react";
import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";

import type { Level, RoomType, TreeSite } from "./api";
import { LevelIcon, rackStatusStyle } from "./badges";

/** A node of the facility tree, flattened from GET /facility/tree. */
export interface Node {
  level: Level;
  id: string;
  name: string;
  /** Site code, building code. */
  code?: string;
  roomType?: RoomType;
  rackCount?: number;
  rackStatus?: keyof typeof rackStatusStyle;
  children: Node[];
}

export function toNodes(sites: TreeSite[]): Node[] {
  return sites.map((s) => ({
    level: "site",
    id: s.id,
    name: s.name,
    code: s.code,
    rackCount: s.rack_count,
    children: s.buildings.map((b) => ({
      level: "building",
      id: b.id,
      name: b.name,
      code: b.code,
      rackCount: b.rack_count,
      children: b.floors.map((f) => ({
        level: "floor",
        id: f.id,
        name: f.name,
        rackCount: f.rack_count,
        children: f.rooms.map((r) => ({
          level: "room",
          id: r.id,
          name: r.name,
          roomType: r.room_type,
          rackCount: r.rack_count,
          children: r.racks.map((k) => ({ level: "rack", id: k.id, name: k.name, rackStatus: k.status, children: [] })),
        })),
      })),
    })),
  }));
}

/** Keys ("level:id") of the ancestors of the node key, if it is in the tree. */
export function ancestorsOf(nodes: Node[], key: string): string[] {
  for (const n of nodes) {
    const k = `${n.level}:${n.id}`;
    if (k === key) return [];
    const inner = ancestorsOf(n.children, key);
    if (inner.length > 0 || n.children.some((c) => `${c.level}:${c.id}` === key)) return [k, ...inner];
  }
  return [];
}

export function findNode(nodes: Node[], key: string): Node | undefined {
  for (const n of nodes) {
    if (`${n.level}:${n.id}` === key) return n;
    const inner = findNode(n.children, key);
    if (inner) return inner;
  }
  return undefined;
}

/**
 * Site › Building › Floor › Room › Rack as an ARIA tree (arrow keys move, Enter selects, Right/Left open/close).
 * The ancestors of the selected node are opened automatically.
 */
export function FacilityTree({ nodes, selected, onSelect }: { nodes: Node[]; selected: string; onSelect: (key: string) => void }) {
  const { t } = useTranslation("facility");
  // Explicit choices win over the automatic opening of the selection's ancestors.
  const [choice, setChoice] = useState<ReadonlyMap<string, boolean>>(
    () => new Map(nodes.length === 1 && nodes[0] ? [[`site:${nodes[0].id}`, true]] : []),
  );
  const auto = useMemo(() => new Set(ancestorsOf(nodes, selected)), [nodes, selected]);
  const isOpen = (k: string) => choice.get(k) ?? auto.has(k);
  const toggle = (k: string, to?: boolean) => {
    const want = to ?? !isOpen(k);
    setChoice((prev) => new Map(prev).set(k, want));
  };

  // Visible nodes in order, for keyboard navigation.
  const visible: { key: string; node: Node; depth: number; parent?: string }[] = [];
  const walk = (ns: Node[], depth: number, parent?: string) => {
    for (const n of ns) {
      const key = `${n.level}:${n.id}`;
      visible.push({ key, node: n, depth, ...(parent ? { parent } : {}) });
      if (n.children.length > 0 && isOpen(key)) walk(n.children, depth + 1, key);
    }
  };
  walk(nodes, 0);
  const focusKey = visible.some((v) => v.key === selected) ? selected : visible[0]?.key;

  const onKey = (e: React.KeyboardEvent, i: number) => {
    const v = visible[i];
    if (!v) return;
    const focus = (j: number) => {
      const target = visible[j];
      if (target) document.getElementById(`tree-${target.key}`)?.focus();
    };
    switch (e.key) {
      case "ArrowDown":
        focus(i + 1);
        break;
      case "ArrowUp":
        focus(i - 1);
        break;
      case "ArrowRight":
        if (v.node.children.length > 0 && !isOpen(v.key)) toggle(v.key, true);
        else focus(i + 1);
        break;
      case "ArrowLeft":
        if (v.node.children.length > 0 && isOpen(v.key)) toggle(v.key, false);
        else if (v.parent) focus(visible.findIndex((x) => x.key === v.parent));
        break;
      case "Enter":
      case " ":
        onSelect(v.key);
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  return (
    <ul role="tree" aria-label={t("tree.label")} className="space-y-0.5 text-sm" data-testid="facility-tree">
      {visible.map((v, i) => {
        const hasChildren = v.node.children.length > 0;
        const expanded = isOpen(v.key);
        const isSelected = v.key === selected;
        const status = v.node.rackStatus ? rackStatusStyle[v.node.rackStatus] : undefined;
        return (
          <li
            key={v.key}
            id={`tree-${v.key}`}
            role="treeitem"
            aria-level={v.depth + 1}
            aria-selected={isSelected}
            aria-expanded={hasChildren ? expanded : undefined}
            tabIndex={v.key === focusKey ? 0 : -1}
            onKeyDown={(e) => onKey(e, i)}
            onClick={() => onSelect(v.key)}
            data-node={v.key}
            className={cn(
              "flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-2 outline-none hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring",
              isSelected && "bg-accent font-medium text-accent-foreground",
            )}
            style={{ paddingLeft: `${v.depth * 1}rem` }}
          >
            {hasChildren ? (
              <button
                type="button"
                tabIndex={-1}
                className="grid size-5 shrink-0 place-items-center rounded hover:bg-muted"
                aria-label={expanded ? t("tree.collapse", { name: v.node.name }) : t("tree.expand", { name: v.node.name })}
                onClick={(e) => {
                  e.stopPropagation();
                  toggle(v.key);
                }}
              >
                <ChevronRight className={cn("size-4 transition-transform", expanded && "rotate-90")} aria-hidden />
              </button>
            ) : (
              <span className="size-5 shrink-0" aria-hidden />
            )}
            <LevelIcon
              level={v.node.level}
              roomType={v.node.roomType}
              className={cn("size-4 shrink-0 text-muted-foreground", v.node.roomType === "mmr" && "text-primary")}
            />
            {v.node.code && v.node.level === "site" && <span className="font-mono text-xs text-muted-foreground">{v.node.code}</span>}
            <span className="min-w-0 truncate">{v.node.name}</span>
            {v.node.roomType === "mmr" && <span className="sr-only">{t("tree.mmr")}</span>}
            {status && (
              <span
                className={cn("ml-auto size-2 shrink-0 rounded-full border", status.badge)}
                aria-hidden
                title={t(`rackStatus.${v.node.rackStatus ?? "planned"}`)}
              />
            )}
            {v.node.level !== "rack" && (v.node.rackCount ?? 0) > 0 && (
              <span className="ml-auto shrink-0 text-xs text-muted-foreground tabular-nums">
                {t("tree.racks", { count: v.node.rackCount ?? 0 })}
              </span>
            )}
          </li>
        );
      })}
    </ul>
  );
}
