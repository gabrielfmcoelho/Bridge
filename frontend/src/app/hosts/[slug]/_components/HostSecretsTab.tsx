"use client";

import { useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import type { Secret } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import Card from "@/components/ui/Card";
import SectionHeading from "@/components/ui/SectionHeading";
import PillButton from "@/components/ui/PillButton";
import StatusDot from "@/components/ui/StatusDot";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/** Secrets the scan found in this host's command lines (vault, group `scan-*`). */
export default function HostSecretsTab({ hostId, scanText }: { hostId: number; scanText: string }) {
  const { t } = useLocale();
  const { data, isLoading, error } = useQuery({
    queryKey: ["host-scan-secrets", hostId],
    queryFn: () => secretsAPI.list({ scope: "host", parent_id: hostId, type: "env_var" }),
  });
  const secrets = (data ?? []).filter((s) => s.group_label?.startsWith("scan-"));

  return (
    <div className="space-y-3 animate-fade-in">
      <SectionHeading variant="section" icon={ICON_PATHS.lock} count={secrets.length} hint={t("host.secrets.hint")}>
        {t("host.secrets.title")}
      </SectionHeading>
      {error ? (
        <StatusAlert variant="error">{t("host.secrets.loadError")}: {(error as Error).message}</StatusAlert>
      ) : !isLoading && secrets.length === 0 ? (
        <EmptyState compact icon="search" title={t("host.secrets.empty")} />
      ) : (
        <div className="space-y-2">
          {secrets.map((s) => (
            <SecretRow key={s.id} secret={s} inScan={scanText.includes(`{{secret:${s.id}}}`)} />
          ))}
        </div>
      )}
    </div>
  );
}

function SecretRow({ secret, inScan }: { secret: Secret; inScan: boolean }) {
  const { t, formatDateTime } = useLocale();
  const flag = useFlag();
  const r = useSecretReveal();

  // A 403 (no hosts.secrets.reveal) lands here, as an error flag.
  useEffect(() => {
    if (r.error) flag({ appearance: "error", title: t("host.secrets.revealError"), description: r.error });
  }, [r.error]); // eslint-disable-line react-hooks/exhaustive-deps

  return (
    <Card hover={false} padding="sm">
      <div className="flex flex-wrap items-center gap-3 text-xs">
        <div className="min-w-0 flex-1">
          <div className="font-mono font-medium text-[var(--text-primary)] truncate">{secret.name}</div>
          {secret.description && <div className="text-2xs text-[var(--text-muted)] truncate" title={secret.description}>{secret.description}</div>}
        </div>
        <span className="inline-flex items-center gap-1.5 text-[var(--text-secondary)]">
          <StatusDot color={inScan ? "success" : "muted"} />
          {inScan ? t("host.secrets.inScan") : t("host.secrets.gone")}
        </span>
        <span className="text-2xs text-[var(--text-faint)]">{t("host.secrets.updated", { when: formatDateTime(secret.updated_at) })}</span>
        <PillButton
          active={r.revealed}
          disabled={r.loading}
          onClick={() => (r.revealed ? r.hide() : r.reveal(secret.id))}
          lead={<Icon path={ICON_PATHS.lock} className="w-3 h-3" />}
        >
          {r.revealed ? t("host.secrets.hide") : t("host.secrets.reveal")}
        </PillButton>
      </div>
      {r.revealed && r.value != null && (
        <div className="mt-2 flex items-center gap-2">
          <code className="min-w-0 flex-1 break-all rounded bg-[var(--bg-elevated)] px-2 py-1 font-mono text-xs text-[var(--text-primary)]">{r.value}</code>
          <PillButton active={r.copyState === "copied"} onClick={() => r.copy()}>
            {r.copyState === "copied" ? t("host.secrets.copied") : t("host.secrets.copy")}
          </PillButton>
        </div>
      )}
    </Card>
  );
}
