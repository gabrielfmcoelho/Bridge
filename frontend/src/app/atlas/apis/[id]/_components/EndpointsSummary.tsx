"use client";

import SectionCard from "@/components/ui/SectionCard";
import Field from "@/components/ui/Field";
import { RowList, RowGroup, ListRow, RowText } from "@/components/ui/RowList";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { ApiCatalog } from "@/lib/types";
import { MethodBadge } from "../../_components/apiDisplay";
import { isSpecStale, SPEC_STALE_DAYS } from "../../_components/apiInsights";

const METHOD_ORDER = ["GET", "POST", "PUT", "PATCH", "DELETE"];

function tally(values: string[]): [string, number][] {
  const m = new Map<string, number>();
  for (const v of values) m.set(v, (m.get(v) ?? 0) + 1);
  return [...m];
}

/** The right column of an API's "Visão geral": its spec's freshness, then
 *  the endpoints by method and by tag. A row opens the Endpoints tab there. */
export default function EndpointsSummary({ api, onOpenEndpoints }: { api: ApiCatalog; onOpenEndpoints: () => void }) {
  const { t, locale } = useLocale();
  const ops = api.operations ?? [];
  const methods = tally(ops.map((o) => o.method.toUpperCase()))
    .sort((a, b) => (METHOD_ORDER.indexOf(a[0]) + 1 || 99) - (METHOD_ORDER.indexOf(b[0]) + 1 || 99));
  const tags = tally(ops.flatMap((o) => (o.tags?.length ? o.tags : [""])))
    .sort((a, b) => b[1] - a[1] || (a[0] === "" ? 1 : b[0] === "" ? -1 : a[0].localeCompare(b[0])));
  const stale = isSpecStale(api);

  return (
    <>
      <SectionCard as="h3" title={t("atlas.apis.specTitleCard")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("atlas.apis.updated")} value={api.updated_at ? getTimeAgo(api.updated_at, locale) : ""} />
          <Field label={t("atlas.apis.freshness")} value={api.source_type !== "url" ? t("atlas.apis.freshnessUpload") : stale ? t("atlas.apis.kpi.specStaleHint", { days: String(SPEC_STALE_DAYS) }) : t("atlas.apis.specFresh")} />
          <Field className="col-span-2" label={t("atlas.apis.specHash")} value={api.spec_hash ? api.spec_hash.slice(0, 16) : ""} mono />
        </div>
      </SectionCard>

      <SectionCard as="h3" title={t("atlas.apis.tabEndpoints")} count={ops.length} body="flush" empty={ops.length === 0 ? t("atlas.apis.noEndpoints") : undefined}>
        <RowList>
          <RowGroup title={t("atlas.apis.byMethod")}>
            {methods.map(([m, n]) => (
              <ListRow key={m} onClick={onOpenEndpoints}>
                <MethodBadge method={m} />
                <span className="flex-1" />
                <span className="text-xs font-mono text-[var(--text-muted)]">{n}</span>
              </ListRow>
            ))}
          </RowGroup>
          <RowGroup title={t("atlas.apis.byTag")}>
            {tags.map(([tag, n]) => (
              <ListRow key={tag || "-"} onClick={onOpenEndpoints}>
                <RowText title={tag || t("atlas.apis.untagged")} />
                <span className="text-xs font-mono text-[var(--text-muted)]">{n}</span>
              </ListRow>
            ))}
          </RowGroup>
        </RowList>
      </SectionCard>
    </>
  );
}
