import { getJson, patchJsonIfMatch, postJson, postJsonIfMatch } from "@/lib/api";

/** Permissions of this module (internal/platform/rbac, v0.4.1). */
export const PERM_FACILITY_READ = "facility.read";
export const PERM_FACILITY_MANAGE = "facility.manage";
export const PERM_RACK_READ = "rack.read";
export const PERM_RACK_MANAGE = "rack.manage";

export const SITE_STATUSES = ["planned", "active", "inactive"] as const;
export const ROOM_TYPES = ["whitespace", "mmr", "power", "cooling", "noc", "storage", "staging", "other"] as const;
export const RACK_STATUSES = ["planned", "active", "reserved", "decommissioning", "retired"] as const;
export type SiteStatus = (typeof SITE_STATUSES)[number];
export type RoomType = (typeof ROOM_TYPES)[number];
export type RackStatus = (typeof RACK_STATUSES)[number];

/** Levels of the hierarchy; the URL selection is "<level>:<id>". */
export const LEVELS = ["site", "building", "floor", "room", "rack"] as const;
export type Level = (typeof LEVELS)[number];

interface Versioned {
  id: string;
  version: number;
  created_at: string;
  updated_at: string;
  archived_at: string | null;
}

/** Mirrors api/openapi.yaml#/components/schemas/Site (and the other levels below). Times are UTC. */
export interface Site extends Versioned {
  code: string;
  name: string;
  address: string;
  status: SiteStatus;
}
export interface Building extends Versioned {
  site_id: string;
  site_code: string;
  site_name: string;
  code: string;
  name: string;
  notes: string;
}
export interface Floor extends Versioned {
  building_id: string;
  building_name: string;
  site_id: string;
  site_code: string;
  name: string;
  level: number;
  notes: string;
}
export interface Room extends Versioned {
  floor_id: string;
  floor_name: string;
  building_id: string;
  building_name: string;
  site_id: string;
  site_code: string;
  name: string;
  room_type: RoomType;
  notes: string;
  rack_count: number;
}
export interface Rack extends Versioned {
  room_id: string;
  room_name: string;
  room_type: RoomType;
  floor_id: string;
  floor_name: string;
  building_id: string;
  building_name: string;
  site_id: string;
  site_code: string;
  site_name: string;
  name: string;
  asset_tag: string | null;
  row_label: string;
  row_position: number | null;
  u_height: number;
  starting_unit: number;
  desc_units: boolean;
  width_in: 19 | 23;
  depth_mm: number | null;
  max_weight_kg: number | null;
  max_power_kw: number | null;
  status: RackStatus;
  notes: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

/** GET /facility/tree. */
export interface TreeRack {
  id: string;
  name: string;
  status: RackStatus;
}
export interface TreeRoom {
  id: string;
  name: string;
  room_type: RoomType;
  rack_count: number;
  racks_by_status: Partial<Record<RackStatus, number>>;
  racks: TreeRack[];
}
export interface TreeFloor {
  id: string;
  name: string;
  level: number;
  rack_count: number;
  rooms: TreeRoom[];
}
export interface TreeBuilding {
  id: string;
  code: string;
  name: string;
  rack_count: number;
  floors: TreeFloor[];
}
export interface TreeSite {
  id: string;
  code: string;
  name: string;
  status: SiteStatus;
  rack_count: number;
  buildings: TreeBuilding[];
}

export const facilityKey = ["facility"] as const;
export const treeKey = [...facilityKey, "tree"] as const;
export const PAGE_SIZE = 50;

const plural: Record<Level, string> = { site: "sites", building: "buildings", floor: "floors", room: "rooms", rack: "racks" };
const path = (level: Level, id?: string) => `/api/v1/${plural[level]}${id ? `/${encodeURIComponent(id)}` : ""}`;

export function fetchTree(signal?: AbortSignal) {
  return getJson<{ sites: TreeSite[] }>("/api/v1/facility/tree", signal);
}

export type Entity = Site | Building | Floor | Room | Rack;
export interface EntityOf {
  site: Site;
  building: Building;
  floor: Floor;
  room: Room;
  rack: Rack;
}

export function fetchEntity<L extends Level>(level: L, id: string, signal?: AbortSignal) {
  return getJson<EntityOf[L]>(path(level, id), signal);
}

export function createEntity<L extends Level>(level: L, body: Record<string, unknown>) {
  return postJson<EntityOf[L]>(path(level), body);
}

/** PATCH with If-Match: a stale version answers 412 version_conflict. */
export function updateEntity<L extends Level>(level: L, id: string, version: number, body: Record<string, unknown>) {
  return patchJsonIfMatch<EntityOf[L]>(path(level, id), body, version);
}

export function archiveEntity<L extends Level>(level: L, id: string, version: number) {
  return postJsonIfMatch<EntityOf[L]>(`${path(level, id)}/archive`, version);
}

export interface RackFilter {
  q: string;
  site_id: string;
  status: "" | RackStatus;
  include_archived: boolean;
  sort: string;
  offset: number;
}

export function fetchRacks(f: RackFilter, signal?: AbortSignal) {
  const p = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(f.offset) });
  if (f.q.trim()) p.set("q", f.q.trim());
  if (f.site_id) p.set("site_id", f.site_id);
  if (f.status) p.set("status", f.status);
  if (f.include_archived) p.set("include_archived", "true");
  if (f.sort) p.set("sort", f.sort);
  return getJson<Page<Rack>>(`/api/v1/racks?${p.toString()}`, signal);
}

export function fetchSites(signal?: AbortSignal) {
  return getJson<Page<Site>>("/api/v1/sites?limit=200", signal);
}

/** Rack names (D-083): 1–64 characters, letters, digits, space and . _ - /, starting with a letter or digit. */
export const RACK_NAME_RE = /^[A-Za-z0-9][A-Za-z0-9 ._/-]{0,63}$/;
