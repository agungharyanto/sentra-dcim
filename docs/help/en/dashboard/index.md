# Dashboard

**Module:** dashboard · **Available since:** v0.2.3 · **Status:** Available

## Purpose
The first page after opening Sentra DCIM: an overview of the data center (power, environment, alarms) for NOC staff
and facility managers. **No devices or simulators are connected yet**, so the Dashboard only shows "No devices
connected yet" — no sample numbers or charts. Data appears once Telemetry Ingestion ships (v0.6).

## Required role
| Action | Role |
|---|---|
| View the Dashboard | Holder of the temporary basic_auth credentials (until Auth v0.3, see [admin/web-basic-auth.md](../../admin/web-basic-auth.md)); from v0.3: every signed-in role |

## Steps
1. Open `https://sentra.bajastu.id/` and enter the user name and password. `/` redirects to `/dashboard`.
2. Navigation lives in the **left sidebar**, grouped into Monitoring, Infrastructure, Operations and System. Menus
   that are not ready yet are marked **Coming soon**; opening one shows a short description of that module.
3. **Collapse the sidebar** with the button at the bottom left (icons only; hover or focus an icon to see its name).
   The browser remembers this choice.
4. On small screens (phone/tablet), open the menu with the **☰** button at the top left; close it with **Esc** or ×.
5. The header shows the page title and breadcrumb, a **clock** in the server time zone (e.g. WIB / GMT+7), the
   **language** switch (ID/EN) and the **theme** (Dark/Light/Follow system).
6. The application version is shown at the bottom of the sidebar — quote it when reporting a problem.
7. Keyboard users: press **Tab** once for the **Skip to content** link.

## Related API endpoints
| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/ui/bootstrap` | Menu registry (groups, status, order), default language & time zone, version — see [api/ui-bootstrap.md](../../api/ui-bootstrap.md) |

## FAQ / troubleshooting
- **"Page not found"**: the address does not exist; use the sidebar or *Back to Dashboard*.
- **"Something went wrong"**: the page failed to render; the sidebar still works. If it repeats, report it together
  with the application version.
- **Clock differs from my local time**: it follows the server time zone (`DISPLAY_TIMEZONE`), not your device's.
