"use client";

import { useLocale } from "@/contexts/LocaleContext";
import LinkedDatabasesCard from "@/app/atlas/databases/_components/LinkedDatabasesCard";
import type { DatabaseEngine } from "@/lib/types";

/** A service's databases: the ones it hosts (when it is an engine) and the
 *  ones it uses. "+" prefills the matching link. */
export default function ServiceDatabasesTab({ serviceId, projectId, isEngine, engine, canEdit }: {
  serviceId: number;
  projectId?: number | null;
  isEngine: boolean;
  engine?: DatabaseEngine;
  canEdit: boolean;
}) {
  const { t } = useLocale();
  const projectIds = projectId ? [projectId] : [];
  return (
    <div className="space-y-5">
      {isEngine && (
        <LinkedDatabasesCard title={t("atlas.databases.hosted")} filter={{ instance_service_id: serviceId }} canEdit={canEdit}
          empty={t("atlas.databases.noneHosted")} prefill={{ instanceServiceId: serviceId, engine, projectIds }} />
      )}
      <LinkedDatabasesCard title={t("atlas.databases.used")} filter={{ service_id: serviceId }} canEdit={canEdit}
        empty={t("atlas.databases.noneUsed")} prefill={{ consumerServiceIds: [serviceId], projectIds }} />
    </div>
  );
}
