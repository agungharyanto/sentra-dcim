---
title: Signing in and out
audience: user
order: 26
---

# Signing in and out

## Purpose

Sign in to Sentra DCIM with your personal account so every action is recorded under your name, and sign out when
you are done.

## Signing in

1. Open the Sentra DCIM address; the **Sign in** page appears.
2. Enter your **Username** (or NIP) and **Password**, then choose **Sign in**.
3. Enter the 6-digit code from the authenticator app on your phone. At your first sign-in you set it up first
   ([Two-step verification](mfa.md)).
4. If the account was just created or its password was reset, you are asked to
   [change the password](ganti-password.md) right away, before using the application.
5. You then land on the page you wanted to open (or the [Dashboard](../dashboard/index.md)). Your name is shown in
   the top right.

## Messages you may see

| Message | Meaning and action |
|---|---|
| **Incorrect username or password.** | Check the spelling and Caps Lock. The message is deliberately the same for every cause, including a locked or disabled account. |
| **Too many sign-in attempts. Try again in … minutes.** | Wait for the stated time. |
| **Your session has ended.** | The session ended after inactivity, reached its time limit, or was revoked. Sign in again. |

**Locked account:** after 5 wrong passwords the account is locked for 15 minutes and then unlocks by itself. If you
forgot your password, contact your administrator.

## Signing out

Choose your name in the top right, then **Sign out**. The session is revoked on the server immediately. Closing
the browser also removes the session cookie, but **Sign out** is safer, especially on shared computers.

## Session time limits

- A session ends after **30 minutes without activity** (clicks, typing, scrolling; automatic page updates do not
  count). Two minutes before, a warning appears with **Stay signed in**.
- A session also ends at the latest **12 hours after signing in**, even while you are active.
- An Admin can change both limits ([Session & password policy](../settings/session-password.md)); monitoring (kiosk)
  accounts have no such limits.

## Who can use it

Every user with an account.

## Related API endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/auth/login` | Sign in; creates a new session |
| `POST /api/v1/auth/logout` | Sign out; revokes this session |
| `GET /api/v1/auth/me` | The signed-in user |

Full details are in the [API reference](/help/api/reference).
