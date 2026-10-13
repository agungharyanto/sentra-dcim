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

/** Only same-site paths are accepted as a post-login destination (no open redirect). */
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/dashboard";
  if (next.startsWith("/login") || next.startsWith("/change-password")) return "/dashboard";
  return next;
}
