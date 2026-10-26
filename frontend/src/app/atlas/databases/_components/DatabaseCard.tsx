"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import { CardHeader, CardMetadataGrid, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Database } from "@/lib/types";
import { plainText } from "@/app/atlas/apis/_components/apiInsights";
import { linkedNames, useApiLinkNames } from "@/app/atlas/apis/_components/apiDisplay";
import { ENVIRONMENT_COLOR, SENSITIVITY_COLOR, endpointLabel, isExternal, isPersonal, lacksBackup } from "./databaseInsights";

export default function DatabaseCard({ db }: { db: Database }) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const consumers = db.consumer_service_ids ?? [];
  const where = isExternal(db) ? db.external_provider || t("atlas.databases.external") : db.instance?.nickname ?? "";
  return (
    <Link href={`/atlas/databases/${db.id}`} className="block h-full">
      <Card accent={isPersonal(db) ? SENSITIVITY_COLOR[db.sensitivity] : "info"} className="h-full flex flex-col overflow-hidden">
        <CardHeader
          titleFont="display"
          title={db.name}
          subtitle={db.db_name || undefined}
          status={
            <span className="inline-flex flex-wrap items-center gap-1.5">
              <Badge color="cyan">{t(`atlas.databases.engine.${db.engine}`)}</Badge>
              <Badge color={ENVIRONMENT_COLOR[db.environment]}>{t(`atlas.databases.environment.${db.environment}`)}</Badge>
              <Badge color={SENSITIVITY_COLOR[db.sensitivity]}>{t(`atlas.databases.sensitivity.${db.sensitivity}`)}</Badge>
            </span>
          }
          description={plainText(db.description)}
        />
        <CardMetadataGrid
          items={[
            { label: isExternal(db) ? t("atlas.databases.provider") : t("atlas.databases.instance"), value: where },
            { label: t("atlas.databases.endpoint"), value: endpointLabel(db) },
            { label: t("atlas.databases.consumers"), value: linkedNames(consumers, names.service) },
            { label: t("atlas.apis.mainResponsavel"), value: db.main_responsavel_name ?? "" },
          ]}
        />
        <CardIndicatorGrid>
          <CardIndicator icon={ICON_PATHS.link} count={consumers.length} color="accent" title={t("atlas.databases.consumers")} />
          <CardIndicator icon={ICON_PATHS.folder} count={db.project_ids?.length ?? 0} color="accent" title={t("topology.projects")} />
          <CardIndicator icon={ICON_PATHS.keyOutline} count={db.has_credential ? 1 : 0} color="success" hideCount
            title={db.has_credential ? t("atlas.databases.hasCredential") : t("atlas.databases.noCredential")} />
          <CardIndicator icon={ICON_PATHS.alert} count={lacksBackup(db) ? 1 : 0} color="warning" hideCount
            title={lacksBackup(db) ? t("atlas.databases.kpi.prodNoBackup") : t("atlas.databases.backupOk")} />
        </CardIndicatorGrid>
      </Card>
    </Link>
  );
}
