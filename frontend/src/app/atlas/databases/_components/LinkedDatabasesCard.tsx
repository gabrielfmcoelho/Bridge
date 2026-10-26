"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { databasesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Badge from "@/components/ui/Badge";
import Drawer from "@/components/ui/Drawer";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import DatabaseForm, { type DatabaseFormPrefill } from "../DatabaseForm";
import { SENSITIVITY_COLOR } from "./databaseInsights";

type LinkFilter = { instance_service_id: number } | { service_id: number } | { project_id: number };

/** The query a service's or project's databases tab and its badge share. */
export const linkedDatabasesQuery = (filter: LinkFilter) => ({
  queryKey: ["databases", "linked", filter],
  queryFn: () => databasesAPI.list(filter),
});

/** Databases linked to a service (as instance or consumer) or project, as a
 *  flush row list; "+" registers one with the link prefilled. */
export default function LinkedDatabasesCard({ title, filter, prefill, canEdit, empty }: {
  title: string;
  filter: LinkFilter;
  prefill: DatabaseFormPrefill;
  canEdit: boolean;
  empty: string;
}) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [formSubHeader, setFormSubHeader] = useState<React.ReactNode>(null);
  const [formFooter, setFormFooter] = useState<React.ReactNode>(null);
  const { data: dbs = [], isLoading } = useQuery(linkedDatabasesQuery(filter));

  return (
    <>
      <SectionCard
        as="h3"
        title={title}
        count={dbs.length}
        body="flush"
        empty={!isLoading && dbs.length === 0 ? empty : undefined}
        controls={canEdit && (
          <IconButton onClick={() => setCreating(true)} label={t("atlas.databases.add")}><Icon path={ICON_PATHS.plus} /></IconButton>
        )}
      >
        <RowList>
          {dbs.map((d) => (
            <ListRow key={d.id} href={`/atlas/databases/${d.id}`}>
              <Icon path={ICON_PATHS.database} className="w-3.5 h-3.5 shrink-0 text-[var(--info)]" />
              <RowText title={d.name} meta={[d.db_name, t(`atlas.databases.engine.${d.engine}`)].filter(Boolean).join(" · ")} />
              <Badge color={SENSITIVITY_COLOR[d.sensitivity]}>{t(`atlas.databases.sensitivity.${d.sensitivity}`)}</Badge>
            </ListRow>
          ))}
        </RowList>
      </SectionCard>
      <Drawer open={creating} onClose={() => setCreating(false)} title={t("atlas.databases.addTitle")} subHeader={formSubHeader} footer={formFooter}>
        <DatabaseForm
          prefill={prefill}
          onClose={() => setCreating(false)}
          onSubHeaderChange={setFormSubHeader}
          onFooterChange={setFormFooter}
          onSuccess={() => {
            setCreating(false);
            qc.invalidateQueries({ queryKey: ["databases"] });
            qc.invalidateQueries({ queryKey: ["graph"] });
          }}
        />
      </Drawer>
    </>
  );
}
