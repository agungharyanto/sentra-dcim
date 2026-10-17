#!/usr/bin/env bash
# Facility API test (v0.4.1) through the real Caddy: permission matrix of the 6 system roles on the facility and
# rack endpoints, site scope (a user with a role for site A only sees A: lists filtered, B by ID = 404, cannot create
# sites), ETag/If-Match (428/412), archive rules. It CREATES sites, racks and a test account, so it refuses to run
# against anything but a local throwaway stack (CI).
#
# One test account is enough: role changes apply on the next request, so the Admin switches its role per step
# (keeps the sign-ins per IP far below the login rate limit).
# Usage: BASE_URL=https://localhost SENTRA_APP_USER=… SENTRA_APP_PASS=… SENTRA_APP_TOTP_SECRET=… test/api/facility.sh
set -uo pipefail

main() {
# shellcheck source=/dev/null
source "$(dirname "$0")/../../scripts/lib/source-rev.sh"
source_rev_line "$0"
BASE="${BASE_URL:-https://localhost}"
BASE="${BASE%/}"
host="${BASE#*://}"; host="${host%%[:/]*}"
if [[ "$host" != localhost && "$host" != 127.0.0.1 ]]; then
  echo "REFUSED: facility.sh creates data; it only runs against a local throwaway stack (got $host)" >&2
  exit 2
fi
root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root" || exit 2
env_domain="$(sed -n 's/^DOMAIN=//p' .env 2>/dev/null | tail -1)"
if [[ -n "$env_domain" && "$env_domain" != localhost ]]; then
  echo "REFUSED: .env has DOMAIN=$env_domain (a real deployment); facility.sh only runs on a throwaway stack" >&2
  exit 2
fi
: "${SENTRA_APP_USER:?}" "${SENTRA_APP_PASS:?}" "${SENTRA_APP_TOTP_SECRET:?}"
pass=0 fail=0
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# A failure also prints the last HTTP status and error code (no bodies: they may hold temporary passwords).
check() { if [[ "$2" == "0" ]]; then printf '  PASS  %s\n' "$1"; pass=$((pass+1)); else printf '  FAIL  %s (last: %s %s)\n' "$1" "${code:-}" "$(jq -r '.error.code // empty' "$tmp/b" 2>/dev/null)"; fail=$((fail+1)); fi; }
totp() { printf '%s' "$1" | test/lib/totp.py; }
# http <jar> <method> <path> [json] [extra curl args…] → $code, body in $tmp/b, headers in $tmp/h
http() {
  local jar="$1" method="$2" path="$3" data="${4:-}"; shift 4 2>/dev/null || shift $#
  local csrf; csrf="$(awk '$6=="__Host-sentra_csrf"{print $7}' "$jar" 2>/dev/null)"
  local args=(-sk -D "$tmp/h" -o "$tmp/b" -w '%{http_code}' -b "$jar" -c "$jar" -H "Origin: $BASE" -X "$method")
  [[ "$method" != GET ]] && args+=(-H "X-Sentra-CSRF: ${csrf:-login}")
  [[ -n "$data" ]] && args+=(-H 'Content-Type: application/json' --data-binary "$data")
  local try
  for try in 1 2 3 4 5; do
    code="$(curl "${args[@]}" "$@" "$BASE$path")"
    # The API rate limit (per client IP) answers 429 with Retry-After: wait and repeat, it is not what is tested.
    [[ "$code" == 429 ]] || break
    sleep "$(tr -d '\r' <"$tmp/h" | sed -n 's/^[Rr]etry-[Aa]fter: //p' | head -1 | grep -E '^[0-9]+$' || echo 1)"
  done
  [[ "$code" == 429 ]] && echo "  ...  $method $path still 429 after $try tries" >&2
}
j() { jq -r "$1" "$tmp/b"; }
etag() { tr -d '\r' <"$tmp/h" | sed -n 's/^[Ee][Tt][Aa][Gg]: //p'; }
# signin <jar> <user> <password> <totp-secret>: password + MFA (waits for a fresh 30 s step: anti-replay).
signin() {
  local body; body="$(U="$2" P="$3" jq -n '{username: env.U, password: env.P}')"
  http "$1" POST /api/v1/auth/login "$body"
  [[ "$code" == 200 ]] || { echo "STOP: login $2 = $code" >&2; exit 3; }
  sleep "$((30 - $(date +%s) % 30))"
  http "$1" POST /api/v1/auth/mfa/verify "$(C="$(totp "$4")" jq -n '{code: env.C}')"
  [[ "$code" == 204 ]] || { echo "STOP: MFA $2 = $code $(j .error.code)" >&2; exit 3; }
}

admin="$tmp/admin.jar" user="$tmp/user.jar"
signin "$admin" "$SENTRA_APP_USER" "$SENTRA_APP_PASS" "$SENTRA_APP_TOTP_SECRET"

# Test account (temporary password → enrol MFA → own password).
uname="ci.facility.$RANDOM"
http "$admin" POST /api/v1/users "$(N="$uname" jq -n '{username: env.N, display_name: "CI Facility", roles: ["viewer"]}')"
uid="$(j .user.id)"; temp="$(j .temporary_password)"
check "admin creates the test account" "$([[ $code == 201 && -n $uid ]]; echo $?)"
http "$user" POST /api/v1/auth/login "$(U="$uname" P="$temp" jq -n '{username: env.U, password: env.P}')"
http "$user" POST /api/v1/auth/mfa/enrol ""
secret="$(j .secret | tr -d ' ')"
http "$user" POST /api/v1/auth/mfa/enrol/confirm "$(C="$(totp "$secret")" jq -n '{code: env.C}')"
newpw="facility-$(openssl rand -hex 12)"
http "$user" POST /api/v1/auth/password "$(C="$temp" N="$newpw" jq -n '{current_password: env.C, new_password: env.N}')"
check "test account signed in with MFA and its own password" "$([[ $code == 204 ]]; echo $?)"

# Data: sites A and B, each with building › floor › room (MMR in A) › rack.
mk() { http "$admin" POST "/api/v1/$1" "$2"; j .id; }
sa="$(mk sites '{"code":"CIA","name":"CI Site A"}')"; sb="$(mk sites '{"code":"CIB","name":"CI Site B"}')"
ba="$(mk buildings "{\"site_id\":\"$sa\",\"name\":\"Gedung A\"}")"; bb="$(mk buildings "{\"site_id\":\"$sb\",\"name\":\"Gedung B\"}")"
fa="$(mk floors "{\"building_id\":\"$ba\",\"name\":\"L1\",\"level\":1}")"; fb="$(mk floors "{\"building_id\":\"$bb\",\"name\":\"L1\"}")"
ra="$(mk rooms "{\"floor_id\":\"$fa\",\"name\":\"MMR A\",\"room_type\":\"mmr\"}")"; rb="$(mk rooms "{\"floor_id\":\"$fb\",\"name\":\"Server B\"}")"
ka="$(mk racks "{\"room_id\":\"$ra\",\"name\":\"A-01\"}")"; kb="$(mk racks "{\"room_id\":\"$rb\",\"name\":\"B-01\"}")"
check "admin builds two sites down to racks" "$([[ $ka =~ ^[0-9a-f-]{36}$ && $kb =~ ^[0-9a-f-]{36}$ ]]; echo $?)"

# ETag / If-Match.
http "$admin" GET "/api/v1/racks/$ka" ""; tag="$(etag)"
check "GET rack carries ETag \"1\"" "$([[ $tag == '"1"' ]]; echo $?)"
http "$admin" PATCH "/api/v1/racks/$ka" '{"name":"A-02"}'
check "PATCH without If-Match = 428 precondition_required" "$([[ $code == 428 && $(j .error.code) == precondition_required ]]; echo $?)"
http "$admin" PATCH "/api/v1/racks/$ka" '{"name":"A-02"}' -H "If-Match: $tag"
check "PATCH (rename) with If-Match = 200, ETag \"2\"" "$([[ $code == 200 && $(etag) == '"2"' ]]; echo $?)"
http "$admin" PATCH "/api/v1/racks/$ka" '{"notes":"x"}' -H "If-Match: $tag"
check "PATCH with a stale If-Match = 412 version_conflict" "$([[ $code == 412 && $(j .error.code) == version_conflict ]]; echo $?)"
http "$admin" POST /api/v1/racks "{\"room_id\":\"$ra\",\"name\":\"a-02\"}"
check "duplicate rack name per site (case-insensitive) = 409" "$([[ $code == 409 && $(j .error.code) == duplicate_name ]]; echo $?)"
http "$admin" PATCH "/api/v1/racks/$ka" "{\"room_id\":\"$rb\"}" -H 'If-Match: "2"'
check "moving a rack to another site = 422 cross_site_move" "$([[ $code == 422 && $(j .error.code) == cross_site_move ]]; echo $?)"
http "$admin" GET "/api/v1/rooms/$ra" ""
http "$admin" POST "/api/v1/rooms/$ra/archive" "" -H "If-Match: $(etag)"
check "archiving a room with racks = 409 has_active_children" "$([[ $code == 409 && $(j .error.code) == has_active_children ]]; echo $?)"

# Permission matrix: the test account takes each system role in turn.
for role in admin noc security technician viewer auditor; do
  http "$admin" PUT "/api/v1/users/$uid/roles" "{\"roles\":[\"$role\"]}"
  [[ "$code" == 200 ]] || { check "set role $role" 1; continue; }
  reads=0
  for p in /api/v1/facility/tree /api/v1/sites "/api/v1/sites/$sb" /api/v1/buildings /api/v1/floors /api/v1/rooms /api/v1/racks "/api/v1/racks/$kb"; do
    http "$user" GET "$p" ""; [[ "$code" == 200 ]] || reads=1
  done
  check "$role: reads tree, sites, buildings, floors, rooms, racks (200)" "$reads"
  http "$user" POST /api/v1/buildings "{\"site_id\":\"$sb\",\"name\":\"X $role\"}"
  if [[ $role == admin ]]; then
    check "$role: creates a building (201)" "$([[ $code == 201 ]]; echo $?)"
  else
    check "$role: creating a building = 403 forbidden" "$([[ $code == 403 && $(j .error.code) == forbidden ]]; echo $?)"
  fi
  http "$user" POST "/api/v1/racks/$kb/archive" "" -H 'If-Match: "1"'
  if [[ $role != admin ]]; then
    check "$role: archiving a rack = 403 forbidden" "$([[ $code == 403 && $(j .error.code) == forbidden ]]; echo $?)"
  fi
done

# Site scope: no global role, Technician for site A only.
http "$admin" PUT "/api/v1/users/$uid/site-roles" "{\"site_roles\":[{\"role\":\"technician\",\"site_id\":\"$sa\"}]}"
check "admin assigns Technician for site A" "$([[ $code == 200 ]]; echo $?)"
http "$admin" PUT "/api/v1/users/$uid/roles" '{"roles":[]}'
check "admin removes the global roles" "$([[ $code == 200 ]]; echo $?)"
http "$user" GET /api/v1/sites ""
check "scoped: site list has only CIA" "$([[ $code == 200 && $(j '[.items[].code] | join(",")') == CIA ]]; echo $?)"
http "$user" GET /api/v1/racks ""
check "scoped: rack list has only site A racks" "$([[ $(j '[.items[].site_code] | unique | join(",")') == CIA ]]; echo $?)"
http "$user" GET "/api/v1/racks?site_id=$sb" ""
check "scoped: filtering on site B gives an empty list" "$([[ $code == 200 && $(j .total) == 0 ]]; echo $?)"
nf=0
for p in "/api/v1/sites/$sb" "/api/v1/buildings/$bb" "/api/v1/floors/$fb" "/api/v1/rooms/$rb" "/api/v1/racks/$kb"; do
  http "$user" GET "$p" ""; [[ "$code" == 404 && $(j .error.code) == not_found ]] || nf=1
done
check "scoped: site B objects by ID = 404 not_found" "$nf"
http "$user" GET "/api/v1/racks/$ka" ""
check "scoped: site A rack by ID = 200" "$([[ $code == 200 ]]; echo $?)"
http "$user" GET /api/v1/facility/tree ""
check "scoped: tree shows only site A" "$([[ $(j '[.sites[].code] | join(",")') == CIA ]]; echo $?)"
http "$user" POST /api/v1/sites '{"code":"CIX","name":"Not allowed"}'
check "scoped: creating a site = 403" "$([[ $code == 403 ]]; echo $?)"
http "$user" GET /api/v1/users ""
check "scoped: a site role grants no global permission (users = 403)" "$([[ $code == 403 ]]; echo $?)"
http "$admin" PUT "/api/v1/users/$uid/site-roles" "{\"site_roles\":[{\"role\":\"admin\",\"site_id\":\"$sa\"}]}"
check "Admin cannot be assigned per site (422)" "$([[ $code == 422 ]]; echo $?)"

# Clean up: disable the account (accounts are never deleted).
http "$admin" POST "/api/v1/users/$uid/disable" ""
echo "Facility API: $pass passed, $fail failed"
[[ $fail -eq 0 ]]
}

main "$@"
