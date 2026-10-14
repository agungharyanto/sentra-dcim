---
title: Preferences
audience: user
order: 29
---

# Preferences

## Purpose

Choose the language, time zone, time format and date format **for your own account**, different from the system
settings — for example staff in WITA who want to see `Asia/Makassar` times.

## Who can use it

Every signed-in user (no special permission).

## Steps

1. Open the account menu (your name, top right) → **Preferences**.
2. For each field choose your own value, or **Follow system** (the current system value is shown next to it).
3. Choose **Save**. The display changes immediately; the **Preview** shows the result.

The language preference applies every time you **sign in** and when you save your preferences. The **ID/EN** buttons
in the header only switch the language in the browser you are using (they do not change your account preference).

While a field is on "Follow system", changes an Admin makes in [Regional settings](../settings/regional.md) apply to
you as well. Every preference change is recorded in the Audit Log (`user.preferences.update`).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/me/preferences` | every signed-in user |
| `PUT /api/v1/me/preferences` (body `{"values": {"regional.timezone": "Asia/Makassar"}}`, `null` = follow system) | every signed-in user |
