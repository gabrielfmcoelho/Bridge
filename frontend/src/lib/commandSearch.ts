// Pure matching/ranking for the command palette (components/layout/CommandPalette).
// No runtime imports, so `node --test src/lib/commandSearch.test.ts` runs it as is.

export type SearchKind = "page" | "host" | "dns" | "service" | "project" | "contact";

/** Group order in the palette: pages first. */
export const SEARCH_KINDS: SearchKind[] = ["page", "host", "dns", "service", "project", "contact"];

export interface SearchEntry {
  kind: SearchKind;
  /** Unique within the palette; doubles as the option's DOM id suffix. */
  key: string;
  label: string;
  /** Secondary line (slug, hostname…). */
  sub?: string;
  /** Render label/sub in the mono face (identifiers: slugs, domains, hostnames). */
  mono?: boolean;
  href: string;
  icon?: string;
  /** Everything the query may match; label and sub are always included. */
  terms?: string[];
}

export interface SearchGroup {
  kind: SearchKind;
  items: SearchEntry[];
}

/** Lower-case and strip diacritics: "Serviço" and "servico" match. */
export function normalize(s: string): string {
  return s.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
}

// 0 = some term starts with the query, 1 = a word inside a term does,
// 2 = plain substring, -1 = no match.
function score(entry: SearchEntry, q: string): number {
  let best = -1;
  for (const raw of [entry.label, entry.sub, ...(entry.terms ?? [])]) {
    if (!raw) continue;
    const term = normalize(raw);
    const i = term.indexOf(q);
    if (i < 0) continue;
    const s = i === 0 ? 0 : /[\s\-_./@]/.test(term[i - 1]) ? 1 : 2;
    if (best < 0 || s < best) best = s;
    if (best === 0) break;
  }
  return best;
}

/**
 * Groups matching entries by kind (SEARCH_KINDS order), best matches first,
 * at most `perGroup` each. An empty query lists pages only ("Go to").
 */
export function searchEntries(entries: SearchEntry[], query: string, perGroup = 5, emptyPages = 8): SearchGroup[] {
  const q = normalize(query.trim());
  if (!q) {
    const pages = entries.filter((e) => e.kind === "page").slice(0, emptyPages);
    return pages.length ? [{ kind: "page", items: pages }] : [];
  }
  const scored = entries
    .map((entry, i) => ({ entry, s: score(entry, q), i }))
    .filter((x) => x.s >= 0)
    .sort((a, b) => a.s - b.s || a.i - b.i);
  return SEARCH_KINDS.map((kind) => ({
    kind,
    items: scored.filter((x) => x.entry.kind === kind).slice(0, perGroup).map((x) => x.entry),
  })).filter((g) => g.items.length > 0);
}
