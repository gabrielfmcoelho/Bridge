"use client";

import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import { secretsAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface CredentialsTabProps {
  serviceId: number;
  isAdmin: boolean;
  t: (key: string) => string;
}

export default function CredentialsTab({ serviceId, isAdmin, t }: CredentialsTabProps) {
  const router = useRouter();
  // Fetch shared service-scoped secrets via the unified vault. The legacy
  // /api/services/{id}/credentials route was removed in the Phase 1 cutover.
  const { data: secrets = [] } = useQuery({
    queryKey: ["service-secrets", serviceId],
    queryFn: () => secretsAPI.list({ scope: "service", parent_id: serviceId, visibility: "shared" }),
  });

  return (
    <div className="space-y-5 animate-fade-in">
      <SectionCard
        title={t("service.credentials")}
        controls={isAdmin && (
          <Button
            size="sm"
            variant="secondary"
            onClick={() => router.push(`/secrets?scope=service&parent_id=${serviceId}`)}
          >
            {t("service.addCredential")}
          </Button>
        )}
      >
        {secrets.length > 0 ? (
          <div className="space-y-2">
            {secrets.map((s) => (
              <div
                key={s.id}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)]"
              >
                <Icon path={ICON_PATHS.keyOutline} className="w-4 h-4 text-[var(--accent)] shrink-0" strokeWidth={1.5} />
                <Badge>{s.name}</Badge>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("service.noCredentials")}</p>
        )}
      </SectionCard>
    </div>
  );
}
