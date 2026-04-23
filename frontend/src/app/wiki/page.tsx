"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { outlineAPI, type OutlineDocumentNode } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import PageShell from "@/components/layout/PageShell";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import CreateDocumentModal from "@/components/wiki/CreateDocumentModal";
import WikiSearchBar from "@/components/wiki/WikiSearchBar";
import WikiTree from "@/components/wiki/WikiTree";
import WikiDocumentViewer from "@/components/wiki/WikiDocumentViewer";

type ViewMode = "docs" | "search";

function collectIds(nodes: OutlineDocumentNode[], into: Set<string>) {
  for (const n of nodes) {
    into.add(n.id);
    if (n.children) collectIds(n.children, into);
  }
}

// Renders an Outline collection's icon. Outline's `icon` may be:
//   - a single emoji char ("📂"), which we render as text;
//   - an icon name ("academic-cap") we don't try to map → fall back to a folder
//     SVG tinted with the collection's color;
//   - empty → folder SVG with the color (or a neutral color).
function CollectionIconPip({ icon, color, size = 14 }: { icon?: string; color?: string; size?: number }) {
  const tint = color || "var(--text-faint)";
  // Treat anything ≤ 4 codepoints as an emoji-style glyph.
  if (icon && [...icon].length <= 4) {
    return (
      <span
        aria-hidden
        className="inline-flex items-center justify-center shrink-0"
        style={{ width: size, height: size, fontSize: Math.round(size * 0.95), lineHeight: 1 }}
      >
        {icon}
      </span>
    );
  }
  return (
    <svg
      aria-hidden
      width={size}
      height={size}
      className="shrink-0"
      viewBox="0 0 24 24"
      fill="none"
      stroke={tint}
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
    </svg>
  );
}

export default function WikiPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const { locale } = useLocale();
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const selectedDocId = searchParams.get("doc");
  // Default view is search — when a user lands on /wiki with no doc in the URL
  // they see a Google-style search surface instead of an empty-document prompt.
  const [viewMode, setViewMode] = useState<ViewMode>(selectedDocId ? "docs" : "search");
  const [searchQuery, setSearchQuery] = useState("");
  const [createForCollection, setCreateForCollection] = useState<string | null>(null);
  const [collapsedCollections, setCollapsedCollections] = useState<Set<string>>(new Set());
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const { data: tree, isLoading: treeLoading } = useQuery({
    queryKey: ["wiki-tree"],
    queryFn: outlineAPI.commonWikiTree,
    retry: false,
  });

  const { data: doc, isLoading: docLoading, error: docError } = useQuery({
    queryKey: ["wiki-doc", selectedDocId],
    queryFn: () => outlineAPI.getDocument(selectedDocId!),
    enabled: !!selectedDocId,
    retry: false,
  });

  const { data: searchData, isFetching: searchFetching } = useQuery({
    queryKey: ["common-wiki-search", searchQuery],
    queryFn: () => outlineAPI.searchCommonWiki(searchQuery),
    enabled: !!searchQuery.trim(),
    retry: false,
  });

  const createMutation = useMutation({
    mutationFn: ({ title, collectionID }: { title: string; collectionID: string }) =>
      outlineAPI.createCommonDocument(title, collectionID),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["wiki-tree"] });
      if (res.id) selectDoc(res.id);
    },
  });

  useEffect(() => {
    if (!tree?.sections?.length) return;
    setExpandedNodes((prev) => {
      if (prev.size > 0) return prev;
      const ids = new Set<string>();
      for (const s of tree.sections) collectIds(s.nodes ?? [], ids);
      return ids;
    });
  }, [tree]);

  const selectDoc = (id: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("doc", id);
    router.push(`/wiki?${params.toString()}`);
    setViewMode("docs");
    setMobileNavOpen(false);
  };

  const enterSearchMode = () => {
    setViewMode("search");
    setMobileNavOpen(false);
  };

  const toggleCollection = (id: string) => {
    setCollapsedCollections((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleNode = (id: string) => {
    setExpandedNodes((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const isSearching = viewMode === "search" && searchQuery.trim().length > 0;
  const searchHits = searchData?.results ?? [];

  // Only docs inside configured common collections are viewable in-app — others
  // still surface as search hits but point out to Outline (sshcm's tree query
  // never saw them, so no tree entry exists and /api/wiki/documents would 403).
  const allowedCollectionIds = useMemo(
    () => new Set((tree?.sections ?? []).map((s) => s.collection_id)),
    [tree]
  );

  // ── Left-nav: Search entry + Collections tree ────────
  const searchNavButton = (
    <button
      type="button"
      onClick={enterSearchMode}
      className={`w-full flex items-center gap-2 rounded-[var(--radius-md)] px-2.5 py-2 text-sm transition-colors ${
        viewMode === "search"
          ? "bg-[var(--accent-muted)] text-[var(--accent)]"
          : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
      }`}
    >
      <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M10 18a8 8 0 100-16 8 8 0 000 16z" />
      </svg>
      Search
    </button>
  );

  const nav = (
    <div className="space-y-3">
      {treeLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-6 w-full rounded-[var(--radius-sm)]" />
          ))}
        </div>
      ) : !tree?.enabled ? (
        <p className="text-xs text-[var(--text-faint)]">Outline integration disabled.</p>
      ) : !tree?.configured ? (
        <p className="text-xs text-[var(--text-faint)]">Outline not configured.</p>
      ) : (tree.sections?.length ?? 0) === 0 ? (
        <p className="text-xs text-[var(--text-faint)]">
          No common collections configured. Ask an admin to pick some in Settings → Integrations → Outline.
        </p>
      ) : (
        tree.sections.map((section) => {
          const collapsed = collapsedCollections.has(section.collection_id);
          return (
            <div key={section.collection_id}>
              <div className="flex items-center gap-1 mb-1">
                <button
                  type="button"
                  onClick={() => toggleCollection(section.collection_id)}
                  className="flex-1 min-w-0 flex items-center gap-1.5 text-left"
                >
                  <svg
                    className={`w-3 h-3 text-[var(--text-faint)] shrink-0 transition-transform ${collapsed ? "" : "rotate-90"}`}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                  </svg>
                  <CollectionIconPip icon={section.collection?.icon} color={section.collection?.color} size={14} />
                  <span className="text-xs font-semibold text-[var(--text-secondary)] uppercase tracking-wider truncate">
                    {section.collection?.name ?? section.collection_id}
                  </span>
                </button>
                {canEdit && section.collection && (
                  <button
                    type="button"
                    onClick={() => setCreateForCollection(section.collection_id)}
                    className="text-[var(--text-faint)] hover:text-[var(--accent)] text-xs px-1"
                    title="New page in this collection"
                  >
                    +
                  </button>
                )}
              </div>
              {!collapsed && (
                <>
                  {section.warning ? (
                    <p className="text-[11px] text-amber-400 bg-amber-500/10 border border-amber-500/30 rounded-[var(--radius-sm)] px-2 py-1">
                      {section.warning}
                    </p>
                  ) : (
                    <WikiTree
                      nodes={section.nodes ?? []}
                      selectedId={viewMode === "docs" ? selectedDocId : null}
                      expandedIds={expandedNodes}
                      onSelect={selectDoc}
                      onToggle={toggleNode}
                    />
                  )}
                </>
              )}
            </div>
          );
        })
      )}
    </div>
  );

  // ── Right pane body ─────────────────────────────────────────────────────
  // (The "Wiki" page title lives in the global app header now — see Header.tsx.)

  const paneContent = (() => {
    if (viewMode === "search") {
      // Single container that slides from vertically-centered (empty query) to
      // top-anchored (with results) via an animated padding-top. Horizontal
      // centering stays constant through the transition.
      return (
        <div className="flex-1 flex flex-col items-center">
          <div
            className={`w-full max-w-[560px] transition-[padding] duration-500 ease-out ${
              isSearching ? "pt-6" : "pt-[28vh]"
            }`}
          >
            {!isSearching && (
              <div className="text-center mb-6">
                <h2
                  className="text-3xl font-semibold text-[var(--text-primary)]"
                  style={{ fontFamily: "var(--font-display)" }}
                >
                  Search the wiki
                </h2>
                <p className="mt-2 text-sm text-[var(--text-muted)]">
                  Looks across every collection the integration can see.
                </p>
              </div>
            )}
            <WikiSearchBar
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder="Type to search…"
            />
            {isSearching && (
              <div className="mt-5">
                {searchFetching && searchHits.length === 0 ? (
                  <p className="text-sm text-[var(--text-muted)] text-center py-6 animate-pulse">Searching…</p>
                ) : searchHits.length === 0 ? (
                  <p className="text-sm text-[var(--text-muted)] text-center py-6">
                    No results for “{searchQuery}”.
                  </p>
                ) : (
                  <ul className="space-y-2">
                    {searchHits.map((hit) => {
                      // Default to allowed when the backend hasn't labelled the
                      // hit with a collection id (older deployments, or Outline
                      // hits that omit it). The /api/wiki/documents/{id} handler
                      // still enforces scope server-side, so this is safe.
                      const canOpen =
                        !hit.collection_id || allowedCollectionIds.has(hit.collection_id);
                      const rowClass =
                        "block w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2.5 transition-colors";
                      const inner = (
                        <>
                          <div className="flex items-start justify-between gap-3">
                            <p className="text-sm font-medium text-[var(--text-primary)] flex-1 min-w-0 truncate">
                              {hit.title || "Untitled"}
                            </p>
                            {!canOpen && (
                              <span
                                title="Outside the configured common collections — opens in Outline only"
                                className="shrink-0 text-[var(--text-faint)]"
                              >
                                <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                  <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V7a4.5 4.5 0 10-9 0v3.5M6 10.5h12a1.5 1.5 0 011.5 1.5v7a1.5 1.5 0 01-1.5 1.5H6A1.5 1.5 0 014.5 19v-7A1.5 1.5 0 016 10.5z" />
                                </svg>
                              </span>
                            )}
                          </div>
                          {hit.context && (
                            <p className="text-[11px] text-[var(--text-muted)] mt-0.5 line-clamp-2">{hit.context}</p>
                          )}
                          <p className="text-[10px] text-[var(--text-faint)] mt-1" style={{ fontFamily: "var(--font-mono)" }}>
                            {getTimeAgo(hit.updated_at, locale)}
                          </p>
                        </>
                      );
                      return (
                        <li key={hit.id}>
                          {canOpen ? (
                            <button
                              type="button"
                              onClick={() => selectDoc(hit.id)}
                              className={`${rowClass} hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)]`}
                            >
                              {inner}
                            </button>
                          ) : (
                            <div
                              className={`${rowClass} opacity-60 cursor-not-allowed`}
                              aria-disabled
                            >
                              {inner}
                            </div>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            )}
          </div>
        </div>
      );
    }

    if (selectedDocId) {
      return (
        <div className="max-w-[780px] mx-auto w-full">
          {doc && (
            <div className="mb-3">
              <h2
                className="text-3xl font-bold text-[var(--text-primary)] leading-tight flex items-start gap-2"
                style={{ fontFamily: "var(--font-display)" }}
              >
                {doc.emoji && <span className="shrink-0">{doc.emoji}</span>}
                <span>{doc.title || "Untitled"}</span>
              </h2>
              <p className="mt-1.5 text-xs text-[var(--text-muted)]">
                Updated {getTimeAgo(doc.updated_at, locale)}
                {doc.updated_by ? ` by ${doc.updated_by}` : ""}
              </p>
            </div>
          )}
          <WikiDocumentViewer doc={doc} isLoading={docLoading} error={docError as Error | null} />
        </div>
      );
    }
    return (
      <EmptyState
        icon="folder"
        title="Pick a document"
        description="Choose a page from the left nav, or use Search to query the workspace."
      />
    );
  })();

  return (
    <PageShell fullBleed>
      <div className="flex h-full min-h-0">
        {/* LEFT NAV — flush left, full height; collapsible on desktop. */}
        {sidebarCollapsed ? (
          <aside className="hidden md:flex w-12 shrink-0 border-r border-[var(--border-subtle)] bg-[var(--bg-surface)] flex-col items-center py-2 gap-1">
            <button
              type="button"
              onClick={() => setSidebarCollapsed(false)}
              title="Expand nav"
              className="w-8 h-8 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </button>
            <div className="w-6 border-t border-[var(--border-subtle)] my-1" />
            <button
              type="button"
              onClick={enterSearchMode}
              title="Search"
              className={`w-8 h-8 inline-flex items-center justify-center rounded-[var(--radius-sm)] transition-colors ${
                viewMode === "search"
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M10 18a8 8 0 100-16 8 8 0 000 16z" />
              </svg>
            </button>
            <div className="flex-1 w-full overflow-y-auto flex flex-col items-center gap-1 pt-1">
              {(tree?.sections ?? []).map((s) => (
                <button
                  key={s.collection_id}
                  type="button"
                  title={s.collection?.name ?? s.collection_id}
                  onClick={() => {
                    setSidebarCollapsed(false);
                    setCollapsedCollections((prev) => {
                      const next = new Set(prev);
                      next.delete(s.collection_id);
                      return next;
                    });
                  }}
                  className="w-8 h-8 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
                >
                  <CollectionIconPip icon={s.collection?.icon} color={s.collection?.color} size={18} />
                </button>
              ))}
            </div>
            <div className="w-full border-t border-[var(--border-subtle)]">
              <Link
                href={
                  (viewMode === "docs" && doc?.browse_url ? doc.browse_url : tree?.base_url) || "#"
                }
                target="_blank"
                rel="noopener noreferrer"
                title="Open in Outline"
                className="w-full h-10 inline-flex items-center justify-center bg-[var(--accent)] text-white hover:opacity-90 transition-opacity"
              >
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </Link>
            </div>
          </aside>
        ) : (
          <aside
            className={`${
              mobileNavOpen ? "flex" : "hidden"
            } md:flex w-[280px] shrink-0 border-r border-[var(--border-subtle)] bg-[var(--bg-surface)] flex-col`}
          >
            <div className="flex-1 min-h-0 overflow-y-auto p-3">
              {/* Top row — collapse chevron (left) + Search button side-by-side */}
              <div className="flex items-center gap-2 mb-3">
                <button
                  type="button"
                  onClick={() => setSidebarCollapsed(true)}
                  title="Collapse nav"
                  className="hidden md:inline-flex w-8 h-8 items-center justify-center rounded-[var(--radius-md)] text-[var(--text-faint)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)] shrink-0"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
                  </svg>
                </button>
                <div className="flex-1 min-w-0">{searchNavButton}</div>
              </div>
              {nav}
            </div>
          {/* Footer — edge-to-edge primary button. Always enabled: deep-links
              to the selected doc, or falls back to the Outline workspace root. */}
          <div className="shrink-0 border-t border-[var(--border-subtle)]">
            <Link
              href={
                (viewMode === "docs" && doc?.browse_url ? doc.browse_url : tree?.base_url) || "#"
              }
              target="_blank"
              rel="noopener noreferrer"
              className="w-full h-10 inline-flex items-center justify-center gap-1.5 bg-[var(--accent)] text-white text-[11px] font-medium hover:opacity-90 transition-opacity"
            >
              Open in Outline
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            </Link>
          </div>
          </aside>
        )}

        {/* RIGHT PANE — the only scrollable region */}
        <section className="flex-1 min-w-0 flex flex-col h-full">
          <div className="md:hidden shrink-0 border-b border-[var(--border-default)] bg-[var(--bg-surface)] px-4 py-2">
            <button
              type="button"
              onClick={() => setMobileNavOpen((v) => !v)}
              className="text-xs px-2 py-1 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-elevated)]"
            >
              {mobileNavOpen ? "Close nav" : "Browse"}
            </button>
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto px-6 py-6 flex flex-col">
            {paneContent}
          </div>
        </section>
      </div>

      <CreateDocumentModal
        open={createForCollection !== null}
        onClose={() => setCreateForCollection(null)}
        onSubmit={(title) =>
          createMutation
            .mutateAsync({ title, collectionID: createForCollection! })
            .then(() => setCreateForCollection(null))
        }
        submitting={createMutation.isPending}
      />
    </PageShell>
  );
}
