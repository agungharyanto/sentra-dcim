import { getJson, patchJson, postJson, putJson } from "@/lib/api";

export type UserStatus = "active" | "disabled";

/** Mirrors api/openapi.yaml#/components/schemas/User. Times are UTC (RFC 3339). */
export interface User {
  id: string;
  username: string;
  display_name: string;
  email: string | null;
  status: UserStatus;
  roles: string[];
  must_change_password: boolean;
  locked: boolean;
  last_login_at: string | null;
  created_at: string;
  updated_at: string;
}

/** Mirrors api/openapi.yaml#/components/schemas/UserList. */
export interface UserList {
  users: User[];
  total: number;
  limit: number;
  offset: number;
}

/** Mirrors api/openapi.yaml#/components/schemas/Role. Label: users:roles.<key>. */
export interface Role {
  key: string;
  system: boolean;
  permissions: string[];
  user_count: number;
}

export interface UserFilter {
  q: string;
  status: "" | UserStatus;
  role: string;
  offset: number;
}

export const PAGE_SIZE = 50;

/** Permissions used by this module (internal/platform/rbac). */
export const PERM_USERS_READ = "users.read";
export const PERM_USERS_MANAGE = "users.manage";
export const PERM_ROLES_READ = "roles.read";

/** Display order of the system roles (internal/platform/rbac.SystemRoleKeys). */
export const SYSTEM_ROLES = ["admin", "noc", "security", "technician", "viewer", "auditor"] as const;

export const usersKey = ["users"] as const;
export const rolesKey = ["roles"] as const;

export function fetchUsers(f: UserFilter, signal?: AbortSignal) {
  const p = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(f.offset) });
  if (f.q.trim()) p.set("q", f.q.trim());
  if (f.status) p.set("status", f.status);
  if (f.role) p.set("role", f.role);
  return getJson<UserList>(`/api/v1/users?${p.toString()}`, signal);
}

export function fetchUser(id: string, signal?: AbortSignal) {
  return getJson<User>(`/api/v1/users/${encodeURIComponent(id)}`, signal);
}

export function fetchRoles(signal?: AbortSignal) {
  return getJson<{ roles: Role[]; permissions: string[] }>("/api/v1/roles", signal);
}

export interface NewUser {
  username: string;
  display_name: string;
  email: string;
  roles: string[];
}

/** The temporary password is returned once; never store or log it. */
export function createUser(u: NewUser) {
  return postJson<{ user: User; temporary_password: string }>("/api/v1/users", u);
}

export function updateProfile(id: string, profile: { display_name: string; email: string }) {
  return patchJson<User>(`/api/v1/users/${encodeURIComponent(id)}`, profile);
}

export function setRoles(id: string, roles: string[]) {
  return putJson<User>(`/api/v1/users/${encodeURIComponent(id)}/roles`, { roles });
}

export function setStatus(id: string, status: UserStatus) {
  const action = status === "disabled" ? "disable" : "enable";
  return postJson<User>(`/api/v1/users/${encodeURIComponent(id)}/${action}`);
}

export function resetPassword(id: string) {
  return postJson<{ temporary_password: string }>(`/api/v1/users/${encodeURIComponent(id)}/password-reset`);
}
