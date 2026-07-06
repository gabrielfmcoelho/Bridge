"use client";

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

  if (secrets.length === 0 && apiDocs.length === 0 && wiki.length === 0) return null;

  return (
    <nav
      aria-label={t("share.indexTitle")}
      className="lg:sticky lg:top-6 max-h-[calc(100vh-3rem)] overflow-y-auto rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]/60 p-3 text-xs"
    >
      <p className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2 px-1">
        {t("share.indexTitle")}
      </p>

      {secrets.length > 0 && (
        <div className="mb-3">
          <p className="text-[11px] font-semibold text-[var(--text-secondary)] px-1 mb-1">
            {t("share.secrets")}
          </p>
          <ul className="space-y-0.5">
            {secrets.map((s, i) => (
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

      {apiDocs.map((doc, i) => {
        const groups = groupOperationsByTag(operationsFromSpec(doc.spec));
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

      {wiki.length > 0 && (
        <div className="mt-3">
          <p className="text-[11px] font-semibold text-[var(--text-secondary)] px-1 mb-1">
            {t("share.wiki")}
          </p>
          {wiki.map((item, i) => (
            <div key={`wiki-${i}`} className="mb-2 last:mb-0">
              <button
                type="button"
                onClick={() => scrollToAnchor(`wiki-${i}`)}
                className="w-full truncate text-left px-1 mb-1 text-[11px] font-semibold text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer"
              >
                {item.title}
              </button>
              {item.kind === "collection" && item.documents.length > 0 && (
                <WikiIndexNodes docs={item.documents} section={i} />
              )}
            </div>
          ))}
        </div>
      )}
    </nav>
  );
}
