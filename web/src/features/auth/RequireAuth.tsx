import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router";

import { useAuth } from "./auth-context";

/**
 * Route guard: without a session → /login?next=<here>; then the MFA step (/mfa) and, while the password must be
 * changed, /change-password — unless this route is that step.
 */
export function RequireAuth({ children, step }: { children: ReactNode; step?: "mfa" | "password" }) {
  const { me } = useAuth();
  const { pathname, search } = useLocation();
  if (!me) {
    return <Navigate to={`/login?next=${encodeURIComponent(pathname + search)}`} replace />;
  }
  if (step === "mfa") return children;
  if (!me.session.mfa_verified) {
    return <Navigate to={`/mfa?next=${encodeURIComponent(pathname + search)}`} replace />;
  }
  if (me.user.must_change_password && step !== "password") {
    return <Navigate to={`/change-password?next=${encodeURIComponent(pathname + search)}`} replace />;
  }
  return children;
}
