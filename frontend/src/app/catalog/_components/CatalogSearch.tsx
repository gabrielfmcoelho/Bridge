"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import Pagination from "@/components/ui/Pagination";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { Skeleton } from "@/components/ui/Skeleton";
import { catalogAPI, offeringsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { Offering, RequestType } from "@/lib/types";
import CatalogAssetsTable, { type AssetSortKey } from "./CatalogAssetsTable";
import OfferingCard from "./OfferingCard";
import RequestFormModal from "./RequestFormModal";

const ASSETS_PER_PAGE = 12;
// The catch-all offering, surfaced as the header action and as the way out of
// an empty search. Looked up by slug because it is seeded data, not a schema
// flag — if an admin ever deletes it the button simply stops rendering rather
// than throwing.
const CATCHALL_SLUG = "solicitacao-avulsa";
const DEBOUNCE_MS = 300;
const KIND_FILTERS = ["all", "offering", "asset"] as const;
type KindFilter = (typeof KIND_FILTERS)[number];

// The point of this page is anti-duplication: a user searching "postgres" must
// see the running pg-prod-01 service at least as prominently as the "Managed
// Postgres" offering, so they don't file a request for something that already
// exists. Existing assets are therefore rendered ahead of offerings.
//
// The two groups are fetched SEPARATELY rather than as one mixed, paginated
// list. One list meant whichever group came first ate page 1 — 14 offerings
// could push every existing asset onto page 2, defeating the whole page. Two
// requests give each group an honest count and its own paging, and let the
// offerings side use GET /api/offerings, which returns the full record the
// cards need (request_type, category, use_cases) instead of the flat
// CatalogHit that /catalog/search projects.
export default function CatalogSearch() {
  const { t } = useLocale();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const initialQ = searchParams.get("q") ?? "";
  const [inputValue, setInputValue] = useState(initialQ);
  const [debouncedQ, setDebouncedQ] = useState(initialQ);
  const [kind, setKind] = useState<KindFilter>("all");
  const [assetPage, setAssetPage] = useState(1);
  const [assetSort, setAssetSort] = useState<{ key: AssetSortKey; dir: "asc" | "desc" }>({ key: "name", dir: "asc" });
  // Multi-select: an empty set means "no type filter", not "nothing selected".
  const [types, setTypes] = useState<Set<RequestType>>(new Set());
  const [selectedOffering, setSelectedOffering] = useState<Offering | null>(null);

  // Debounce: echo the input instantly, push it into the queries ~300ms later.
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(() => setDebouncedQ(inputValue.trim()), DEBOUNCE_MS);
    return () => {
      if (debounceTimer.current) clearTimeout(debounceTimer.current);
    };
  }, [inputValue]);

  useEffect(() => {
    setAssetPage(1);
  }, [debouncedQ, kind, assetSort]);

  // Keep ?q= shareable.
  useEffect(() => {
    const qs = new URLSearchParams(searchParams.toString());
    if (debouncedQ) qs.set("q", debouncedQ);
    else qs.delete("q");
    const next = qs.toString();
    router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQ]);

  const showOfferings = kind === "all" || kind === "offering";
  const showAssets = kind === "all" || kind === "asset";

  // Section order is the browse/search distinction made visible. Browsing with
  // no query, the catalog itself leads — that is what the page is for. The
  // moment a query exists the user is checking whether their thing already
  // exists, so the existing-assets group takes the top and the offerings they
  // might otherwise duplicate sit below it.
  const searching = debouncedQ.length > 0;

  // ponytail: the whole active catalog is fetched once and filtered in the
  // browser. A service catalog is curated — tens, not thousands — and one
  // cached list means the catch-all stays reachable no matter what the user
  // typed, plus no refetch per keystroke. Move filtering back to the server
  // the day an org authors more than ~60 offerings.
  const offeringsQuery = useQuery({
    queryKey: ["catalog-offerings"],
    queryFn: () => offeringsAPI.list({ active: true }),
  });

  const assetsQuery = useQuery({
    queryKey: ["catalog-assets", debouncedQ, assetPage, assetSort.key, assetSort.dir],
    queryFn: () =>
      catalogAPI.search({
        q: debouncedQ, kind: "asset", sort: assetSort.key, dir: assetSort.dir,
        page: assetPage, per_page: ASSETS_PER_PAGE,
      }),
    enabled: showAssets,
  });

  const catalog = offeringsQuery.data ?? [];
  const catchAll = catalog.find((o) => o.slug === CATCHALL_SLUG);

  // Mirrors the server's own offering match (name, description, category and
  // use cases) so moving this back server-side later changes nothing visible.
  const matchesQuery = (o: Offering) => {
    if (!debouncedQ) return true;
    const needle = debouncedQ.toLowerCase();
    return [o.name, o.description, o.category, ...(o.use_cases ?? [])].some((f) =>
      (f ?? "").toLowerCase().includes(needle)
    );
  };
  const allOfferings = catalog.filter(matchesQuery);

  // Tags are built from what the current query actually returned, so the page
  // never offers a filter that leads to an empty list. A type that is selected
  // stays listed even at zero, otherwise the chip clearing the filter would
  // vanish along with its own results.
  const typeCounts = new Map<RequestType, number>();
  for (const o of allOfferings) typeCounts.set(o.request_type, (typeCounts.get(o.request_type) ?? 0) + 1);
  for (const sel of types) if (!typeCounts.has(sel)) typeCounts.set(sel, 0);
  const typeTags = [...typeCounts.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));

  const offerings = types.size === 0 ? allOfferings : allOfferings.filter((o) => types.has(o.request_type));

  const toggleType = (rt: RequestType) =>
    setTypes((prev) => {
      const next = new Set(prev);
      if (!next.delete(rt)) next.add(rt);
      return next;
    });
  const assets = assetsQuery.data?.data ?? [];
  const assetTotal = assetsQuery.data?.meta.total ?? 0;
  const assetPages = Math.max(1, Math.ceil(assetTotal / ASSETS_PER_PAGE));

  const kindLabel = (k: KindFilter) =>
    k === "all" ? t("common.all") : k === "offering" ? t("catalog.resultsOfferings") : t("catalog.resultsExisting");

  const chipClass = (active: boolean) =>
    `rounded-full border px-3.5 py-1.5 text-xs transition-[color,background-color,border-color,transform] duration-200 active:scale-[0.97] ${
      active
        ? "border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent)]"
        : "border-[var(--border-default)] text-[var(--text-secondary)] hover:border-[var(--border-strong)] hover:text-[var(--text-primary)]"
    }`;

  const assetsSection = (
        <section>
          <SectionLabel label={t("catalog.resultsExisting")} count={assetsQuery.isLoading ? undefined : assetTotal} />
          {assetsQuery.isLoading ? (
            <Skeleton className="h-[320px] rounded-[var(--radius-lg)]" />
          ) : assetsQuery.isError ? (
            <StatusAlert variant="error">{(assetsQuery.error as Error)?.message || t("catalog.error")}</StatusAlert>
          ) : assets.length === 0 ? (
            <EmptyState icon="search" title={t("catalog.assetsEmpty")} description={t("catalog.assetsEmptyHint")} compact />
          ) : (
            <>
              <CatalogAssetsTable
                hits={assets}
                sortKey={assetSort.key}
                sortDir={assetSort.dir}
                onSortChange={(key, dir) => setAssetSort({ key, dir })}
              />
              {assetPages > 1 && (
                <div className="mt-4">
                  <Pagination page={assetPage} totalPages={assetPages} total={assetTotal} perPage={ASSETS_PER_PAGE} onChange={setAssetPage} />
                </div>
              )}
            </>
          )}
        </section>
  );

  const offeringsSection = (
        <section>
          <SectionLabel label={t("catalog.resultsOfferings")} count={offeringsQuery.isLoading ? undefined : offerings.length} />
          {offeringsQuery.isLoading ? (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-[280px] rounded-[var(--radius-lg)]" />
              ))}
            </div>
          ) : offeringsQuery.isError ? (
            <StatusAlert variant="error">{(offeringsQuery.error as Error)?.message || t("catalog.error")}</StatusAlert>
          ) : offerings.length === 0 ? (
            <EmptyState
              icon="box"
              title={t("catalog.offeringsEmpty")}
              description={t("catalog.offeringsEmptyHint")}
              compact
              action={
                catchAll && (
                  <Button size="sm" onClick={() => setSelectedOffering(catchAll)}>
                    {t("catalog.avulsa")}
                  </Button>
                )
              }
            />
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {offerings.map((o, i) => (
                <OfferingCard key={o.id} offering={o} onRequest={setSelectedOffering} index={i} />
              ))}
            </div>
          )}
        </section>
  );

  return (
    <PageShell>
      <PageHeader
        title={t("catalog.title")}
        addLabel={catchAll ? t("catalog.avulsa") : undefined}
        onAdd={catchAll ? () => setSelectedOffering(catchAll) : undefined}
      />
      <p className="-mt-4 mb-6 text-sm text-[var(--text-muted)]">{t("catalog.subtitle")}</p>

      <div className="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex-1 sm:max-w-md">
          <Input value={inputValue} onChange={(e) => setInputValue(e.target.value)} placeholder={t("catalog.searchPlaceholder")} />
        </div>
        <div className="flex gap-1.5">
          {KIND_FILTERS.map((k) => (
            <button key={k} type="button" className={chipClass(kind === k)} onClick={() => setKind(k)}>
              {kindLabel(k)}
            </button>
          ))}
        </div>
      </div>

      {/* Type tags filter offerings only, so they appear only when offerings
          are on screen. They act on the already-fetched list — no refetch. */}
      {showOfferings && typeTags.length > 1 && (
        <div className="mb-4 flex flex-wrap items-center gap-1.5">
          <button type="button" className={chipClass(types.size === 0)} onClick={() => setTypes(new Set())}>
            {t("catalog.allTypes")}
          </button>
          <span aria-hidden className="mx-1 h-4 w-px bg-[var(--border-subtle)]" />
          {typeTags.map(([rt, count]) => (
            <button
              key={rt}
              type="button"
              aria-pressed={types.has(rt)}
              className={chipClass(types.has(rt))}
              onClick={() => toggleType(rt)}
            >
              {t(`catalog.requestType.${rt}`)}
              <span className="ml-1.5 text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>{count}</span>
            </button>
          ))}
        </div>
      )}

      <p className="mb-8 text-xs text-[var(--text-faint)]">{t("catalog.searchPromptHint")}</p>

      {/* DOM order, not just paint order — tab and screen-reader order must
          match what the eye sees, so the sections are reordered as nodes
          rather than flipped with flex-col-reverse. */}
      <div className="space-y-10">
        {(searching ? ["assets", "offerings"] : ["offerings", "assets"]).map((section) =>
          section === "assets"
            ? showAssets && <div key="assets">{assetsSection}</div>
            : showOfferings && <div key="offerings">{offeringsSection}</div>
        )}
      </div>

      <RequestFormModal
        target={selectedOffering && { id: selectedOffering.id, name: selectedOffering.name }}
        onClose={() => setSelectedOffering(null)}
      />
    </PageShell>
  );
}

// Section labels sit on a hairline rule rather than inside a card header —
// the groups are separated by structure and space, not by another box.
function SectionLabel({ label, count }: { label: string; count?: number }) {
  return (
    <div className="mb-4 flex items-baseline gap-2.5 border-b border-[var(--border-subtle)] pb-2.5">
      <h2 className="text-xs font-semibold uppercase tracking-wider text-[var(--text-secondary)]">{label}</h2>
      {count !== undefined && (
        <span className="text-xs text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
          {count}
        </span>
      )}
    </div>
  );
}
