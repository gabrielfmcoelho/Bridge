"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Button from "@/components/ui/Button";
import TabBar, { type Tab } from "@/components/ui/TabBar";
import SortableTable, { sortRows, type SortableColumn } from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import ListToolbar from "@/components/ui/ListToolbar";
import { SkeletonTable } from "@/components/ui/Skeleton";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { requestsAPI } from "@/lib/api";
import { statusColor, statusLabelKey } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { ServiceRequest, RequestStatus } from "@/lib/types";
import RequestFilterDrawer from "./RequestFilterDrawer";

const PER_PAGE = 20;
const DEBOUNCE_MS = 300;
const VIEWS = ["mine", "approve", "fulfill", "all"] as const;
type View = (typeof VIEWS)[number];
type ColKey = "title" | "offering" | "requester" | "priority" | "status" | "created_at";

function isView(v: string | null): v is View {
  return !!v && (VIEWS as readonly string[]).includes(v);
}

const PRIORITY_RANK: Record<string, number> = { low: 0, medium: 1, high: 2, critical: 3 };
// Mirrors the submitted → ... → terminal ordering in src/lib/requests.ts (TRANSITIONS).
const STATUS_RANK: Record<RequestStatus, number> = {
  submitted: 0, under_review: 1, needs_info: 2, approved: 3,
  in_progress: 4, delivered: 5, rejected: 6, cancelled: 7,
};

export default function RequestList() {
  const { t, formatDate } = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [view, setView] = useState<View>(isView(searchParams.get("view")) ? (searchParams.get("view") as View) : "mine");
  const [inputValue, setInputValue] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [status, setStatus] = useState("");
  const [offeringId, setOfferingId] = useState("");
  const [priority, setPriority] = useState("");
  const [page, setPage] = useState(1);
  const [filtersOpen, setFiltersOpen] = useState(false);

  // Debounce the free-text search the same way src/app/catalog/_components/
  // CatalogSearch.tsx does: instant local echo, ~300ms before it hits the query.
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(() => setDebouncedQ(inputValue.trim()), DEBOUNCE_MS);
    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current);
    };
  }, [inputValue]);

  // Keep ?view= in sync so a tab is shareable — same pattern as CatalogSearch's ?q= sync.
  useEffect(() => {
    const qs = new URLSearchParams(searchParams.toString());
    qs.set("view", view);
    const next = qs.toString();
    router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [view]);

  useEffect(() => {
    setPage(1);
  }, [view, status, offeringId, debouncedQ]);

  // Query key starts with "requests" on purpose: task A4's submit flow calls
  // queryClient.invalidateQueries({ queryKey: ["requests"] }), and React Query
  // matches by prefix — anything else here and submitting a request would
  // silently stop refreshing this list.
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["requests", view, status, offeringId, debouncedQ, page],
    queryFn: () =>
      requestsAPI.listPaginated({
        view,
        status: status || undefined,
        offering_id: offeringId ? Number(offeringId) : undefined,
        q: debouncedQ || undefined,
        page,
        per_page: PER_PAGE,
      }),
  });

  const rows = data?.data ?? [];
  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  // ponytail: requestsAPI.listPaginated has no priority param (see
  // task-a5-brief.md) — narrowing by priority can only act on the rows
  // already on this page, not the whole server-side result set. Add a real
  // ?priority= once the API grows one; until then this stays a page-local
  // filter and RequestFilterDrawer says so in the UI.
  const filteredRows = priority ? rows.filter((r) => r.priority === priority) : rows;
  const emptyDueToFilter = rows.length > 0 && filteredRows.length === 0;

  const tabs: Tab[] = VIEWS.map((v) => ({ key: v, label: t(`requests.view.${v}`) }));
  const activeFilterCount = [status, offeringId, priority].filter(Boolean).length;

  const columns: SortableColumn<ColKey>[] = [
    { key: "title", label: t("requests.titleField") },
    { key: "offering", label: t("requests.offering") },
    { key: "requester", label: t("requests.requester") },
    { key: "priority", label: t("requests.priority") },
    { key: "status", label: t("common.status") },
    { key: "created_at", label: t("requests.createdAt") },
  ];

  // Client-side comparators for the sort above — see the ponytail note there;
  // this only ever sorts the rows already on the page.
  const comparators: Record<ColKey, (a: ServiceRequest, b: ServiceRequest) => number> = {
    title: (a, b) => a.title.localeCompare(b.title),
    offering: (a, b) => (a.offering_name ?? "").localeCompare(b.offering_name ?? ""),
    requester: (a, b) => (a.requester_name ?? "").localeCompare(b.requester_name ?? ""),
    priority: (a, b) => (PRIORITY_RANK[a.priority] ?? 0) - (PRIORITY_RANK[b.priority] ?? 0),
    status: (a, b) => STATUS_RANK[a.status] - STATUS_RANK[b.status],
    created_at: (a, b) => a.created_at.localeCompare(b.created_at),
  };

  const clearFilters = () => {
    setStatus("");
    setOfferingId("");
    setPriority("");
  };

  // The four tabs mean genuinely different things when empty — a truthful
  // message per view, not one generic "no results".
  function emptyState(): { icon: "folder" | "key" | "box" | "globe"; title: string; description: string; action?: ReactNode } {
    switch (view) {
      case "mine":
        return {
          icon: "folder",
          title: t("requests.empty.mine.title"),
          description: t("requests.empty.mine.hint"),
          action: (
            <Link href="/catalog">
              <Button size="sm">{t("requests.browseCatalog")}</Button>
            </Link>
          ),
        };
      case "approve":
        return { icon: "key", title: t("requests.empty.approve.title"), description: t("requests.empty.approve.hint") };
      case "fulfill":
        return { icon: "box", title: t("requests.empty.fulfill.title"), description: t("requests.empty.fulfill.hint") };
      case "all":
      default:
        return { icon: "globe", title: t("requests.empty.all.title"), description: t("requests.empty.all.hint") };
    }
  }

  return (
    <PageShell>
      <PageHeader title={t("requests.title")} addLabel={t("requests.new")} onAdd={() => router.push("/catalog")} />
      <p className="text-sm text-[var(--text-muted)] -mt-4 mb-6">{t("requests.subtitle")}</p>

      <TabBar tabs={tabs} activeTab={view} onChange={(k) => setView(k as View)} className="mb-4" />

      <ListToolbar
        search={inputValue}
        onSearchChange={setInputValue}
        onFilterClick={() => setFiltersOpen(true)}
        activeFilterCount={activeFilterCount}
        searchPlaceholder={t("requests.searchPlaceholder")}
      />

      {isLoading ? (
        <SkeletonTable rows={6} />
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message || t("requests.error")}</StatusAlert>
      ) : filteredRows.length === 0 ? (
        emptyDueToFilter ? (
          <EmptyState icon="search" title={t("requests.emptyFiltered")} description={t("requests.emptyFilteredHint")} />
        ) : (
          <EmptyState {...emptyState()} />
        )
      ) : (
        <div className="space-y-3">
          <SortableTable columns={columns} defaultSort="created_at" defaultDir="desc">
            {(sortKey, sortDir) =>
              sortRows(filteredRows, sortKey, sortDir, comparators).map((r, i) => (
                <tr
                  key={r.id}
                  className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
                  onClick={() => router.push(`/requests/${r.id}`)}
                >
                  <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">
                    <Link href={`/requests/${r.id}`} onClick={(e) => e.stopPropagation()} className="hover:underline">
                      {r.title}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{r.offering_name ?? "—"}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{r.requester_name ?? "—"}</td>
                  <td className="px-4 py-2.5 text-[var(--text-secondary)]">{t(`requests.priorityLevels.${r.priority}`)}</td>
                  <td className="px-4 py-2.5">
                    <Badge color={statusColor(r.status)}>{t(statusLabelKey(r.status))}</Badge>
                  </td>
                  <td className="px-4 py-2.5 text-[var(--text-muted)]">{formatDate(r.created_at)}</td>
                </tr>
              ))
            }
          </SortableTable>
          <Pagination page={page} totalPages={totalPages} total={total} perPage={PER_PAGE} onChange={setPage} />
        </div>
      )}

      <RequestFilterDrawer
        open={filtersOpen}
        onClose={() => setFiltersOpen(false)}
        status={status}
        onStatusChange={setStatus}
        offeringId={offeringId}
        onOfferingIdChange={setOfferingId}
        priority={priority}
        onPriorityChange={setPriority}
        onClearAll={clearFilters}
      />
    </PageShell>
  );
}
