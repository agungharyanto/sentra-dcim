import { ApiError, deleteJson, getJson, postJson } from "@/lib/api";

/** Mirrors api/openapi.yaml#/components/schemas/AuthUser. */
export interface AuthUser {
  id: string;
  username: string;
  display_name: string;
  must_change_password: boolean;
  /** Role keys (label: users:roles.<key>) and the union of their permissions (RBAC, v0.3.2). */
  roles: string[];
  permissions: string[];
}

/** Mirrors api/openapi.yaml#/components/schemas/SessionInfo. Times are UTC (RFC 3339). */
export interface SessionInfo {
  id: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  idle_expires_at: string;
  ip: string | null;
  user_agent: string;
  current: boolean;
}

/** Mirrors api/openapi.yaml#/components/schemas/AuthMe. */
export interface AuthMe {
  user: AuthUser;
  session: SessionInfo;
}

export const meQueryKey = ["auth", "me"] as const;
export const sessionsQueryKey = ["auth", "sessions"] as const;

/** Policy shared with the backend (internal/auth/password). */
export const PASSWORD_MIN = 12;
export const PASSWORD_MAX = 128;

/** The signed-in user, or null without a valid session. */
export async function fetchMe(signal?: AbortSignal): Promise<AuthMe | null> {
  try {
    return await getJson<AuthMe>("/api/v1/auth/me", signal);
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
}

export function login(username: string, password: string) {
  return postJson<AuthMe>("/api/v1/auth/login", { username, password });
}

export function logout() {
  return postJson("/api/v1/auth/logout");
}

export function changePassword(currentPassword: string, newPassword: string) {
  return postJson("/api/v1/auth/password", { current_password: currentPassword, new_password: newPassword });
}

export function fetchSessions(signal?: AbortSignal) {
  return getJson<{ sessions: SessionInfo[] }>("/api/v1/auth/sessions", signal);
}

export function revokeSession(id: string) {
  return deleteJson(`/api/v1/auth/sessions/${encodeURIComponent(id)}`);
}
