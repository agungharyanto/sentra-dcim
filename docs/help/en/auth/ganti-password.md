---
title: Change password
audience: user
order: 27
---

# Change password

## Purpose

Change your account password: **mandatory** on first sign-in (the temporary password from your administrator),
and voluntarily at any time, for example if you suspect someone else knows it.

## Password rules

- Length **12–128 characters**. No mandatory upper-case letters, digits or symbols.
- A long phrase you can remember (e.g. several random words) is safer than a short complex word.
- It must differ from the current password **and from your last few passwords** (default 5, set by an Admin);
  otherwise the message "This password was used recently" appears. Do not reuse a password from another service.
- A password is **valid for 90 days** (set by an Admin; accounts whose only role is Viewer do not expire). From 14 days
  before, a banner at the top of the application reminds you. After it expires you must choose a new password after
  signing in. See [Session & password policy](../settings/session-password.md).

## Steps — first password

1. Sign in with the temporary password. The **Change password** page appears automatically; other pages are not
   available yet.
2. Fill in **Current password** (the temporary one), **New password** and **Repeat new password**.
3. Choose **Save password**. You continue straight into the application.

## Steps — voluntary change

1. Choose your name in the top right, then **Change password**.
2. Fill in the three fields, then choose **Save password**.

After the change, **every other session** of your account (other devices) is signed out. The session you are
using stays active. See [My sessions](sesi.md).

## Messages you may see

| Message | Action |
|---|---|
| **The current password is incorrect.** | Check your old password. |
| **The new password must be 12–128 characters.** | Use a longer password. |
| **The repeated new password does not match.** | Type it again carefully. |

## Who can use it

Every user, for their own account. Forgot your password? Contact your administrator.

## Related API endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/auth/password` | Change own password; revokes other sessions |
