#!/usr/bin/env bash
# test-glpi-category.sh — probe whether the configured profile can list or
# search ITILCategory via the GLPI REST API. Answers: "can the user realistically
# use the category picker from sshcm, or is it blocked by profile rights?"
#
# Usage:
#   ./scripts/test-glpi-category.sh              # list + search with no filter
#   ./scripts/test-glpi-category.sh "ServiçosTI" # search for that substring
#
# Reads credentials from scripts/.env (or $ENV_FILE / repo_root/.env).

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

load_env_file() {
  local f="$1"; [[ -f "$f" ]] || return 1
  set -a
  # shellcheck disable=SC1090
  source <(grep -vE '^\s*(#|$)' "$f" | sed -E 's/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/\1=\2/')
  set +a
}
env_file=""
[[ -n "${ENV_FILE:-}" && -f "$ENV_FILE" ]] && env_file="$ENV_FILE"
[[ -z "$env_file" && -f "$script_dir/.env" ]] && env_file="$script_dir/.env"
[[ -z "$env_file" && -f "$repo_root/.env" ]] && env_file="$repo_root/.env"
[[ -n "$env_file" ]] && load_env_file "$env_file" && printf '  \033[2mloaded %s\033[0m\n' "$env_file"

: "${GLPI_URL:?set GLPI_URL}"
: "${GLPI_USER_TOKEN:?set GLPI_USER_TOKEN}"
GLPI_APP_TOKEN="${GLPI_APP_TOKEN:-}"

QUERY="${1:-}"

base="${GLPI_URL%/}"
[[ "$base" != */apirest.php ]] && base="$base/apirest.php"

say()  { printf '\n\033[1;34m→ %s\033[0m\n' "$*"; }
ok()   { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
warn() { printf '  \033[1;33m!\033[0m %s\n' "$*"; }
die()  { printf '\n\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

command -v curl >/dev/null || die "missing: curl"
command -v jq   >/dev/null || die "missing: jq"

app_header=()
[[ -n "$GLPI_APP_TOKEN" ]] && app_header=(-H "App-Token: $GLPI_APP_TOKEN")

# ──────────────────────────────────────────────────────────────────────────────
say "bootstrap · initSession"
init_resp=$(curl -sS -w '\n%{http_code}' \
  -H "Authorization: user_token $GLPI_USER_TOKEN" \
  -H 'Accept: application/json' \
  "${app_header[@]}" "$base/initSession")
init_http=$(tail -n1 <<<"$init_resp")
init_body=$(head -n-1 <<<"$init_resp")
[[ "$init_http" == "200" ]] || die "initSession HTTP $init_http"
session=$(jq -r '.session_token' <<<"$init_body")
[[ -n "$session" && "$session" != "null" ]] || die "no session_token"
ok "session acquired"

glpi() {
  local path="$1"; shift
  curl -sS -g -w '\n%{http_code}' \
    -H "Session-Token: $session" \
    -H 'Accept: application/json' \
    "${app_header[@]}" "$@" \
    "$base$path"
}
split_resp() { local raw="$1"; RESP_HTTP="${raw##*$'\n'}"; RESP_BODY="${raw%$'\n'*}"; }

trap 'glpi /killSession >/dev/null 2>&1 || true' EXIT

# ──────────────────────────────────────────────────────────────────────────────
say "1 · GET /ITILCategory  (plain list — version-proof, respects profile rights)"
split_resp "$(glpi '/ITILCategory?range=0-49&expand_dropdowns=false')"
case "$RESP_HTTP" in
  200|206)
    cnt=$(jq 'length // 0' <<<"$RESP_BODY")
    ok "plain GET returned $cnt categor(y/ies)"
    jq -r '.[]? | "   #\(.id)  completename=\(.completename // .name)"' <<<"$RESP_BODY" 2>/dev/null | head -15
    ;;
  204) warn "204 — profile can READ ITILCategory but sees 0 rows (unusual)" ;;
  401) warn "401 — session invalid (should have been caught at initSession)" ;;
  403) warn "403 — profile CANNOT read ITILCategory via REST. Manual-entry in sshcm requires guessing IDs." ;;
  404) warn "404 — endpoint missing (GLPI server misconfigured?)" ;;
  *)   warn "HTTP $RESP_HTTP — body: $(head -c 200 <<<"$RESP_BODY")" ;;
esac

# ──────────────────────────────────────────────────────────────────────────────
say "2 · GET /search/ITILCategory  (what sshcm's picker actually calls)"
q='forcedisplay[0]=2&forcedisplay[1]=1&range=0-24'
if [[ -n "$QUERY" ]]; then
  q+="&criteria[0][field]=1&criteria[0][searchtype]=contains&criteria[0][value]=$(printf %s "$QUERY" | jq -sRr @uri)"
fi
split_resp "$(glpi "/search/ITILCategory?$q")"
case "$RESP_HTTP" in
  200|206)
    total=$(jq -r '.totalcount // 0' <<<"$RESP_BODY")
    rows=$(jq -r '.data | length // 0' <<<"$RESP_BODY")
    ok "search: totalcount=$total  rows=$rows"
    jq -r '.data[]? | "   #\(."2")  \(."1")"' <<<"$RESP_BODY" 2>/dev/null | head -15
    ;;
  204) warn "204 — search returned nothing$([ -n "$QUERY" ] && echo " for \"$QUERY\"")" ;;
  403) warn "403 — search endpoint blocked for this profile even though plain GET may work" ;;
  *)   warn "HTTP $RESP_HTTP — body: $(head -c 200 <<<"$RESP_BODY")" ;;
esac

# ──────────────────────────────────────────────────────────────────────────────
say "3 · getMyProfiles  (which profile is this session using?)"
split_resp "$(glpi '/getMyProfiles')"
if [[ "$RESP_HTTP" == "200" ]]; then
  active=$(jq -r '.active_profile.name // "?"' <<<"$RESP_BODY")
  all=$(jq -r '.myprofiles[]?.name' <<<"$RESP_BODY" | paste -sd, -)
  ok "active profile: $active   (all accessible: $all)"
fi

printf '\n'
ok "Done. Interpretation:"
cat <<EOF
  - Step 1 returning 200 with rows = categories are reachable. You (or your
    users) can pick them in the sshcm manual-ID field, OR we could wire a
    sshcm-side picker that uses the plain GET endpoint (more permissive than
    /search on many installs).
  - Step 1 returning 403 = this GLPI profile has no READ right on
    ITILCategory. The Formcreator form in question is effectively unusable
    from sshcm for this user; escalate: GLPI admin needs to grant the
    profile "read" on ITILCategory (Administration → Profiles → <profile> →
    Management → ITIL items → Category).
  - Step 2 outcome tells you whether the search-based picker is available
    (some installs allow the plain list but lock down /search).
EOF
