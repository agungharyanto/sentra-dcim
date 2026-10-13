# shellcheck shell=bash
# Prints the first line of a test/ops run: which committed code is running and whether the working tree is
# clean, so results from uncommitted code are obvious (CLAUDE.md rule 13, v0.2.9). Usage (first statement of main):
#   source "$(dirname "$0")/../lib/source-rev.sh"; source_rev_line "$0"
source_rev_line() {
  local dir top rev state
  dir="$(cd "$(dirname "$1")" && pwd)"
  top="$(git -C "$dir" rev-parse --show-toplevel 2>/dev/null)" || { echo "== $(basename "$1") @ (not a git checkout)"; return 0; }
  rev="$(git -C "$top" describe --tags --always 2>/dev/null)"
  if [[ -z "$(git -C "$top" status --porcelain 2>/dev/null)" ]]; then state=clean; else state=DIRTY; fi
  echo "== $(basename "$1") @ $rev ($state) — $top"
}
