"use client";

import { useLocale } from "@/contexts/LocaleContext";
import LinkedDatabasesCard from "@/app/atlas/databases/_components/LinkedDatabasesCard";

/** A project's databases: linked directly, or through its services (as their
 *  engine or as a consumer). */
export default function ProjectDatabasesTab({ projectId, canEdit }: { projectId: number; canEdit: boolean }) {
  const { t } = useLocale();
  return (
    <LinkedDatabasesCard title={t("nav.databases")} filter={{ project_id: projectId }} canEdit={canEdit}
      empty={t("atlas.databases.noneLinked")} prefill={{ projectIds: [projectId] }} />
  );
}
