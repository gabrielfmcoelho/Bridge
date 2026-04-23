#!/usr/bin/env bash
# test-glpi.sh — probe a GLPI instance with a user token to verify the sshcm
# integration will work end-to-end.
#
# Credentials are read from a .env file (or exported env vars). Resolution order:
#   1. $ENV_FILE if set
#   2. scripts/.env (next to this script)
#   3. repo-root/.env
#   4. already-exported shell env
#
# Required keys:
#   GLPI_URL=https://glpi.example.org
#   GLPI_USER_TOKEN=<personal API token from Preferences>
# Optional:
#   GLPI_APP_TOKEN=<only if your instance requires it>
#
# Exit code: 0 on success, non-zero on any failure.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"

# Load a .env file in the shell's current process. Parses KEY=VALUE lines,
# ignores comments and blank lines, and strips surrounding single/double quotes
# from the value. Exports the vars so child curl invocations inherit them.
load_env_file() {
  local f="$1"
  [[ -f "$f" ]] || return 1
  # Use `set -a` so every assignment is exported automatically.
  set -a
  # shellcheck disable=SC1090
  source <(
    grep -vE '^\s*(#|$)' "$f" | sed -E 's/^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/\1=\2/'
  )
  set +a
}

env_file=""
if [[ -n "${ENV_FILE:-}" && -f "$ENV_FILE" ]]; then
  env_file="$ENV_FILE"
elif [[ -f "$script_dir/.env" ]]; then
  env_file="$script_dir/.env"
elif [[ -f "$repo_root/.env" ]]; then
  env_file="$repo_root/.env"
fi

if [[ -n "$env_file" ]]; then
  load_env_file "$env_file"
  printf '  \033[2mloaded %s\033[0m\n' "$env_file"
fi

: "${GLPI_URL:?set GLPI_URL in .env or export it (e.g. https://glpi.example.org)}"
: "${GLPI_USER_TOKEN:?set GLPI_USER_TOKEN in .env (personal API token from Preferences)}"
GLPI_APP_TOKEN="${GLPI_APP_TOKEN:-}"

base="${GLPI_URL%/}"
if [[ "$base" != */apirest.php ]]; then
  base="$base/apirest.php"
fi

say() { printf '\n\033[1;34m→ %s\033[0m\n' "$*"; }
ok()  { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
warn(){ printf '  \033[1;33m!\033[0m %s\n' "$*"; }
die() { printf '\n\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null || die "missing dependency: $1"; }
need curl
need jq

app_header=()
if [[ -n "$GLPI_APP_TOKEN" ]]; then
  app_header=(-H "App-Token: $GLPI_APP_TOKEN")
fi

# ──────────────────────────────────────────────────────────────────────────────
say "1/4 · initSession  (exchanges user token → session token)"
init_body=$(curl -sS -w '\n%{http_code}' \
  -H "Authorization: user_token $GLPI_USER_TOKEN" \
  -H 'Accept: application/json' \
  "${app_header[@]}" \
  "$base/initSession")
http=$(tail -n1 <<<"$init_body")
payload=$(head -n-1 <<<"$init_body")

if [[ "$http" != "200" ]]; then
  echo "$payload"
  if [[ "$http" == "400" ]] && grep -q 'app_token' <<<"$payload"; then
    die "HTTP 400 — this GLPI requires an App-Token. Set GLPI_APP_TOKEN and retry."
  fi
  die "HTTP $http from initSession. Check URL + token."
fi
session=$(jq -r '.session_token // empty' <<<"$payload")
[[ -n "$session" ]] || die "no session_token in response: $payload"
ok "session_token acquired"

# Helper that always attaches Session-Token (+ App-Token if set).
# -g disables curl's URL globbing so the square brackets in GLPI's search
# params (e.g. criteria[0][field]=12) are sent verbatim instead of being
# parsed as glob ranges.
glpi() {
  local path="$1"; shift
  curl -sS -g -w '\n%{http_code}' \
    -H "Session-Token: $session" \
    -H 'Accept: application/json' \
    "${app_header[@]}" \
    "$@" \
    "$base$path"
}

# split_resp reads the combined body+\n+HTTP code stdout of `glpi` and sets two
# globals: RESP_BODY, RESP_HTTP. Called without a pipe so assignments stick.
split_resp() {
  local raw="$1"
  RESP_HTTP="${raw##*$'\n'}"
  RESP_BODY="${raw%$'\n'*}"
}

# ──────────────────────────────────────────────────────────────────────────────
say "2/4 · getMyProfiles  (which profiles can this token act as?)"
split_resp "$(glpi /getMyProfiles)"
[[ "$RESP_HTTP" == "200" ]] || { echo "$RESP_BODY"; die "HTTP $RESP_HTTP"; }
profiles=$(jq -r '.myprofiles[]?.name' <<<"$RESP_BODY" | paste -sd, -)
ok "profiles: ${profiles:-<none>}"

# Force recursive visibility across every entity the token can see. Without
# this, GLPI scopes searches to the user's current "active entity" — which is
# often just the root and excludes sub-entities, explaining a 62-vs-13 gap
# between the dashboard and the API totalcount.
say "2b/4 · changeActiveEntities  (expand scope to all reachable entities)"
split_resp "$(glpi /changeActiveEntities -X POST -H 'Content-Type: application/json' \
  --data '{"entities_id":"all","is_recursive":true}')"
if [[ "$RESP_HTTP" == "200" ]]; then
  ok "scope now covers all entities (recursive)"
else
  warn "changeActiveEntities HTTP $RESP_HTTP — continuing with default scope"
fi

# Count per status so we can see what the API actually sees. Matches the buckets
# on GLPI's Central dashboard (1=Novo, 2=Atribuído, 3=Planejado, 4=Pendente,
# 5=Solucionado/Resolvido, 6=Fechado).
say "2c/4 · count by status  (should match the dashboard cards)"
for code in 1 2 3 4 5 6; do
  label=$(case "$code" in 1) echo Novo;; 2) echo Atribuído;; 3) echo Planejado;; 4) echo Pendente;; 5) echo Solucionado;; 6) echo Fechado;; esac)
  cq="criteria[0][field]=12&criteria[0][searchtype]=equals&criteria[0][value]=$code&range=0-0"
  split_resp "$(glpi "/search/Ticket?$cq")"
  if [[ "$RESP_HTTP" == "200" || "$RESP_HTTP" == "206" ]]; then
    total=$(jq -r '.totalcount // 0' <<<"$RESP_BODY")
  elif [[ "$RESP_HTTP" == "204" ]]; then
    total=0
  else
    total="?"
  fi
  printf '  %-14s %s\n' "$label" "$total"
done

# ──────────────────────────────────────────────────────────────────────────────
say "3/4 · search/Ticket  (open tickets — the bulk-import endpoint sshcm uses)"
# forcedisplay: 2=id, 1=name, 12=status, 3=priority, 15=date, 80=entity
q='criteria[0][field]=12&criteria[0][searchtype]=notequals&criteria[0][value]=6&criteria[0][link]=AND'
q+='&forcedisplay[0]=2&forcedisplay[1]=1&forcedisplay[2]=12&forcedisplay[3]=3&forcedisplay[4]=15&forcedisplay[5]=80'
q+='&range=0-9'
split_resp "$(glpi "/search/Ticket?$q")"
if [[ "$RESP_HTTP" == "200" || "$RESP_HTTP" == "206" ]]; then
  total=$(jq -r '.totalcount // 0' <<<"$RESP_BODY")
  shown=$(jq -r '.data | length // 0' <<<"$RESP_BODY")
  ok "open tickets: totalcount=$total  (showing $shown)"
  jq -r '.data[]? | "   #\(."2")  \(."1")  (entity=\(."80"))"' <<<"$RESP_BODY" | head -10
elif [[ "$RESP_HTTP" == "204" ]]; then
  warn "search returned HTTP 204 — 0 tickets. Still counts as success."
else
  echo "$RESP_BODY"
  die "search/Ticket returned HTTP $RESP_HTTP"
fi

# ──────────────────────────────────────────────────────────────────────────────
say "3b/4 · PluginFormcreatorForm  (custom forms the profile can see)"
split_resp "$(glpi '/PluginFormcreatorForm?range=0-9')"
if [[ "$RESP_HTTP" == "200" || "$RESP_HTTP" == "206" ]]; then
  cnt=$(jq -r '. | length // 0' <<<"$RESP_BODY" 2>/dev/null || echo "?")
  ok "forms visible: $cnt"
  jq -r '.[]? | "   #\(.id)  \(.name) (active=\(.is_active))"' <<<"$RESP_BODY" 2>/dev/null | head -10
elif [[ "$RESP_HTTP" == "204" ]]; then
  warn "0 forms visible to this profile"
elif [[ "$RESP_HTTP" == "404" ]]; then
  warn "Formcreator plugin endpoint not available (404) — either the plugin isn't installed or the profile can't see it"
else
  warn "PluginFormcreatorForm returned HTTP $RESP_HTTP — Formcreator features won't work for this profile"
fi

# ──────────────────────────────────────────────────────────────────────────────
say "4/4 · killSession  (cleanup)"
glpi /killSession >/dev/null || warn "killSession failed (non-fatal)"
ok "done."

cat <<EOF

Next:
  - In sshcm Settings → Integrations → GLPI: leave App-Token blank if GLPI_APP_TOKEN was unset during this run; the integration will work without it.
  - Add a profile under the GLPI card, paste the same GLPI_USER_TOKEN, and click Test.
  - Open /chamados → switch to "All tickets (profile)" → select the profile to see every ticket listed above.
EOF
