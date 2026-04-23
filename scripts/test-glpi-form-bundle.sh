#!/usr/bin/env bash
# test-glpi-form-bundle.sh — diagnose why sshcm's Formcreator drawer is empty.
# Replicates exactly what the backend's new fetch-all-then-filter path does
# against a real GLPI instance, printing counts + sample rows at each step so
# we see which stage returns zero.
#
# Usage:
#   ./scripts/test-glpi-form-bundle.sh <form_id>
#   ./scripts/test-glpi-form-bundle.sh 5

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

FORM_ID="${1:-}"
[[ -n "$FORM_ID" ]] || { echo "usage: $0 <form_id>  (e.g. $0 5)"; exit 1; }

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

cleanup() { glpi /killSession >/dev/null 2>&1 || true; }
trap cleanup EXIT

# ──────────────────────────────────────────────────────────────────────────────
say "1 · GET /PluginFormcreatorForm/$FORM_ID  (is the form even reachable?)"
split_resp "$(glpi "/PluginFormcreatorForm/$FORM_ID?expand_dropdowns=false")"
case "$RESP_HTTP" in
  200) ok "form fetched"
       jq -r '"   name=\(.name)  is_active=\(.is_active)  entities_id=\(.entities_id)"' <<<"$RESP_BODY" ;;
  *)   die "HTTP $RESP_HTTP on form fetch — fix this before continuing" ;;
esac

# ──────────────────────────────────────────────────────────────────────────────
say "2 · GET /PluginFormcreatorSection  (fetch all sections, filter to form=$FORM_ID)"
split_resp "$(glpi '/PluginFormcreatorSection?range=0-199&expand_dropdowns=false')"
case "$RESP_HTTP" in
  200|206) total_sections=$(jq 'length' <<<"$RESP_BODY") ;;
  204) total_sections=0; warn "204 — zero sections across the whole GLPI" ;;
  *) die "HTTP $RESP_HTTP";;
esac
ok "all sections: $total_sections"
# Dump one row so we can see the actual FK field name/type.
echo "  sample row (first):"
jq -c '.[0] // empty' <<<"$RESP_BODY" | head -c 400 | sed 's/^/    /'
printf '\n'

section_ids=$(jq -r --argjson fid "$FORM_ID" '[.[] | select((.plugin_formcreator_forms_id | tostring) == ($fid | tostring)) | .id] | .[]' <<<"$RESP_BODY")
section_count=$(printf '%s\n' "$section_ids" | grep -c . || true)
if [[ "$section_count" -eq 0 ]]; then
  warn "No sections matched form id=$FORM_ID via plugin_formcreator_forms_id — the FK column name on THIS Formcreator may differ."
  echo "  possible FK-ish keys in the first section row:"
  jq -r '.[0] // empty | to_entries[] | select(.key | test("form"; "i")) | "    .\(.key) = \(.value | tostring | .[0:60])"' <<<"$RESP_BODY"
else
  ok "filtered sections (form=$FORM_ID): $section_count"
  for sid in $section_ids; do printf '    section #%s\n' "$sid"; done
fi

# Bail if no sections — the rest of the pipeline needs them.
[[ "$section_count" -gt 0 ]] || { warn "Stopping — can't continue without section ids."; exit 0; }

# ──────────────────────────────────────────────────────────────────────────────
say "3 · GET /PluginFormcreatorQuestion  (fetch all, filter by section ids)"
split_resp "$(glpi '/PluginFormcreatorQuestion?range=0-999&expand_dropdowns=false')"
case "$RESP_HTTP" in
  200|206) total_q=$(jq 'length' <<<"$RESP_BODY") ;;
  204) total_q=0 ;;
  *) die "HTTP $RESP_HTTP";;
esac
ok "all questions: $total_q"
echo "  sample row:"
jq -c '.[0] // empty' <<<"$RESP_BODY" | head -c 500 | sed 's/^/    /'
printf '\n'

# Build jq filter matching any of our section ids.
# Use `tostring` comparisons so int-vs-string quirks don't drop rows.
filter='[]'
if [[ "$section_count" -gt 0 ]]; then
  ids_json=$(printf '%s\n' "$section_ids" | jq -R . | jq -sc '[.[] | tonumber? // tostring]')
  kept=$(jq --argjson ids "$ids_json" '[ .[] | select(([(.plugin_formcreator_sections_id | tostring)]) as $t | $ids | map(tostring) | any(. == $t[0])) ]' <<<"$RESP_BODY" 2>/dev/null || echo '[]')
  kept_count=$(jq 'length' <<<"$kept")
  ok "questions in form $FORM_ID (via section FK): $kept_count"
  jq -r '.[] | "    q#\(.id)  section=\(.plugin_formcreator_sections_id)  type=\(.fieldtype)  name=\(.name)"' <<<"$kept" | head -20
fi

if [[ "${kept_count:-0}" -eq 0 && "$total_q" -gt 0 ]]; then
  warn "No questions matched. Suspect FK column. Fields on the sample question that look form-related:"
  jq -r '.[0] // empty | to_entries[] | select(.key | test("section|form"; "i")) | "    .\(.key) = \(.value | tostring | .[0:60])"' <<<"$RESP_BODY"
fi

# ──────────────────────────────────────────────────────────────────────────────
say "4 · ALTERNATIVE · GET /PluginFormcreatorForm/$FORM_ID/PluginFormcreatorQuestion  (relation walk)"
split_resp "$(glpi "/PluginFormcreatorForm/$FORM_ID/PluginFormcreatorQuestion?range=0-199&expand_dropdowns=false")"
case "$RESP_HTTP" in
  200|206) rel_count=$(jq 'length' <<<"$RESP_BODY"); ok "relation walk returned $rel_count question(s)" ;;
  204) warn "204 — relation walk returned nothing" ;;
  404) warn "404 — relation endpoint not exposed for this plugin" ;;
  *) warn "HTTP $RESP_HTTP" ;;
esac

# ──────────────────────────────────────────────────────────────────────────────
say "5 · ALTERNATIVE · GET /PluginFormcreatorForm/$FORM_ID/PluginFormcreatorSection  (relation walk)"
split_resp "$(glpi "/PluginFormcreatorForm/$FORM_ID/PluginFormcreatorSection?range=0-49&expand_dropdowns=false")"
case "$RESP_HTTP" in
  200|206) ok "relation walk returned $(jq 'length' <<<"$RESP_BODY") section(s)" ;;
  *) warn "HTTP $RESP_HTTP" ;;
esac

printf '\n'
ok "Done. Interpretation:"
cat <<EOF
  - If step 2 matched 0 sections but the full list is non-empty → the FK column
    has a different name on your Formcreator. The script prints candidate
    "form-related" keys so we can patch the Go filter.
  - If step 3 matched 0 questions but the list is non-empty → same story for
    the question→section FK.
  - If steps 4 & 5 (relation walk) return rows while the manual filter doesn't,
    sshcm should switch to the relation walk instead. Let me know which you see.
EOF
