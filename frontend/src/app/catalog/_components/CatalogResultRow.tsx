"use client";

import Link from "next/link";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import { useLocale } from "@/contexts/LocaleContext";
import type { CatalogHit } from "@/lib/types";

// asset_type -> the same nav label already used for that inventory section,
// so a result row's type badge reads consistently with the sidebar entry
// its link opens.
const ASSET_TYPE_LABEL_KEY: Record<string, string> = {
  host: "nav.hosts",
  service: "nav.services",
  dns: "nav.dns",
  project: "nav.projects",
  api_catalog: "nav.apis",
  tool: "nav.tools",
};

export default function CatalogResultRow({
  hit,
  onRequest,
}: {
  hit: CatalogHit;
  /** Fired with the offering hit when its primary action is pressed. A no-op
   *  today — task A4 wires this into the request modal, using the state this
   *  handler is backed by in CatalogSearch. */
  onRequest: (hit: CatalogHit) => void;
}) {
  const { t } = useLocale();

  if (hit.kind === "offering") {
    return (
      <Card accent="none" className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="font-medium text-[var(--text-primary)] truncate">{hit.name}</p>
          {hit.description && <p className="text-xs text-[var(--text-muted)] truncate">{hit.description}</p>}
        </div>
        <Button size="sm" onClick={() => onRequest(hit)} className="shrink-0">
          {t("catalog.request")}
        </Button>
      </Card>
    );
  }

  const typeLabelKey = ASSET_TYPE_LABEL_KEY[hit.asset_type];

  return (
    <Link href={hit.href} className="block">
      <Card accent="emerald" clickIndicator="link" className="flex items-center gap-3 !pb-7">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 flex-wrap">
            <p className="font-medium text-[var(--text-primary)] truncate">{hit.name}</p>
            <Badge color="gray">{typeLabelKey ? t(typeLabelKey) : hit.asset_type}</Badge>
            {hit.entidade_name && <span className="text-xs text-[var(--text-muted)] truncate">{hit.entidade_name}</span>}
          </div>
          {hit.description && <p className="text-xs text-[var(--text-muted)] truncate mt-0.5">{hit.description}</p>}
        </div>
      </Card>
    </Link>
  );
}
