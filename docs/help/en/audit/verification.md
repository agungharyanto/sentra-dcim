---
title: Audit log integrity verification
audience: user
order: 10
---

# Audit log integrity verification

## Purpose

Make sure no audit record was silently changed, deleted or inserted — even by someone with direct database access.

## How it works

- Every event gets a sequence number (**seq**) and a SHA-256 **hash** computed from the event's content **plus the
  previous event's hash**. The events form a *hash chain*: changing one event makes its hash mismatch, and deleting one
  breaks the sequence.
- The database computes the hash when the event is written, one at a time, so the chain cannot fork even when many
  events arrive together. The application cannot change or delete events (its database access can only add and read).
- **Verification** recomputes the whole chain independently and compares. The application runs it **automatically
  every day**; a failure is logged as an error in the system log (ready to be forwarded to the Alarm Engine).
- **Anchors in backups:** every daily backup stores the hash of the newest event at that moment. If someone rewrote
  the *entire* chain (so it looks consistent again), the hashes stored in the backups no longer match. The check
  against anchors is run by the server operations staff.

## Steps

1. Open **Audit Log**. The **Hash chain integrity** card shows the latest result: **Intact**, **Broken**, or **Not
   verified yet** (since the application last restarted), with its time and the number of events checked.
2. Choose **Verify now** to recompute it now. Only one verification runs at a time. A manual verification is recorded
   as an `audit.verify` event.
3. If the result is **Broken**, the card names the first affected event number (**seq**) and the kind of problem:
   content changed, event missing, or chain rewritten.

## If verification fails

1. Do **not** change anything on the server; note the time and the reported event number.
2. Report it to an Admin and the server operations staff. They compare the chain with the backup anchors and the
   archives and, if needed, restore the database from the newest intact backup.
3. Export the CSV around that event number for the investigation.

After a database restore, verification is run again to confirm the restored chain is intact.

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/audit/integrity` | `audit.read` |
| `POST /api/v1/audit/integrity/verify` | `audit.read` |
