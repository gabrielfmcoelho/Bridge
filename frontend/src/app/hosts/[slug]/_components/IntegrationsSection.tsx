"use client";

import { useQuery } from "@tanstack/react-query";
import { coolifyAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import CoolifyIntegration from "./CoolifyIntegration";

type Props = {
  slug: string;
  keyTestStatus?: "success" | "failed" | null;
  coolifyServerUUID?: string | null;
  t: (key: string) => string;
  isAdmin: boolean;
};

export default function IntegrationsSection({ slug, keyTestStatus, coolifyServerUUID, t, isAdmin }: Props) {
  const { data: coolifyStatus } = useQuery({
    queryKey: ["coolify-status"],
    queryFn: coolifyAPI.status,
    staleTime: 60_000,
  });

  // Don't render the section at all if no integrations are enabled
  if (!coolifyStatus?.enabled) return null;

  const keyReady = keyTestStatus === "success";

  return (
    <SectionCard as="h3" title={t("operation.integrations")} body="flush">
      <CoolifyIntegration
        slug={slug}
        coolifyUUID={coolifyServerUUID}
        available={keyReady}
        t={t}
        isAdmin={isAdmin}
      />
    </SectionCard>
  );
}
