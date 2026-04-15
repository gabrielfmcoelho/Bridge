"use client";

import { useQuery } from "@tanstack/react-query";
import { coolifyAPI } from "@/lib/api";
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
    <div className="space-y-2 pt-2">
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">
        {t("operation.integrations")}
      </h3>
      <CoolifyIntegration
        slug={slug}
        coolifyUUID={coolifyServerUUID}
        available={keyReady}
        t={t}
        isAdmin={isAdmin}
      />
    </div>
  );
}
