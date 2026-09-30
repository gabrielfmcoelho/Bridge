"use client";

import { useQuery } from "@tanstack/react-query";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useDebounce } from "@/hooks/useDebounce";
import SectionCard from "@/components/ui/SectionCard";
import { RowList, ListRow } from "@/components/ui/RowList";
import { Skeleton } from "@/components/ui/Skeleton";
import { MethodBadge } from "./apiDisplay";

/** The "Endpoints" tab: operations across every visible API, by the toolbar search. */
export default function EndpointSearch({ search }: { search: string }) {
  const { t } = useLocale();
  const q = useDebounce(search.trim(), 250);
  const { data: ops = [], isLoading } = useQuery({
    queryKey: ["api-ops", q],
    queryFn: () => apiCatalogAPI.searchOperations({ q: q || undefined }),
  });
  return (
    <SectionCard as="h2" title={t("atlas.apis.tabEndpoints")} count={ops.length} body="flush"
      empty={!isLoading && ops.length === 0 ? t("atlas.apis.noEndpointMatches") : undefined}>
      {isLoading ? (
        <div className="p-4 space-y-2">{Array.from({ length: 6 }).map((_, i) => <Skeleton key={i} className="h-9" />)}</div>
      ) : (
        <RowList>
          {ops.map((op, i) => (
            <ListRow key={`${op.api_id}-${op.op_key}-${i}`} href={`/atlas/apis/${op.api_id}?op=${encodeURIComponent(op.op_key)}`}>
              <MethodBadge method={op.method} />
              <div className="min-w-0 flex-1">
                <p className="font-mono text-xs text-[var(--text-primary)] truncate">{op.path}</p>
                {(op.summary || op.description) && <p className="text-xs text-[var(--text-muted)] truncate">{op.summary || op.description}</p>}
              </div>
              <span className="shrink-0 text-xs text-[var(--text-muted)] truncate max-w-[40%]">{op.api_name}</span>
            </ListRow>
          ))}
        </RowList>
      )}
    </SectionCard>
  );
}
