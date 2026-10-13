import type { ReactNode } from "react";
import { Navigate, useLocation } from "react-router";

import { useAuth } from "./auth-context";

/**
 * Route guard: without a session → /login?next=<here>; while the password must be changed → /change-password
 * (unless this route is that page).
 */
export function RequireAuth({ children, passwordChange = false }: { children: ReactNode; passwordChange?: boolean }) {
  const { me } = useAuth();
  const { pathname, search } = useLocation();
  if (!me) {
    return <Navigate to={`/login?next=${encodeURIComponent(pathname + search)}`} replace />;
  }
  if (me.user.must_change_password && !passwordChange) {
    return <Navigate to={`/change-password?next=${encodeURIComponent(pathname + search)}`} replace />;
  }
  return children;
}
