// Sidebar collapsed/expanded, remembered per browser (per-user settings arrive with v0.3.5).
const STORAGE_KEY = "sentra.sidebar";

export function readSidebarCollapsed(): boolean {
  try {
    return window.localStorage.getItem(STORAGE_KEY) === "collapsed";
  } catch {
    return false;
  }
}

export function writeSidebarCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, collapsed ? "collapsed" : "expanded");
  } catch {
    // Storage unavailable: the toggle still works for this page.
  }
}
