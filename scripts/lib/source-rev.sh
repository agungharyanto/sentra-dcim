# shellcheck shell=bash
# Prints the first line of a test/ops run: which committed code is running and whether the working tree is
# clean, so results from uncommitted code are obvious (CLAUDE.md rule 13, v0.2.9). Usage (first statement of main):
#   source "$(dirname "$0")/../lib/source-rev.sh"; source_rev_line "$0"
#
# Works under sudo too (ISS-038): the repository top level is found by walking up to `.git` (no git call), and git
# is told that exactly this repository — plus the main repository of a linked worktree — is safe via
# `-c safe.directory=…` on the command line. Root's global git config is never modified.
source_rev_line() {
  local dir top gitfile common rev state
  dir="$(cd "$(dirname "$1")" && pwd)"
  top="$dir"
  while [[ "$top" != / && ! -e "$top/.git" ]]; do top="$(dirname "$top")"; done
  if [[ ! -e "$top/.git" ]]; then echo "== $(basename "$1") @ (not a git checkout)"; return 0; fi
  common="$top"
  if [[ -f "$top/.git" ]]; then                         # linked worktree: ".git" file → gitdir in the main repo
    gitfile="$(sed -n 's/^gitdir: //p' "$top/.git")"
    common="${gitfile%/.git/worktrees/*}"
  fi
  local g=(git -c "safe.directory=$top" -c "safe.directory=$common" -C "$top")
  rev="$("${g[@]}" describe --tags --always 2>/dev/null)" || rev="(git error)"
  if [[ -z "$("${g[@]}" status --porcelain 2>/dev/null)" ]]; then state=clean; else state=DIRTY; fi
  echo "== $(basename "$1") @ $rev ($state) — $top"
}
