import { useContext, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useFormatters } from "@/features/bootstrap/regional";

import { sendActivity, type SessionInfo } from "./api";
import { AuthContext, useAuth } from "./auth-context";

/** Interaction that counts as "the user is here" (not mere pointer movement or background requests). */
const ACTIVITY_EVENTS = ["pointerdown", "keydown", "wheel", "touchstart"] as const;
/** At most one activity signal per minute (the server extends idle by the configured minutes). */
const SEND_EVERY_MS = 60_000;
/** The warning opens this long before the session ends (D-069). */
export const WARN_BEFORE_MS = 120_000;

interface Deadlines {
  idle: number;
  absolute: number;
  /** server clock − client clock, so a skewed client clock does not matter. */
  offset: number;
}

function deadlinesOf(s: SessionInfo, serverTime?: string): Deadlines {
  return {
    idle: Date.parse(s.idle_expires_at),
    absolute: Date.parse(s.expires_at),
    offset: serverTime ? Date.parse(serverTime) - Date.now() : 0,
  };
}

/**
 * Keeps the idle timeout honest (v0.3.7, D-069): real interaction sends POST /auth/activity (≤ 1×/min); polling never
 * extends the session. Two minutes before the end a dialog counts down with "Stay signed in" — or, when the absolute
 * limit is reached, only "Sign out now". Kiosk sessions have no limits and no dialog.
 */
export function SessionGuard() {
  const me = useContext(AuthContext)?.me; // absent in isolated shell tests
  if (!me || me.session.kiosk || !me.session.mfa_verified) return null;
  return <Guard key={me.session.id} session={me.session} serverTime={me.server_time} />;
}

function Guard({ session, serverTime }: { session: SessionInfo; serverTime?: string | undefined }) {
  const { t } = useTranslation("auth");
  const fmt = useFormatters();
  const { refresh, signOut } = useAuth();
  const [deadlines, setDeadlines] = useState<Deadlines>(() => deadlinesOf(session, serverTime));
  const [now, setNow] = useState(() => Date.now());
  const lastSent = useRef(0);
  const sending = useRef(false);

  const signal = async () => {
    if (sending.current) return;
    sending.current = true;
    lastSent.current = Date.now();
    try {
      const res = await sendActivity();
      setDeadlines(deadlinesOf(res.session, res.server_time));
    } catch {
      // 401 is handled globally (session ended → login page); anything else: try again on the next interaction.
      lastSent.current = 0;
    } finally {
      sending.current = false;
    }
  };

  // Opening the app is activity too: it also syncs the deadlines and the server clock.
  useEffect(() => {
    const id = window.setTimeout(() => void signal(), 0);
    return () => window.clearTimeout(id);
  }, []);

  useEffect(() => {
    const onActivity = () => {
      if (Date.now() - lastSent.current >= SEND_EVERY_MS) void signal();
    };
    for (const e of ACTIVITY_EVENTS) window.addEventListener(e, onActivity, { passive: true });
    return () => {
      for (const e of ACTIVITY_EVENTS) window.removeEventListener(e, onActivity);
    };
  }, []);

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, []);

  const serverNow = now + deadlines.offset;
  const end = Math.min(deadlines.idle, deadlines.absolute);
  const left = end - serverNow;
  const absoluteBinds = deadlines.absolute <= deadlines.idle;

  useEffect(() => {
    // The deadline passed: re-read the session; the server answers 401 and the app shows the login page.
    if (left <= 0) void refresh();
  }, [left <= 0, refresh]); // eslint-disable-line react-hooks/exhaustive-deps

  const open = left > 0 && left <= WARN_BEFORE_MS;
  const secs = Math.max(0, Math.ceil(left / 1000));
  const time = `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")}`;
  return (
    <Dialog open={open}>
      <DialogContent data-testid="session-warning" onEscapeKeyDown={(e) => e.preventDefault()} onInteractOutside={(e) => e.preventDefault()}>
        <DialogHeader>
          <DialogTitle>{t("sessionGuard.title")}</DialogTitle>
          <DialogDescription>
            {absoluteBinds
              ? t("sessionGuard.absolute", { time, at: fmt.time(deadlines.absolute - deadlines.offset) })
              : t("sessionGuard.idle", { time })}
          </DialogDescription>
        </DialogHeader>
        <p className="text-center font-mono text-3xl tabular-nums" aria-live="off" data-testid="session-countdown">
          {time}
        </p>
        <DialogFooter>
          <Button variant="outline" onClick={() => void signOut()} data-testid="session-logout">
            {t("sessionGuard.logout")}
          </Button>
          {!absoluteBinds && (
            <Button onClick={() => void signal()} data-testid="session-stay">
              {t("sessionGuard.stay")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** Banner while the password expires soon (D-070); after expiry the forced change page takes over. */
export function PasswordExpiryBanner() {
  const { t } = useTranslation("auth");
  const fmt = useFormatters();
  const me = useContext(AuthContext)?.me;
  const [now] = useState(() => Date.now());
  const at = me?.user.password_expires_at;
  if (!me || !me.user.password_expires_soon || !at) return null;
  const days = Math.max(0, Math.ceil((Date.parse(at) - now) / 86_400_000));
  return (
    <div role="status" className="flex flex-wrap items-center gap-2 border-b bg-status-warning-bg px-4 py-2 text-sm text-status-warning-fg lg:px-6" data-testid="password-expiry-banner">
      <span>{t("passwordExpiry.soon", { count: days, date: fmt.date(at) })}</span>
      <Link to="/account/password" className="font-medium underline underline-offset-4">
        {t("passwordExpiry.change")}
      </Link>
    </div>
  );
}
