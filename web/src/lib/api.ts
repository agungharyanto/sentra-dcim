/** Error carrying the stable API error code (ErrorBody.error.code); the UI translates it via errors:code.<code>. */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;

  constructor(code: string, status: number) {
    super(`api error ${status}: ${code}`);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

export async function getJson<T>(path: string, signal?: AbortSignal): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      headers: { Accept: "application/json" },
      credentials: "same-origin",
      ...(signal ? { signal } : {}),
    });
  } catch {
    throw new ApiError("network", 0);
  }
  if (!res.ok) {
    let code = "unknown";
    try {
      const body = (await res.json()) as { error?: { code?: unknown } };
      if (typeof body.error?.code === "string") code = body.error.code;
    } catch {
      // Non-JSON error (e.g. proxy 401 page): keep "unknown".
    }
    throw new ApiError(code, res.status);
  }
  return (await res.json()) as T;
}
