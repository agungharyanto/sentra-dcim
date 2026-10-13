/** Error carrying the stable API error code (ErrorBody.error.code); the UI translates it via errors:code.<code>. */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  /** Seconds from a Retry-After header (429), if any. */
  readonly retryAfter?: number;

  constructor(code: string, status: number, retryAfter?: number) {
    super(`api error ${status}: ${code}`);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    if (retryAfter !== undefined) this.retryAfter = retryAfter;
  }
}

/** Double-submit CSRF token set by the server at login (readable cookie, ADR-003). */
const CSRF_COOKIE = "__Host-sentra_csrf";
export const CSRF_HEADER = "X-Sentra-CSRF";

export function csrfToken(): string {
  for (const part of document.cookie.split(";")) {
    const [name, ...rest] = part.trim().split("=");
    if (name === CSRF_COOKIE) return rest.join("=");
  }
  return "";
}

type AuthListener = (code: string) => void;
const authListeners = new Set<AuthListener>();

/**
 * Subscribes to authentication errors from any API call: "unauthenticated" (401: session missing, expired or
 * revoked) and "password_change_required" (403). The AuthProvider uses it to send the user to the right page.
 */
export function onAuthError(listener: AuthListener): () => void {
  authListeners.add(listener);
  return () => authListeners.delete(listener);
}

async function request<T>(method: string, path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  // Unsafe methods carry the CSRF token; before login there is none yet and any non-empty value is accepted.
  if (method !== "GET") headers[CSRF_HEADER] = csrfToken() || "login";
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: "same-origin",
      ...(body !== undefined ? { body: JSON.stringify(body) } : {}),
      ...(signal ? { signal } : {}),
    });
  } catch {
    throw new ApiError("network", 0);
  }
  if (!res.ok) {
    let code = "unknown";
    try {
      const data = (await res.json()) as { error?: { code?: unknown } };
      if (typeof data.error?.code === "string") code = data.error.code;
    } catch {
      // Non-JSON error (e.g. proxy 401 page): keep "unknown".
    }
    const retry = Number(res.headers.get("Retry-After"));
    if (code === "unauthenticated" || code === "password_change_required") {
      for (const l of authListeners) l(code);
    }
    throw new ApiError(code, res.status, Number.isFinite(retry) && retry > 0 ? retry : undefined);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function getJson<T>(path: string, signal?: AbortSignal): Promise<T> {
  return request<T>("GET", path, undefined, signal);
}

export function postJson<T = void>(path: string, body?: unknown): Promise<T> {
  return request<T>("POST", path, body);
}

export function deleteJson<T = void>(path: string): Promise<T> {
  return request<T>("DELETE", path);
}
