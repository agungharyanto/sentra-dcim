import { getJson, postJson } from "@/lib/api";

export type AuditResult = "success" | "failure" | "denied";

/** Mirrors api/openapi.yaml#/components/schemas/AuditEvent. Times are UTC (RFC 3339). */
export interface AuditEvent {
  seq: number;
  occurred_at: string;
  actor_type: "user" | "system" | "anonymous";
  actor_id: string | null;
  actor_username: string;
  action: string;
  target_type: string;
  target_id: string;
  result: AuditResult;
  ip: string;
  user_agent: string;
  request_id: string;
  metadata: Record<string, unknown>;
  hash: string;
}

export interface AuditEventList {
  events: AuditEvent[];
  next_before: number | null;
  limit: number;
}

/** Mirrors api/openapi.yaml#/components/schemas/AuditVerifyResult. */
export interface AuditVerifyResult {
  ok: boolean;
  checked: number;
  from_seq: number;
  head_seq: number;
  head_hash: string;
  checkpoint_seq: number;
  anchors_checked: number;
  broken_seq: number;
  reason?: string;
  verified_at: string;
}

export interface AuditIntegrity {
  last: AuditVerifyResult | null;
  running: boolean;
}

/** Filter as sent to the API: since/until are UTC RFC 3339 ("" = open). */
export interface AuditFilter {
  since: string;
  until: string;
  actor: string;
  action: string;
  result: "" | AuditResult;
  target: string;
}

export const RESULTS: readonly AuditResult[] = ["success", "failure", "denied"];

/** Date-time format of the detail view: weekday, seconds and the zone name. */
export const DETAIL_TIME = { weekday: true, seconds: true, zone: true } as const;

export const emptyFilter: AuditFilter = { since: "", until: "", actor: "", action: "", result: "", target: "" };

export const PAGE_SIZE = 50;

/** Permissions used by this module (internal/platform/rbac). */
export const PERM_AUDIT_READ = "audit.read";
export const PERM_AUDIT_EXPORT = "audit.export";

export const auditKey = ["audit"] as const;

export function filterParams(f: AuditFilter): URLSearchParams {
  const p = new URLSearchParams();
  if (f.since) p.set("since", f.since);
  if (f.until) p.set("until", f.until);
  if (f.actor.trim()) p.set("actor", f.actor.trim());
  if (f.action) p.set("action", f.action);
  if (f.result) p.set("result", f.result);
  if (f.target.trim()) p.set("target_id", f.target.trim());
  return p;
}

export function fetchEvents(f: AuditFilter, before: number | undefined, signal?: AbortSignal) {
  const p = filterParams(f);
  p.set("limit", String(PAGE_SIZE));
  if (before !== undefined) p.set("before", String(before));
  return getJson<AuditEventList>(`/api/v1/audit/events?${p.toString()}`, signal);
}

export function fetchActions(signal?: AbortSignal) {
  return getJson<{ actions: string[] }>("/api/v1/audit/actions", signal);
}

export function fetchIntegrity(signal?: AbortSignal) {
  return getJson<AuditIntegrity>("/api/v1/audit/integrity", signal);
}

export function verifyNow() {
  return postJson<AuditVerifyResult>("/api/v1/audit/integrity/verify");
}

/** The CSV is downloaded by the browser (Content-Disposition: attachment) with the session cookie. */
export function exportUrl(f: AuditFilter): string {
  const q = filterParams(f).toString();
  return `/api/v1/audit/events/export.csv${q ? `?${q}` : ""}`;
}

/** Dotted prefixes ("auth", "auth.login", …) of the known actions, for "all actions of a kind" filters. */
export function actionPrefixes(actions: string[]): string[] {
  const out = new Set<string>();
  for (const a of actions) {
    const parts = a.split(".");
    for (let i = 1; i < parts.length; i++) out.add(parts.slice(0, i).join("."));
  }
  return [...out].sort();
}
