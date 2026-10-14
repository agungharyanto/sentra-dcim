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
| Admin | Application administrators | All: `users.read`, `users.manage`, `roles.read`, `system.version.read` |
| NOC | 24/7 monitoring operators | — (grows with the monitoring & alarm modules) |
| Security | Security & physical access staff | — (grows with the access & visitor modules) |
| Technician | Facility & equipment technicians | — (grows with the asset & maintenance modules) |
| Viewer | Management/stakeholders, read only | — (grows with the read-only modules) |
| Auditor | Internal/external auditors | `users.read`, `roles.read` (audit log to follow) |

The current list is always shown on the **Roles** tab of the Users & Roles menu, with the number of users per role.

## What the permissions mean

| Permission | Allows |
|---|---|
| `users.read` | Viewing the user list and user details |
| `users.manage` | Creating, editing, assigning roles, disabling/enabling and resetting passwords of users |
| `roles.read` | Viewing the role list and its permissions |
| `system.version.read` | Viewing application build details (commit, Go version, server clock) |

## Related API endpoints

`GET /api/v1/roles` (permission `roles.read`) — roles, permissions per role and number of users.
