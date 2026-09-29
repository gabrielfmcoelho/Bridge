"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import NewSecretModal from "@/app/secrets/_components/NewSecretModal";

export default function CredentialsTab({ serviceId, isAdmin, t }: { serviceId: number; isAdmin: boolean; t: (key: string) => string }) {
  const [creating, setCreating] = useState(false);
  // Shared service-scoped secrets from the unified vault.
  const { data: secrets = [] } = useQuery({
    // "secrets-all" prefix: the vault modal invalidates it after a save.
    queryKey: ["secrets-all", "service", serviceId],
    queryFn: () => secretsAPI.list({ scope: "service", parent_id: serviceId, visibility: "shared" }),
  });
  return (
    <>
      <SectionCard
        as="h3"
        title={t("service.credentials")}
        count={secrets.length}
        body="flush"
        empty={secrets.length === 0 ? t("service.noCredentials") : undefined}
        controls={isAdmin && (
          <IconButton onClick={() => setCreating(true)} label={t("service.addCredential")}><Icon path={ICON_PATHS.plus} /></IconButton>
        )}
      >
        <RowList>
          {secrets.map((s) => (
            <ListRow key={s.id} href={`/secrets?scope=service&parent_id=${serviceId}`}>
              <Icon path={ICON_PATHS.keyOutline} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" />
              <RowText title={s.name} />
            </ListRow>
          ))}
        </RowList>
      </SectionCard>
      {creating && <NewSecretModal open onClose={() => setCreating(false)} defaultScope="service" defaultParentId={serviceId} />}
    </>
  );
}
