---
title: Roles & permissions
audience: admin
order: 41
---

# Roles & permissions

## Purpose

Explains the built-in Sentra DCIM roles and the permissions they grant, so Admins can give every user the narrowest
role that fits (*least privilege*).

## How it works

- **Permissions** (e.g. `users.read`) are defined by the application and checked on every API request.
- A **role** is a set of permissions. A user may have more than one role; their permissions are the union of all
  of them.
- Every signed-in user — whatever the role — can use the Dashboard, the Help Center and their own account settings
  (change password, My sessions).

## Built-in roles

| Role | For | Current permissions |
|---|---|---|
| Admin | Application administrators | All: `users.read`, `users.manage`, `roles.read`, `system.version.read`, `ui.design_system`, `audit.read`, `audit.export`, `help.admin`, `settings.manage`, `facility.read`, `facility.manage`, `rack.read`, `rack.manage` |
| NOC | 24/7 monitoring operators | `facility.read`, `rack.read` (grows with the monitoring & alarm modules) |
| Security | Security & physical access staff | `facility.read`, `rack.read` (grows with the access & visitor modules) |
| Technician | Facility & equipment technicians | `facility.read`, `rack.read` (grows with the asset & maintenance modules) |
| Viewer | Management/stakeholders, read only | `facility.read`, `rack.read` |
| Auditor | Internal/external auditors | `users.read`, `roles.read`, `audit.read`, `audit.export`, `facility.read`, `rack.read` |

The current list is always shown on the **Roles** tab of the Users & Roles menu, with the number of users per role.

## What the permissions mean

| Permission | Allows |
|---|---|
| `users.read` | Viewing the user list and user details |
| `users.manage` | Creating, editing, assigning roles, disabling/enabling and resetting passwords of users |
| `roles.read` | Viewing the role list and its permissions |
| `system.version.read` | Viewing application build details (commit, Go version, server clock) |
| `ui.design_system` | Opening the UI component showcase page (for developers) |
| `audit.read` | Viewing the audit log and the hash-chain integrity status, running a verification ([Audit Log](../audit/index.md)) |
| `audit.export` | Exporting the audit log to CSV (the export itself is recorded) |
| `help.admin` | Reading Admin-only Help Center guides, including the API reference |
| `facility.read` | Viewing sites, buildings, floors, rooms and the [Facility](../facility/index.md) tree |
| `facility.manage` | Adding, editing and archiving buildings, floors, rooms (and sites when global) |
| `rack.read` | Viewing racks |
| `rack.manage` | Adding, editing, renaming, moving and archiving racks |

## Per-site roles

Besides **global** roles (every site), an Admin can give roles **per site** on the user details → **Per-site
access**. A per-site role grants only the site data permissions (`facility.*`, `rack.*`) for that site; other
permissions (users, settings, audit) come from global roles only. Admin cannot be given per site.

Example: a technician of the Denpasar site → no global role, role **Technician** for site `DPS1`. They see `DPS1`
data only. If they also get the global Viewer role they see **every** site. An account keeps at least one role
(global or per site).

## Related API endpoints

`GET /api/v1/roles` (permission `roles.read`) — roles, permissions per role and number of users.
`PUT /api/v1/users/{id}/site-roles` (permission `users.manage`) — replace all per-site roles of a user.
