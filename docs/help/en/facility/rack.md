---
title: Racks and their names
audience: user
order: 36
---

# Racks and their names

## Purpose

Explains the rack naming rules, what is recorded for each rack, what the statuses mean and how to move a rack.

## Rack naming rules

Rack names are **free** and follow your site's convention (e.g. `R-A01`, `MMR-01`, `Row A/03`):

- 1–64 characters: letters A–Z/a–z, digits, space and `.` `_` `-` `/`.
- Starts with a letter or digit. Leading/trailing spaces are removed automatically.
- **Unique per site**, ignoring upper/lower case (`r-a01` = `R-A01`), among racks that are not archived.
- Racks are linked to other data by an internal ID, so **renaming never breaks anything**. Every rename is
  recorded in the Audit Log as `rack.rename` with the old and new name.

## Rack data

| Data | Notes |
|---|---|
| Asset tag | Optional; unique for ever (stays recorded even when the rack is archived) |
| Row, position in row | Optional, for floor plans (e.g. row `A`, position `3`) |
| Height (U) | 1–60, default 42U |
| First U number, descending U numbers | Unit numbering: by default from U1 at the bottom; tick "descending" when U1 is at the top |
| Width | 19 or 23 inch |
| Depth, max. load, max. power | Optional (mm, kg, kW) |

## Rack status

| Status | Meaning |
|---|---|
| Planned | Not installed yet (default of a new rack) |
| Active | Installed and in use |
| Reserved | Set aside for a tenant or project |
| Decommissioning | Being emptied for removal |
| Retired | No longer used (usually archived afterwards) |

A status is always shown as colour **and** icon **and** text.

## Moving a rack

**Edit** → choose another **Room**. A rack can only move to a room of the **same site**; moves between sites are
refused.

## Who can use it

View: all roles (`rack.read`). Add, edit, rename, move and archive: Admin (`rack.manage`). See
[Facility](index.md).

## Related API endpoints

`GET /api/v1/racks`, `GET /api/v1/racks/{id}` (`rack.read`); `POST /api/v1/racks`, `PATCH /api/v1/racks/{id}`,
`POST /api/v1/racks/{id}/archive` (`rack.manage`, PATCH/archive require `If-Match`).
