"use client";

import { useState } from "react";
import { operationsFromSpec, groupOperationsByTag } from "@/lib/atlas/openapi";
import { useLocale } from "@/contexts/LocaleContext";
import type { BundleApiDocItem, BundleSecretItem, BundleWikiDoc, BundleWikiItem } from "@/lib/types";

// ShareIndexSidebar is the unified endpoint index for the public reveal page: a
// single sticky rail listing the bundle's secrets and, per API, its operations.
// Clicking an entry smooth-scrolls to the matching anchor in the content column
// (secret cards are id="secret-{i}", API sections id="api-{i}"). Operations
// scroll to their API section — deep-scroll into Scalar's DOM is out of scope.

const METHOD_COLORS: Record<string, string> = {
  GET: "text-emerald-400",
  POST: "text-sky-400",
  PUT: "text-amber-400",
  PATCH: "text-amber-400",
  DELETE: "text-red-400",
};

function scrollToAnchor(id: string) {
  if (typeof document === "undefined") return;
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

// WikiIndexNodes recursively lists a collection's document tree as scroll links
// into the content column (each doc is anchored wiki-{section}-doc-{id}).
function WikiIndexNodes({ docs, section }: { docs: BundleWikiDoc[]; section: number }) {
  return (
    <ul className="space-y-0.5">
      {docs.map((doc) => (
        <li key={doc.id}>
          <button
            type="button"
            onClick={() => scrollToAnchor(`wiki-${section}-doc-${doc.id}`)}
            className="w-full truncate text-left px-2 py-0.5 rounded-[var(--radius-sm)] text-[var(--text-muted)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-primary)] cursor-pointer"
          >
            {doc.emoji ? `${doc.emoji} ` : ""}
            {doc.title}
          </button>
          {doc.children && doc.children.length > 0 && (
            <div className="pl-2 border-l border-[var(--border-subtle)] ml-2">
              <WikiIndexNodes docs={doc.children} section={section} />
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

// filterWikiDocs prunes a doc tree to nodes that match (or have a descendant
// that matches) the query, keeping ancestors so the path stays navigable.
function filterWikiDocs(docs: BundleWikiDoc[], q: string): BundleWikiDoc[] {
  if (!q) return docs;
  const out: BundleWikiDoc[] = [];
  for (const d of docs) {
    const kids = filterWikiDocs(d.children ?? [], q);
    if (d.title.toLowerCase().includes(q) || kids.length > 0) {
      out.push({ ...d, children: kids });
    }
  }
  return out;
}

export default function ShareIndexSidebar({
  secrets,
  apiDocs,
  wiki = [],
}: {
  secrets: BundleSecretItem[];
  apiDocs: BundleApiDocItem[];
  wiki?: BundleWikiItem[];
}) {
  const { t } = useLocale();
  const [query, setQuery] = useState("");

  if (secrets.length === 0 && apiDocs.length === 0 && wiki.length === 0) return null;

  // Search filters the index in place: operations by method/path/summary/tag,
  // secrets by name, wiki docs by title. Clicking a result still scrolls to the
  // matching content section.
  const q = query.trim().toLowerCase();
  const match = (s: string | undefined | null) => !q || (s ?? "").toLowerCase().includes(q);

  const secretViews = secrets.map((s, i) => ({ s, i })).filter(({ s }) => match(s.name));

  const apiViews = apiDocs
    .map((doc, i) => {
      const nameHit = match(doc.name);
      const groups = groupOperationsByTag(operationsFromSpec(doc.spec))
        .map((g) => ({
          tag: g.tag,
          operations: nameHit
            ? g.operations
            : g.operations.filter(
                (op) => match(op.method) || match(op.path) || match(op.summary) || match(g.tag),
              ),
        }))
        .filter((g) => g.operations.length > 0);
      return { doc, i, groups, keep: !q || nameHit || groups.length > 0 };
    })
    .filter((v) => v.keep);

  const wikiViews = wiki
    .map((item, i) => {
      const docs = item.kind === "collection" ? filterWikiDocs(item.documents, q) : [];
      const keep = !q || match(item.title) || (item.kind === "collection" && docs.length > 0);
      return { item, i, docs, keep };
    })
    .filter((v) => v.keep);

  const noResults =
    q.length > 0 && secretViews.length === 0 && apiViews.length === 0 && wikiViews.length === 0;

  return (
    <nav
      aria-label={t("share.indexTitle")}
      className="lg:sticky lg:top-6 max-h-[calc(100vh-3rem)] overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]/60 p-3 text-xs"
    >
      <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2 px-1">
        {t("share.indexTitle")}
      </p>

      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder={t("share.searchEndpoints")}
        aria-label={t("share.searchEndpoints")}
        className="w-full mb-3 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-1 text-xs text-[var(--text-primary)] placeholder:text-[var(--text-faint)] focus:border-[var(--accent)] focus:outline-none"
      />

      {noResults && (
        <p className="px-1 text-[11px] text-[var(--text-muted)]">{t("share.searchNoResults")}</p>
      )}

      {secretViews.length > 0 && (
        <div className="mb-3">
          <p className="text-[11px] font-semibold text-[var(--text-secondary)] px-1 mb-1">
            {t("share.secrets")}
          </p>
          <ul className="space-y-0.5">
            {secretViews.map(({ s, i }) => (
              <li key={`secret-${i}`}>
                <button
                  type="button"
                  onClick={() => scrollToAnchor(`secret-${i}`)}
                  className="w-full truncate text-left px-2 py-1 rounded-[var(--radius-sm)] text-[var(--text-secondary)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-primary)] cursor-pointer"
                >
                  {s.name}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {apiViews.map(({ doc, i, groups }) => {
        // Only show tag headings when at least one real tag exists; an all-
        // untagged API renders flat (no lone "Other" label).
        const showTags = groups.some((g) => g.tag !== null);
        return (
          <div key={`api-${i}`} className="mb-3 last:mb-0">
            <button
              type="button"
              onClick={() => scrollToAnchor(`api-${i}`)}
              className="w-full truncate text-left px-1 mb-1 text-[11px] font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
            >
              {doc.name}
            </button>
            {groups.map((group, g) => (
              <div key={`api-${i}-tag-${g}`} className="mb-1.5 last:mb-0">
                {showTags && (
                  <p className="px-2 pt-0.5 text-[9px] font-semibold uppercase tracking-wide text-[var(--text-faint)]">
                    {group.tag ?? t("share.indexOther")}
                  </p>
                )}
                <ul className="space-y-0.5">
                  {group.operations.map((op, j) => (
                    <li key={`api-${i}-tag-${g}-op-${j}`}>
                      <button
                        type="button"
                        onClick={() => scrollToAnchor(`api-${i}`)}
                        title={`${op.method} ${op.path}`}
                        className="w-full flex items-start gap-1.5 text-left px-2 py-0.5 rounded-[var(--radius-sm)] text-[var(--text-muted)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-primary)] cursor-pointer"
                      >
                        <span
                          className={`shrink-0 font-bold text-[9px] w-10 mt-0.5 ${METHOD_COLORS[op.method] || "text-[var(--text-muted)]"}`}
                        >
                          {op.method}
                        </span>
                        <span className="whitespace-normal break-words leading-snug">
                          {op.summary || op.path}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        );
      })}

      {wikiViews.length > 0 && (
        <div className="mt-3">
          <p className="text-[11px] font-semibold text-[var(--text-secondary)] px-1 mb-1">
            {t("share.wiki")}
          </p>
          {wikiViews.map(({ item, i, docs }) => (
            <div key={`wiki-${i}`} className="mb-2 last:mb-0">
              <button
                type="button"
                onClick={() => scrollToAnchor(`wiki-${i}`)}
                className="w-full truncate text-left px-1 mb-1 text-[11px] font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {item.title}
              </button>
              {item.kind === "collection" && docs.length > 0 && (
                <WikiIndexNodes docs={docs} section={i} />
              )}
            </div>
          ))}
        </div>
      )}
    </nav>
  );
}
