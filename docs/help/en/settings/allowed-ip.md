---
title: Allowed IP
audience: user
order: 20
---

# Allowed IP

## Purpose

Restrict **who may open the Sentra DCIM web panel** by IP address or network (e.g. only from the office network and
the NOC room). Applies to the web interface and the API. Restricting administrative access to the server follows in v0.12
together with the WireGuard VPN.

## Who can use it

| Action | Role |
|---|---|
| View and change the list and the mode | Admin (permission `settings.manage`) |

## Modes

| Mode | Behaviour |
|---|---|
| **Off** (default) | Every IP may. Nothing is recorded. |
| **Monitor** | Every IP may, but access from IPs outside the list is recorded in the Audit Log (`security.allowlist_outside`). Use it for a few days to make sure the list is complete. |
| **Enforce** | Only IPs in the list may; others get "Access from this IP address is not allowed" (`security.allowlist_blocked` in the Audit Log). |

In every mode the service **health check** and **HTTPS certificate renewal** stay open (shown under "Always open").

## Steps

1. Open **Settings** → **Allowed IP**. The **Your current IP** card shows the address the server sees and whether it is
   in the list.
2. Add networks: enter an **IP or network (CIDR)** — one address (`A.B.C.D`) or a network (`A.B.C.0/24`,
   IPv6 too) — and a **Label**, then **Add**. **Add my IP** adds your own address.
3. Choose **Monitor**, save, and check the Audit Log (action filter `security`) for a few days: every legitimate IP
   showing up there needs to be added.
4. Choose **Enforce** and save. Confirm the dialog. Within **2 minutes** press **Keep** on the banner that appears. If
   you do not (e.g. you turned out to be locked out), the mode **returns to Monitor automatically**.

### Safeguards against locking yourself out

- **Enforce** cannot be saved while your current IP is not in the list.
- While Enforce is active, removing the network that covers your own IP is refused.
- The 2-minute confirmation with automatic return to Monitor (step 4).
- If everything else fails, the server operations staff can switch Allowed IP off on the server (the emergency
  procedure is in the disaster recovery document).

### Notes

- The IP is taken from the direct connection to the server. Headers sent by the browser (e.g. `X-Forwarded-For`) are
  not trusted, so they cannot be forged.
- Users behind the office NAT show up with the office's public IP. Home/mobile IPs usually change; use the office
  network or a VPN.
- The widest network accepted is /8 (IPv4) or /16 (IPv6). Host bits must be zero: write `A.B.C.0/24`, not
  `A.B.C.5/24`.
- Every change of the list and the mode is recorded in the Audit Log (`allowlist.add`, `allowlist.update`,
  `allowlist.remove`, `allowlist.mode`, `allowlist.confirm`, `allowlist.rollback`).

## Related API endpoints

| Method & path | Permission |
|---|---|
| `GET /api/v1/settings/allowed-ips` | `settings.manage` |
| `POST /api/v1/settings/allowed-ips` (`{"cidr", "label"}`) | `settings.manage` |
| `PATCH /api/v1/settings/allowed-ips/{id}` (`{"label"}`) | `settings.manage` |
| `DELETE /api/v1/settings/allowed-ips/{id}` | `settings.manage` |
| `PUT /api/v1/settings/allowed-ips/mode` (`{"mode": "off" \| "monitor" \| "enforce"}`) | `settings.manage` |
| `POST /api/v1/settings/allowed-ips/confirm` | `settings.manage` |
