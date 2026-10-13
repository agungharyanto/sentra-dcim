---
title: API documentation
audience: admin
order: 90
---

# API documentation

Sentra DCIM is built **API first**: every feature in the interface is available as a REST API under `/api/v1`. The
web interface is just one consumer of that API.

[Open the API reference](/help/api/reference) — endpoints grouped by tag, with method, path, parameters,
request/response schemas and status codes. The reference is **read-only**: there is no button to send requests from
the browser.

## Access

| Endpoint | Current access |
|---|---|
| `/api/v1/health`, `/api/v1/version` | Public, no sign-in |
| `/api/v1/auth/login` | No session (Sign in page) |
| `/api/v1/auth/*`, `/api/v1/ui/bootstrap`, `/api/v1/help/*` | Signed-in users (session cookie) |
| Other endpoints | Not public yet; tokens for machine integrations follow (v0.6) |

Requests that change data (`POST`, `DELETE`, …) must carry the `X-Sentra-CSRF` header with the value of the
`__Host-sentra_csrf` cookie and come from the application's domain; otherwise the answer is `403 csrf_failed`. See
[Signing in and out](../auth/login.md).

## Conventions

- **Times** are sent in UTC (RFC 3339). The display time zone is set by the server (default `Asia/Jakarta`).
- **Errors** use a fixed shape `{"error": {"code", "message", "request_id"}}`. Use `code` (e.g. `not_found`,
  `rate_limited`) in your logic; `message` is for humans only.
- Every response carries an `X-Request-Id` header — include it when reporting a problem.
- **Rate limits** apply per client address; when exceeded the response is `429` with a `Retry-After` header.
- The API version is part of the path (`/api/v1`). Incompatible changes are marked *breaking* in the changelog.
