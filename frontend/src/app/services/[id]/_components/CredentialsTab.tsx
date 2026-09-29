"use client";

import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function CredentialsTab({ serviceId, isAdmin, t }: { serviceId: number; isAdmin: boolean; t: (key: string) => string }) {
  const router = useRouter();
  // Shared service-scoped secrets from the unified vault.
  const { data: secrets = [] } = useQuery({
    queryKey: ["service-secrets", serviceId],
    queryFn: () => secretsAPI.list({ scope: "service", parent_id: serviceId, visibility: "shared" }),
  });
  const addHref = `/secrets?scope=service&parent_id=${serviceId}`;

  return (
    <SectionCard
      as="h3"
      title={t("service.credentials")}
      count={secrets.length}
      body="flush"
      empty={secrets.length === 0 ? t("service.noCredentials") : undefined}
      controls={isAdmin && (
        <IconButton onClick={() => router.push(addHref)} label={t("service.addCredential")}><Icon path={ICON_PATHS.plus} /></IconButton>
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
  );
}
