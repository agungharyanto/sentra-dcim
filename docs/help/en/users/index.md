---
title: Users & Roles
audience: admin
menu: users
order: 40
---

# Users & Roles

## Purpose

Manage Sentra DCIM user accounts: create accounts, assign **roles**, change name and e-mail, disable accounts that
are no longer used, and reset the password of users who forgot it. Accounts are **never deleted** — they are
disabled so the audit trail stays intact.

## Who can use it

| Action | Role |
|---|---|
| View the user list and the role list | Admin, Auditor |
| Create, edit, assign roles, disable/enable, reset password, reset MFA | Admin |

Users without permission do not see this menu; opening its address directly shows *Access denied*.
Roles and their permissions: [Roles & permissions](roles.md).

## Steps

### Creating a user
1. Open **Users & Roles** → **Add user**.
2. Enter the **username** (3–64 characters: letters, digits, `.` `_` `-`; an employee ID is fine), the **display
   name**, an optional e-mail, then select at least one role.
3. Select **Save**. The system creates a **temporary password** (format `xxxx-xxxx-xxxx-xxxx`) and shows it
   **only once**. Copy it and hand it to the user through a secure channel (in person, not a group chat).
4. At first sign-in the user must change that temporary password.

The username cannot be changed once the account exists (it is used in the audit log).

### Changing name, e-mail or roles
1. In the user list select **Manage** on the user's row.
2. Change the **Profile** (display name, e-mail) and select **Save profile**, or tick/untick roles and select
   **Save roles**.
3. Role changes apply **immediately** to the user's next request — no new sign-in needed.

### Disabling and re-enabling
- **Disable** ends all of the user's sessions at once and refuses further sign-ins (the sign-in message stays
  generic).
- **Enable** allows signing in again with the same password.

### Resetting a password
1. On the user's page select **Reset password** and confirm.
2. The new temporary password is shown **once**; all of the user's sessions are ended and an account lockout is
   cleared.
3. The user must change that password at sign-in.

For your own account use [Change password](../auth/ganti-password.md).

### Resetting two-step verification (MFA)
If a user lost the phone and the recovery codes: on the user's page select **Reset MFA** and confirm. The user's MFA
and recovery codes are removed, all sessions end, and they set it up again at the next sign-in
([Two-step verification](../auth/mfa.md)). First make sure the request really comes from the account owner (e.g. in
person). Your own MFA cannot be reset here. The **MFA not set up** marker in the list shows accounts without MFA yet.

## Built-in safeguards

| Rule | Why |
|---|---|
| You cannot disable your own account, remove your own Admin role, or reset your own password on this page | Prevents locking yourself out |
| The last active Admin cannot lose the Admin role or be disabled | There is always at least one Admin |
| Temporary passwords are shown once and never recorded | Passwords are only ever stored as hashes |
| Every change is recorded in the audit log (who, when, what) | Accountability |

If every Admin loses access, the server operations staff can create an emergency Admin with a server
administration command (see [Account & sign-in](../auth/index.md)).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/users` (search `q`, filters `status`, `role`, paging `limit`/`offset`) | `users.read` |
| `GET /api/v1/users/{id}` | `users.read` |
| `POST /api/v1/users` | `users.manage` |
| `PATCH /api/v1/users/{id}` | `users.manage` |
| `PUT /api/v1/users/{id}/roles` | `users.manage` |
| `POST /api/v1/users/{id}/disable`, `POST /api/v1/users/{id}/enable` | `users.manage` |
| `POST /api/v1/users/{id}/password-reset` | `users.manage` |
| `POST /api/v1/users/{id}/mfa-reset` | `users.manage` |
| `GET /api/v1/roles` | `roles.read` |

Schema details: [API reference](/help/api/reference).
