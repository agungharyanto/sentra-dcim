---
title: Session & password policy
audience: user
order: 30
---

# Session & password policy

## Purpose

Set how long a session may stay unused, how long a session lasts at most, when passwords must be changed, and how many
old passwords may not be used again.

## Who can use it

| Action | Role |
|---|---|
| Change the policy | Admin (permission `settings.manage`) |

## Fields

| Field | Range | Default | Notes |
|---|---|---|---|
| Idle limit (minutes) | 5–240 | 30 | Without activity for this long, the user is signed out automatically. A warning appears 2 minutes before. |
| Absolute limit (hours) | 1–24 | 12 | The longest a session lasts, active or not. May not be shorter than the idle limit. |
| Password validity (days) | 0 or 30–365 | 90 | 0 = never expires. Does not apply to accounts whose only role is Viewer. |
| Warning before expiry (days) | 1–60 | 14 | Must be shorter than the validity. |
| Password history | 1–24 | 5 | That many last passwords (the current one included) may not be re-used. |

## Steps

1. Open **Settings** → **Session & password**.
2. Change the numbers you need and **Save**. **Reset to default** appears on changed fields.
3. Changes are recorded in the Audit Log (`settings.update`, old → new value) and apply within seconds.

## How it works

- **What counts as activity:** clicks, taps, typing or scrolling in the application, and actions that save data.
  Automatic page updates (e.g. the clock or data that reloads by itself) do **not** count, so a screen left open still
  signs out automatically.
- **Warning:** 2 minutes before the automatic sign-out a countdown dialog appears with **Stay signed in**. When the
  absolute limit is reached the session cannot be extended — save your work and sign in again.
- New idle and absolute limits apply to sessions created after the change (idle also on the next activity).
- **Expired passwords:** during the warning period a banner appears at the top of the application. Afterwards the user
  must choose a new password after signing in before using the application.
- **Monitoring (kiosk)** accounts are not subject to the idle and absolute limits — see [Users & Roles](../users/index.md).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET` / `PUT /api/v1/settings/session` (`idle_minutes`, `absolute_hours`) | `settings.manage` |
| `GET` / `PUT /api/v1/settings/password` (`expiry_days`, `warn_days`, `history`) | `settings.manage` |
| `POST /api/v1/auth/activity` (activity signal from the application) | every signed-in user |
