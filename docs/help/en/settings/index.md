---
title: Settings
audience: user
menu: settings
order: 50
---

# Settings

## Purpose

Configure how Sentra DCIM behaves for **all users**. Since v0.3.5 the **Regional** section is available (language, time
zone, time format, date format). Further sections follow the roadmap (Allowed IP, session & password policy).

## Who can use it

| Action | Role |
|---|---|
| View and change system settings | Admin (permission `settings.manage`) |
| Change your **own** regional preferences | Every user — via [Preferences](../auth/preferensi.md), not this menu |

Other roles do not see this menu; opening its address directly shows *Access denied*.

## Principles

- Every setting has a **default value**. Until it is changed the default applies; **Reset to default** removes the
  change.
- Values are validated before saving (allowed choices, known time zone names). If one field is wrong, nothing is saved.
- Every change is **recorded in the Audit Log** (action `settings.update`): who, when, old value → new value.

Details: [Regional](regional.md).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/settings/{group}` (e.g. `regional`) | `settings.manage` |
| `PUT /api/v1/settings/{group}` (body `{"values": {"name": value or null}}`) | `settings.manage` |

Schema details: [API reference](/help/api/reference) (Admin only).
