#!/usr/bin/env bash
# Reprints the grep tallies quoted on /design-system and in src/DESIGN_SYSTEM.md.
# Run from frontend/. Scope: src/**/*.tsx minus the catalog page itself.
set -euo pipefail
cd "$(dirname "$0")/.."
G() { grep -rhoE --include='*.tsx' --exclude-dir=design-system "$1" src | wc -l | tr -d ' '; }
GL() { grep -rlE --include='*.tsx' --exclude-dir=design-system "$1" src | wc -l | tr -d ' '; }
printf '%-52s %s\n' "raw palette classes (text|bg|border-<hue>-N)" "$(G '\b(text|bg|border|ring|from|to|via)-(emerald|green|red|rose|amber|yellow|orange|cyan|sky|blue|indigo|violet|purple|gray|slate|zinc|neutral|stone|teal|lime|pink|fuchsia)-[0-9]{2,3}(/[0-9]+)?\b')"
printf '%-52s %s\n' "semantic token uses var(--success|warning|danger|info)" "$(G 'var\(--(success|warning|danger|info)\)')"
printf '%-52s %s\n' "hex literals in tsx" "$(G '#[0-9a-fA-F]{6}\b')"
printf '%-52s %s\n' "arbitrary font sizes text-[Npx]" "$(G 'text-\[[0-9]+px\]')"
printf '%-52s %s\n' "inline fontFamily styles" "$(G 'fontFamily: "var\(--font-')"
printf '%-52s %s\n' "inline <svg>" "$(G '<svg\b')"
printf '%-52s %s\n' "Icon component uses" "$(G '<Icon\b')"
printf '%-52s %s\n' "transition-all" "$(G '\btransition-all\b')"
printf '%-52s %s\n' "h2/h3 with inline text-xs..uppercase tracking-wider" "$(G '<h[23] className="text-xs font-semibold[^"]*uppercase tracking-wider')"
printf '%-52s %s\n' "hand-written <label> (not ui/)" "$(grep -rhoE --include='*.tsx' --exclude-dir=design-system --exclude-dir=ui '<label\b' src | wc -l | tr -d ' ')"
printf '%-52s %s\n' "raw <button> tags (not ui/)" "$(grep -rhoE --include='*.tsx' --exclude-dir=design-system --exclude-dir=ui '<button\b' src | wc -l | tr -d ' ')"
printf '%-52s %s\n' "focus:outline-none" "$(G 'focus:outline-none')"
printf '%-52s %s\n' "navigator.clipboard.writeText" "$(G 'navigator\.clipboard\.writeText')"
printf '%-52s %s\n' "em-dashes in tsx" "$(G '—')"
printf '%-52s %s\n' "files importing components/ui" "$(GL '@/components/ui/')"
