---
title: Regional settings
audience: user
order: 10
---

# Regional settings

## Purpose

Set the **default** language, time zone and date/time formats for every user. Times are still stored in UTC; these
settings only change how they are **displayed**.

## Fields

| Field | Choices | Default |
|---|---|---|
| System language | Indonesia, English | Indonesia (may differ when set by the server administrator) |
| Time zone | IANA zone name, e.g. `Asia/Jakarta` (WIB), `Asia/Makassar` (WITA), `Asia/Jayapura` (WIT), `UTC` | `Asia/Jakarta` |
| Time format | 24-hour (`14:30`), 12-hour (`2:30 PM`) | 24-hour |
| Date format | `DD/MM/YYYY` (`14/10/2026`), `YYYY-MM-DD` (`2026-10-14`), `DD MMM YYYY` (`14 Oct 2026`) | `DD/MM/YYYY` |

## Steps

1. Open **Settings** → **Regional**.
2. Change the fields you need. The **Preview** below the form shows the result immediately.
3. Choose **Save**. The change applies on the next page load for users who have **no** preference of their own for that
   field.
4. **Reset to default** next to a field restores its default value (also audited).

Unknown time zones are rejected with a message on the field.

## Relation to user preferences

Every user may choose their own values under [Preferences](../auth/preferensi.md). The order is: **user preference** →
when "Follow system", the **system setting** on this page → when not set, the **default**.

Following the effective settings: every date and time in the application (Audit Log, My sessions, Users, the header
clock), the date picker of filters, and the local time column of the Audit Log CSV export (the UTC column stays).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/settings/regional` | `settings.manage` |
| `PUT /api/v1/settings/regional` | `settings.manage` |
| `GET /api/v1/auth/me` (field `regional` = effective values) | every signed-in user |
