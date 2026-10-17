import { ApiError, deleteJson, getJson, postJson } from "@/lib/api";

/** Mirrors api/openapi.yaml#/components/schemas/MfaStatus. */
export interface MfaStatus {
  enabled: boolean;
  enabled_at: string | null;
  recovery_codes_remaining: number;
}

/** Mirrors api/openapi.yaml#/components/schemas/MfaEnrolment. Shown once; never stored. */
export interface MfaEnrolment {
  issuer: string;
  account: string;
  secret: string;
  otpauth_uri: string;
  qr: string[];
}

/** Mirrors api/openapi.yaml#/components/schemas/AuthUser. */
export interface AuthUser {
  id: string;
  username: string;
  display_name: string;
  must_change_password: boolean;
  /** Role keys (label: users:roles.<key>) and the union of their permissions (RBAC, v0.3.2). */
  roles: string[];
  permissions: string[];
  /** Permissions of global role assignments only (v0.4.1, D-084); site-scoped ones are in `permissions` too. */
  global_permissions?: string[];
  /** MFA set up; false → enrolment at sign-in (v0.3.3). */
  mfa_enabled: boolean;
  /** Kiosk/monitoring account (v0.3.7). */
  kiosk?: boolean;
  /** Password expiry (v0.3.7); null = does not expire. */
  password_expires_at?: string | null;
  password_expired?: boolean;
  password_expires_soon?: boolean;
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
  /** false while the session is mfa_pending: only me, logout and auth/mfa/* work. */
  mfa_verified: boolean;
  /** Kiosk session: no idle or absolute limit (v0.3.7). */
  kiosk?: boolean;
}

/** Mirrors api/openapi.yaml#/components/schemas/Regional: effective values (own preference over system). */
export interface Regional {
  language: string;
  timezone: string;
  time_format: "24h" | "12h";
  date_format: "DD/MM/YYYY" | "YYYY-MM-DD" | "DD MMM YYYY";
}

/** Mirrors api/openapi.yaml#/components/schemas/AuthMe. */
export interface AuthMe {
  user: AuthUser;
  session: SessionInfo;
  /** Effective regional settings (v0.3.5). */
  regional?: Regional;
  server_time?: string;
}

/** Response of POST /api/v1/auth/activity (v0.3.7). */
export interface ActivityResult {
  session: SessionInfo;
  server_time: string;
}

/** Signals real user interaction: moves the idle deadline (reads and polling never do, D-069). */
export function sendActivity() {
  return postJson<ActivityResult>("/api/v1/auth/activity");
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

export const mfaStatusKey = ["auth", "mfa"] as const;

export function fetchMfaStatus(signal?: AbortSignal) {
  return getJson<MfaStatus>("/api/v1/auth/mfa", signal);
}

export function startMfaEnrolment() {
  return postJson<MfaEnrolment>("/api/v1/auth/mfa/enrol");
}

/** Activates MFA; returns the 10 recovery codes (shown once). */
export function confirmMfaEnrolment(code: string) {
  return postJson<{ recovery_codes: string[] }>("/api/v1/auth/mfa/enrol/confirm", { code });
}

/** Completes the sign-in with a TOTP code or a recovery code. */
export function verifyMfa(code: string) {
  return postJson("/api/v1/auth/mfa/verify", { code });
}

export function regenerateRecoveryCodes(code: string) {
  return postJson<{ recovery_codes: string[] }>("/api/v1/auth/mfa/recovery-codes", { code });
}
