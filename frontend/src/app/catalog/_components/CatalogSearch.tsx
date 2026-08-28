"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Input from "@/components/ui/Input";
import { Skeleton } from "@/components/ui/Skeleton";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import SectionHeading from "@/components/ui/SectionHeading";
import Pagination from "@/components/ui/Pagination";
import { catalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { CatalogHit } from "@/lib/types";
import CatalogResultRow from "./CatalogResultRow";

const PER_PAGE = 20;
const DEBOUNCE_MS = 300;
const KIND_FILTERS = ["all", "offering", "asset"] as const;
type KindFilter = (typeof KIND_FILTERS)[number];

// The point of this page is anti-duplication: a user searching for "postgres"
// must see the running pg-prod-01 service at least as prominently as the
// "Managed Postgres" offering, so they don't file a request for something
// that already exists. That is why existing assets are grouped and rendered
// ahead of offerings below, not the other way around.
export default function CatalogSearch() {
  const { t } = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const initialQ = searchParams.get("q") ?? "";
  const [inputValue, setInputValue] = useState(initialQ);
  const [debouncedQ, setDebouncedQ] = useState(initialQ);
  const [kind, setKind] = useState<KindFilter>("all");
  const [page, setPage] = useState(1);

  // Lifted here (not into CatalogResultRow) so task A4 can wire the request
  // modal in by reading this state and handing it a close/submit callback,
  // without reshaping this page or its rows.
  const [selectedOffering, setSelectedOffering] = useState<CatalogHit | null>(null);

  // Debounce: update the local input instantly, but only push it into the
  // query (and the URL) ~300ms after the user stops typing.
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(() => {
      setDebouncedQ(inputValue.trim());
    }, DEBOUNCE_MS);
    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current);
    };
  }, [inputValue]);

  useEffect(() => {
    setPage(1);
  }, [debouncedQ, kind]);

  // Keep ?q= in sync so a search is shareable via URL.
  useEffect(() => {
    const qs = new URLSearchParams(searchParams.toString());
    if (debouncedQ) qs.set("q", debouncedQ);
    else qs.delete("q");
    const next = qs.toString();
    router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    // Only re-sync when the debounced term changes; searchParams/router/pathname
    // are stable app-router handles and including them would refire this on
    // every navigation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQ]);

  const hasQuery = debouncedQ.length > 0;

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["catalog-search", debouncedQ, kind, page],
    queryFn: () =>
      catalogAPI.search({
        q: debouncedQ,
        kind: kind === "all" ? undefined : kind,
        page,
        per_page: PER_PAGE,
      }),
    enabled: hasQuery,
  });

  const hits = data?.data ?? [];
  const total = data?.meta.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE));

  const assets = hits.filter((h) => h.kind === "asset");
  const offerings = hits.filter((h) => h.kind === "offering");

  const kindLabel = (k: KindFilter) =>
    k === "all" ? t("common.all") : k === "offering" ? t("catalog.resultsOfferings") : t("catalog.resultsExisting");

  const chipClass = (active: boolean) =>
    `px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-colors ${
      active
        ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
        : "border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
    }`;

  return (
    <PageShell>
      <PageHeader title={t("catalog.title")} />
      <p className="text-sm text-[var(--text-muted)] -mt-4 mb-6">{t("catalog.subtitle")}</p>

      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-6">
        <div className="flex-1 sm:max-w-md">
          <Input
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            placeholder={t("catalog.searchPlaceholder")}
          />
        </div>
        <div className="flex gap-1.5">
          {KIND_FILTERS.map((k) => (
            <button key={k} type="button" className={chipClass(kind === k)} onClick={() => setKind(k)}>
              {kindLabel(k)}
            </button>
          ))}
        </div>
      </div>

      {!hasQuery ? (
        <EmptyState icon="search" title={t("catalog.searchPrompt")} description={t("catalog.searchPromptHint")} />
      ) : isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-[68px] rounded-[var(--radius-lg)]" />
          ))}
        </div>
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message || t("catalog.error")}</StatusAlert>
      ) : hits.length === 0 ? (
        <EmptyState icon="search" title={t("catalog.empty")} description={t("catalog.emptyHint")} />
      ) : (
        <div className="space-y-6">
          {assets.length > 0 && (
            <div>
              <SectionHeading>
                {t("catalog.resultsExisting")} ({assets.length})
              </SectionHeading>
              <div className="space-y-2">
                {assets.map((hit) => (
                  <CatalogResultRow key={`asset-${hit.asset_type}-${hit.id}`} hit={hit} onRequest={setSelectedOffering} />
                ))}
              </div>
            </div>
          )}

          {offerings.length > 0 && (
            <div>
              <SectionHeading>
                {t("catalog.resultsOfferings")} ({offerings.length})
              </SectionHeading>
              <div className="space-y-2">
                {offerings.map((hit) => (
                  <CatalogResultRow key={`offering-${hit.id}`} hit={hit} onRequest={setSelectedOffering} />
                ))}
              </div>
            </div>
          )}

          <Pagination page={page} totalPages={totalPages} total={total} perPage={PER_PAGE} onChange={setPage} />
        </div>
      )}

      {/* Placeholder confirmation that the "Request" action is wired through
          to state — task A4 replaces this with the actual request modal. */}
      {selectedOffering && (
        <p className="text-xs text-[var(--text-faint)] mt-4">
          {t("catalog.request")}: {selectedOffering.name}
        </p>
      )}
    </PageShell>
  );
}
