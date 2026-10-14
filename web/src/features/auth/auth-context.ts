import { createContext, useContext } from "react";

import type { AuthMe } from "./api";

export interface AuthState {
  /** The signed-in user and current session; null without a valid session. */
  me: AuthMe | null;
  /** True after a session ended on its own (expired, revoked elsewhere); shown once on the login page. */
  expired: boolean;
  signIn: (username: string, password: string) => Promise<AuthMe>;
  signOut: () => Promise<void>;
  /** Re-reads /auth/me (e.g. after a password change cleared must_change_password). */
  refresh: () => Promise<void>;
}

export const AuthContext = createContext<AuthState | null>(null);

export function useAuth(): AuthState {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside <AuthProvider>");
  return value;
}

/**
 * Where a signed-in user has to go first: the MFA step (enrol or verify), then a forced password change, else next.
 */
export function pendingStepPath(me: AuthMe, next: string): string | undefined {
  if (!me.session.mfa_verified) return `/mfa?next=${encodeURIComponent(next)}`;
  if (me.user.must_change_password) return `/change-password?next=${encodeURIComponent(next)}`;
  return undefined;
}

/** Only same-site paths are accepted as a post-login destination (no open redirect). */
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/dashboard";
  if (next.startsWith("/login") || next.startsWith("/change-password") || next.startsWith("/mfa")) return "/dashboard";
  return next;
}
