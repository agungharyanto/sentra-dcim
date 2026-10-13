---
title: My sessions
audience: user
order: 28
---

# My sessions

## Purpose

See every device that is signed in with your account and revoke sessions you do not recognise or no longer use,
for example a browser on a shared computer.

## Steps

1. Choose your name in the top right, then **My sessions**.
2. The table lists every active session: **Device** (browser and operating system), **IP address**, **Signed in**,
   **Last activity** and **Ends at the latest**. The session you are using is marked **This session**.
3. Choose **Revoke** on another session to sign it out. That device must sign in again.
4. To end this session, use **Sign out** in your name menu.

On phones every session is shown as a card with the same content.

**A session you do not recognise?** Revoke it, then [change your password](ganti-password.md) right away. A
password change also signs out every other session at once.

## Who can use it

Every user; only **your own** sessions are visible and can be revoked.

## Related API endpoints

| Endpoint | Purpose |
|---|---|
| `GET /api/v1/auth/sessions` | List your own active sessions |
| `DELETE /api/v1/auth/sessions/{id}` | Revoke one of your own sessions |
