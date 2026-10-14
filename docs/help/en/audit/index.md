---
title: Audit Log
audience: user
menu: audit
order: 45
---

# Audit Log

## Purpose

Trace **who did what, when, from where, and with which result** in Sentra DCIM: sign-in and sign-out, two-step
verification, password changes, user and role management, access denials, and exports and verification of the audit
log itself. Audit records **cannot be changed or deleted** from the application, and every record is locked into a
**hash chain**, so silent changes are detected (see [Integrity verification](verification.md)).

## Who can use it

| Action | Role |
|---|---|
| View the audit log and integrity status, run a verification | Admin, Auditor |
| Export CSV | Admin, Auditor |

Other roles do not see this menu; opening its address directly shows *Access denied*.

## Steps

### Finding events
1. Open **Audit Log**. The newest events are on top; times are shown in **WIB** (UTC+7).
2. Filter by **From** / **To** (WIB), **Actor** (part of a username), **Action** (e.g. `auth.login.failure`, or a
   prefix such as `auth` for every sign-in action), **Result** (success, failure, denied) and **Target** (object ID,
   e.g. a user ID). Choose **Apply**; **Reset** clears the filters.
3. Choose **Load more** below the table for older events.
4. Select a row to see its **details**: every column, the metadata (extra facts such as the failure reason or the
   number of attempts), the request ID and the row's hash.

### Exporting CSV
1. Set the filters as above.
2. Choose **Export CSV**. The file holds up to 50,000 of the newest matching events, with times in UTC and WIB.
3. The export itself is recorded in the audit log (action `audit.export`, with its filters and row count).

Cells starting with `=`, `+`, `-` or `@` get a leading quote `'` so Excel/LibreOffice show them as text instead of
running them as formulas (*CSV injection* protection).

### Reading common events

| Action | Meaning |
|---|---|
| `auth.login.password_ok` → `auth.login.success` | Correct password → correct two-step code (complete sign-in) |
| `auth.login.failure` | Failed sign-in; metadata `reason` names the cause (e.g. `invalid_password`, `unknown_user`, `rate_limited`) |
| `auth.lockout` | Account temporarily locked after too many wrong attempts |
| `auth.mfa.enrol_start`, `auth.mfa.enrol`, `auth.mfa.verify` | Starting and finishing two-step setup, and using it |
| `user.*` | User management by an Admin (create, edit, roles, disable, reset) |
| `authz.denied` | Request refused for a missing permission |
| `security.csrf_denied` | Request refused by the CSRF check (e.g. sent from another site) |
| `….repeated` | Summary of repeated refusals — see below |
| `audit.export`, `audit.verify`, `audit.prune` | CSV export, hash-chain verification, pruning by retention |

**Repeated refusals are summarised.** When the same user or IP address is refused again and again on the same address
within 10 minutes, only the first refusal is recorded; the rest are counted and written as **one**
`authz.denied.repeated` or `security.csrf_denied.repeated` event (metadata `repeated` = count, `first_at`/`last_at`).
That way the log cannot be flooded.

**Never recorded:** passwords (including temporary passwords), session or CSRF tokens, two-step codes, authenticator
secrets and recovery codes. A malformed username on a failed sign-in is not stored either — it is often a password
typed into the username field.

## Retention

Events are kept for **365 days** (adjustable by the server administrator, at least 90 days). Older events are moved to
a compressed archive in the backup location before they are deleted from the database; the hash chain stays
verifiable because the hash of the last pruned event is kept as a **checkpoint** (event `audit.prune`).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/audit/events` (filters `since`, `until`, `actor`, `action`, `result`, `target_type`, `target_id`; paging `limit` + `before`) | `audit.read` |
| `GET /api/v1/audit/events/export.csv` (same filters) | `audit.export` |
| `GET /api/v1/audit/actions` | `audit.read` |
| `GET /api/v1/audit/integrity` | `audit.read` |
| `POST /api/v1/audit/integrity/verify` | `audit.read` |

Schema details: [API reference](/help/api/reference) (Admin only).
