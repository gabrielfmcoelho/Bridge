"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Drawer from "@/components/ui/Drawer";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import ApiForm, { type ApiFormPrefill } from "../ApiForm";
import { baseHost } from "./apiDisplay";

type LinkFilter = { service_id: number } | { project_id: number };

/** The query a service's or project's APIs tab and its badge share. */
export const linkedApisQuery = (filter: LinkFilter) => ({
  queryKey: ["api-catalog", "linked", filter],
  queryFn: () => apiCatalogAPI.list(filter),
});

/**
 * The APIs linked to a service or project, as a flush row list; "+" imports
 * a new one with the link (and whatever else is known) prefilled.
 */
export default function LinkedApisCard({ filter, prefill, canEdit }: { filter: LinkFilter; prefill: ApiFormPrefill; canEdit: boolean }) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const { data: apis = [], isLoading } = useQuery(linkedApisQuery(filter));

  return (
    <>
      <SectionCard
        as="h3"
        title={t("nav.apis")}
        count={apis.length}
        body="flush"
        empty={!isLoading && apis.length === 0 ? t("atlas.apis.noneLinked") : undefined}
        controls={canEdit && (
          <IconButton onClick={() => setCreating(true)} label={t("atlas.apis.add")}><Icon path={ICON_PATHS.plus} /></IconButton>
        )}
      >
        <RowList>
          {apis.map((a) => (
            <ListRow key={a.id} href={`/atlas/apis/${a.id}`}>
              <Icon path={ICON_PATHS.code} className="w-3.5 h-3.5 shrink-0 text-[var(--rose)]" />
              <RowText title={a.name} meta={baseHost(a) || undefined} />
              <span className="shrink-0 text-xs text-[var(--text-muted)] font-mono">
                {[a.version_label, t("atlas.apis.endpointsCount", { count: String(a.operation_count) })].filter(Boolean).join(" · ")}
              </span>
            </ListRow>
          ))}
        </RowList>
      </SectionCard>
      <Drawer open={creating} onClose={() => setCreating(false)} title={t("atlas.apis.importTitle")} subHeader={formSubHeader} footer={formFooter}>
        <ApiForm
          prefill={prefill}
          onClose={() => setCreating(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={() => {
            setCreating(false);
            qc.invalidateQueries({ queryKey: ["api-catalog"] });
            qc.invalidateQueries({ queryKey: ["graph"] });
          }}
        />
      </Drawer>
    </>
  );
}
