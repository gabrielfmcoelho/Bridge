"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery, useQueries } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { RowList, RowGroup, ListRow, RowText } from "@/components/ui/RowList";
import VaultEntryEditor from "@/components/vault/VaultEntryEditor";
import { ICON_PATHS } from "@/lib/icon-paths";
import { serviceTitle } from "@/lib/serviceDisplay";
import type { Secret, Service } from "@/lib/types";

/**
 * The Segredos tab: the project's own shared secrets, then each of its
 * services' — the project contains the services, so it sees theirs too.
 * Keys start with "secrets-all", which the vault's create/edit invalidate.
 */
export default function ProjectSecretsTab({ projectId, services, canEdit }: { projectId: number; services: Service[]; canEdit: boolean }) {
  const { t } = useLocale();
  const router = useRouter();
  const [editing, setEditing] = useState<Secret | null>(null);

  const { data: own = [] } = useQuery({
    queryKey: ["secrets-all", "projeto", projectId],
    queryFn: () => secretsAPI.list({ scope: "projeto", parent_id: projectId, visibility: "shared" }),
  });
  // ponytail: one request per service; add a ?project_id= filter to
  // /api/secrets if projects grow to dozens of services.
  const perService = useQueries({
    queries: services.map((s) => ({
      queryKey: ["secrets-all", "service", s.id],
      queryFn: () => secretsAPI.list({ scope: "service", parent_id: s.id, visibility: "shared" }),
    })),
  });
  const serviceGroups = services
    .map((s, i) => ({ service: s, secrets: perService[i]?.data ?? [] }))
    .filter((g) => g.secrets.length > 0);

  const row = (s: Secret) => (
    <ListRow key={s.id} onClick={canEdit ? () => setEditing(s) : undefined}>
      <Icon path={ICON_PATHS.keyOutline} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" />
      <RowText title={s.name} meta={s.group_label || s.description || undefined} />
      <span className="text-2xs font-mono text-[var(--text-muted)] shrink-0">{s.type}</span>
    </ListRow>
  );

  return (
    <div className="space-y-5">
      <SectionCard
        as="h3"
        title={t("project.secretsTitle")}
        description={t("project.secretsHint")}
        count={own.length}
        body="flush"
        empty={own.length === 0 ? t("project.noSecrets") : undefined}
        controls={canEdit && (
          <IconButton onClick={() => router.push("/secrets?scope=projeto")} label={t("vault.newSecretButton")}><Icon path={ICON_PATHS.plus} /></IconButton>
        )}
      >
        <RowList>{own.map(row)}</RowList>
      </SectionCard>

      {serviceGroups.length > 0 && (
        <SectionCard as="h3" title={t("project.serviceSecretsTitle")} count={serviceGroups.reduce((n, g) => n + g.secrets.length, 0)} body="flush">
          {serviceGroups.map((g) => (
            <RowGroup key={g.service.id} title={serviceTitle(g.service).title}>{g.secrets.map(row)}</RowGroup>
          ))}
        </SectionCard>
      )}

      <VaultEntryEditor secret={editing} onClose={() => setEditing(null)} />
    </div>
  );
}
