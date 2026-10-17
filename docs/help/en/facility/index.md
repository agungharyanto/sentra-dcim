---
title: Facility
audience: user
menu: facility
order: 35
---

# Facility

## Purpose

Records **where everything is**: the hierarchy **Site › Building › Floor › Room › Rack**. Later modules (tenants,
assets/devices, cabling, monitoring) build on it. The **Facility** menu has two tabs:

- **Hierarchy** — the tree on the left, details of the selection on the right (stacked on small screens).
- **Rack list** — all racks with search, site/status filters, sorting and archive.

## Hierarchy

| Level | Holds | Notes |
|---|---|---|
| **Site** | A data center location | The site code (e.g. `DPS1`) is unique and cannot be changed; status Planned / Active / Inactive |
| **Building** | A building of the site | Building code optional |
| **Floor** | A floor of the building | Level: 0 = ground floor, negative = basement |
| **Room** | A room on the floor | Room type: Server room, **Meet-me room (MMR)**, Power room, Cooling room, NOC, Storage, Staging, Other |
| **Rack** | A rack in the room | Free name, see [Racks and their names](rack.md) |

Names are unique within their parent (buildings per site, floors per building, rooms per floor), ignoring upper/lower
case. Rack names are unique **per site**.

### Meet-me room (MMR)

A room of type **MMR** is where cables between tenants and carriers meet (cross-connects). In the tree and the rack
list MMR rooms carry a cable icon. Tracing cables rack ↔ rack and rack ↔ MMR arrives in a later version (v0.4).

## Steps

1. Open **Facility** → **Hierarchy**. Select a node in the tree (click, or arrow up/down then Enter; arrow
   right/left opens/closes a branch).
2. **Add site** (button above the tree), then on the details of each node **Add building / floor / room / rack**.
3. **Edit** changes the node; racks also have **Rename**.
4. **Archive** hides a node without deleting it. A node with active contents cannot be archived — archive its
   contents first (bottom-up: rack → room → floor → building → site).
5. In the **Rack list**, tick **Show archived** to see archived racks. Click a rack name to open it in the hierarchy.

### Archive, never delete

Facility data is never deleted: archiving keeps the history, and every change is recorded in the
[Audit Log](../audit/index.md) (values before and after). Archived names may be reused; site codes and rack asset
tags stay recorded and cannot be reused.

### "Someone else changed this data"

Every item has a version. If someone else saved a change after you opened the form, your save is refused with
**Someone else changed this data** — choose **Reload**, check the latest version, then repeat your change. This keeps
changes from overwriting each other.

## Who can use it

| Action | Role |
|---|---|
| View the hierarchy and racks | All roles (Admin, NOC, Security, Technician, Viewer, Auditor) — permissions `facility.read`, `rack.read` |
| Add, edit, archive buildings/floors/rooms | Admin — permission `facility.manage` |
| Add, edit, rename, move, archive racks | Admin — permission `rack.manage` |
| Add and archive **sites** | Admin with **global** access |

### Per-site access

Roles can be given **globally** (every site) or **per site** (Users & Roles → user details → **Per-site access**).
A user with per-site roles only sees that site's data: other sites appear neither in the tree nor in lists, and their
addresses answer "not found". To really limit a user to certain sites, give them **no** global role. Admin is always
global. See [Roles & permissions](../users/roles.md).

## Related API endpoints

| Endpoint | Permission |
|---|---|
| `GET /api/v1/facility/tree` | `facility.read` |
| `GET /api/v1/sites`, `GET /api/v1/sites/{id}` (also `buildings`, `floors`, `rooms`) | `facility.read` |
| `POST /api/v1/sites`, `PATCH /api/v1/sites/{id}`, `POST /api/v1/sites/{id}/archive` (also `buildings`, `floors`, `rooms`) | `facility.manage` |
| `GET /api/v1/racks`, `GET /api/v1/racks/{id}` | `rack.read` |
| `POST /api/v1/racks`, `PATCH /api/v1/racks/{id}`, `POST /api/v1/racks/{id}/archive` | `rack.manage` |

Lists support `q`, parent filters (`site_id`, `building_id`, `floor_id`, `room_id`), `status`, `room_type`,
`include_archived`, `sort`, `limit`/`offset`. Changes require an `If-Match` header with the latest `ETag` (`428` when
missing, `412` when someone else changed the data). Details in [API documentation](../api/index.md) and the
[API reference](/help/api/reference).
