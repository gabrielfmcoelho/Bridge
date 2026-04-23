#!/usr/bin/env bash
# test-glpi-forms.sh — diagnose what the Formcreator REST surface returns for
# the profile stored in scripts/.env. Answers the question "why is sshcm's
# /chamados/forms showing 'Nenhum formulário visível' when I can see forms in
# GLPI's web UI?"
#
# Reuses the .env loading + session bootstrap from test-glpi.sh, then walks
# every candidate enumeration path and prints what each returns.
#
# Usage:
#   ./scripts/test-glpi.sh          # must succeed first (sets up .env)
#   ./scripts/test-glpi-forms.sh
#   ./scripts/test-glpi-forms.sh 5  # also probe the bundle for form id=5

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

load_env_file() {
  local f="$1"
  [[ -f "$f" ]] || return 1
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

base="${GLPI_URL%/}"
[[ "$base" != */apirest.php ]] && base="$base/apirest.php"

probe_form_id="${1:-}"

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
  "${app_header[@]}" \
  "$base/initSession")
init_http=$(tail -n1 <<<"$init_resp")
init_body=$(head -n-1 <<<"$init_resp")
[[ "$init_http" == "200" ]] || die "initSession HTTP $init_http: $init_body"
session=$(jq -r '.session_token // empty' <<<"$init_body")
[[ -n "$session" ]] || die "no session_token in response"
ok "session acquired"

glpi() {
  local path="$1"; shift
  curl -sS -g -w '\n%{http_code}' \
    -H "Session-Token: $session" \
    -H 'Accept: application/json' \
    "${app_header[@]}" "$@" \
    "$base$path"
}

split_resp() {
  local raw="$1"
  RESP_HTTP="${raw##*$'\n'}"
  RESP_BODY="${raw%$'\n'*}"
}

# ──────────────────────────────────────────────────────────────────────────────
say "1 · GET /PluginFormcreatorForm  (plain list — no filter)"
split_resp "$(glpi '/PluginFormcreatorForm?range=0-49')"
case "$RESP_HTTP" in
  200|206)
    cnt=$(jq 'length // 0' <<<"$RESP_BODY" 2>/dev/null || echo 0)
    ok "plain GET returned $cnt form(s)"
    jq -r '.[]? | "   #\(.id)  is_active=\(.is_active)  lang=\(.language // "-")  name=\(.name)"' <<<"$RESP_BODY" 2>/dev/null | head -20
    ;;
  204) warn "204 — 0 forms visible to this profile via plain GET" ;;
  404) warn "404 — Formcreator plugin not installed or endpoint gone" ;;
  *)   warn "HTTP $RESP_HTTP — $(head -c 200 <<<"$RESP_BODY")" ;;
esac

# ──────────────────────────────────────────────────────────────────────────────
say "2 · GET /search/PluginFormcreatorForm  (no criteria)"
split_resp "$(glpi '/search/PluginFormcreatorForm?range=0-49&forcedisplay[0]=2&forcedisplay[1]=1')"
if [[ "$RESP_HTTP" == "200" || "$RESP_HTTP" == "206" ]]; then
  total=$(jq -r '.totalcount // 0' <<<"$RESP_BODY")
  rows=$(jq -r '.data | length // 0' <<<"$RESP_BODY")
  ok "search totalcount=$total  rows=$rows"
elif [[ "$RESP_HTTP" == "204" ]]; then
  warn "204 — search with no criteria returned nothing"
else
  warn "HTTP $RESP_HTTP — $(head -c 200 <<<"$RESP_BODY")"
fi

# ──────────────────────────────────────────────────────────────────────────────
say "3 · GET /search/PluginFormcreatorForm?criteria[is_active=1]  (field 8 — sshcm's old call)"
q='criteria[0][field]=8&criteria[0][searchtype]=equals&criteria[0][value]=1'
q+='&forcedisplay[0]=2&forcedisplay[1]=1&range=0-49'
split_resp "$(glpi "/search/PluginFormcreatorForm?$q")"
if [[ "$RESP_HTTP" == "200" || "$RESP_HTTP" == "206" ]]; then
  total=$(jq -r '.totalcount // 0' <<<"$RESP_BODY")
  ok "with field=8 criteria → totalcount=$total  $([ "$total" = 0 ] && echo "(THIS IS WHY THE UI WAS EMPTY — field id 8 doesn't mean is_active on this instance)")"
elif [[ "$RESP_HTTP" == "204" ]]; then
  warn "204 — field=8 criteria matches 0 rows"
else
  warn "HTTP $RESP_HTTP"
fi

# ──────────────────────────────────────────────────────────────────────────────
say "4 · GET /listSearchOptions/PluginFormcreatorForm  (find the correct is_active field id)"
split_resp "$(glpi '/listSearchOptions/PluginFormcreatorForm')"
if [[ "$RESP_HTTP" == "200" ]]; then
  # Object keyed by numeric field id. Dump any entry whose name/table relates to "active".
  matches=$(jq -r 'to_entries[]? | select((.value | tostring | ascii_downcase) | contains("active")) | "\(.key)\t\(.value.name // .value.table // "?")"' <<<"$RESP_BODY" 2>/dev/null || true)
  if [[ -n "$matches" ]]; then
    echo "$matches" | sed 's/^/   field /'
  else
    warn "no fields matched 'active' — try dumping the full option list:"
    echo "   curl -sS -H 'Session-Token: $session' ${app_header[*]} '$base/listSearchOptions/PluginFormcreatorForm' | jq"
  fi
else
  warn "HTTP $RESP_HTTP on /listSearchOptions — your Formcreator may not expose it for this itemtype"
fi

# ──────────────────────────────────────────────────────────────────────────────
if [[ -n "$probe_form_id" ]]; then
  say "5 · GET /PluginFormcreatorForm/$probe_form_id  (single-form fetch)"
  split_resp "$(glpi "/PluginFormcreatorForm/$probe_form_id")"
  case "$RESP_HTTP" in
    200) ok "form visible"; jq -r '"   name=\(.name)  is_active=\(.is_active)  entities_id=\(.entities_id)  lang=\(.language // "-")"' <<<"$RESP_BODY" ;;
    401|403) warn "$RESP_HTTP — profile can't read form $probe_form_id (permission)" ;;
    404) warn "404 — form $probe_form_id doesn't exist or is hidden" ;;
    *) warn "HTTP $RESP_HTTP"; echo "$RESP_BODY" | head -c 400 ;;
  esac

  say "5b · GET /PluginFormcreatorForm/$probe_form_id/PluginFormcreatorQuestion"
  split_resp "$(glpi "/PluginFormcreatorForm/$probe_form_id/PluginFormcreatorQuestion?range=0-49")"
  case "$RESP_HTTP" in
    200|206) ok "questions returned: $(jq 'length // 0' <<<"$RESP_BODY")" ;;
    204) warn "no questions (or endpoint returns 204)" ;;
    *)   warn "HTTP $RESP_HTTP" ;;
  esac
fi

# ──────────────────────────────────────────────────────────────────────────────
say "cleanup · killSession"
glpi /killSession >/dev/null || warn "killSession failed (non-fatal)"

printf '\n'
ok "Probe complete."
cat <<EOF

Interpretation:
  - Step 1 (plain GET) is now sshcm's default path after the fix. If it returns
    your forms, /chamados/forms will show them.
  - Step 3 shows whether field id 8 is the right search option for is_active on
    YOUR Formcreator version. If totalcount is 0 while step 1 shows forms, the
    old code was broken on this instance — hence the "Nenhum formulário visível".
  - Step 4 prints the field ids whose name contains "active"; use that id if you
    ever re-enable the server-side filter.
  - Step 5 (if you passed a form id) confirms whether the profile can also
    fetch the full bundle — the next step sshcm takes when you click a card.

Next:
  - Restart the Go backend so the updated handler runs.
  - Reload /chamados/forms; you should see the forms.
EOF
