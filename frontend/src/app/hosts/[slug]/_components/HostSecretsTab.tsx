"use client";

import { useEffect, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import type { Secret, CredentialSource } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
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

/** {{secret:<id>}} → id, else null. */
function tokenId(token?: string): number | null {
  const m = token?.match(/^\{\{secret:(\d+)\}\}$/);
  return m ? Number(m[1]) : null;
}

/** credential_sources from the latest scan JSON; null = the probe didn't run. */
function parseSources(scanText: string): CredentialSource[] | null {
  if (!scanText) return null;
  try {
    const d = JSON.parse(scanText) as { credential_sources?: CredentialSource[] | null };
    return Array.isArray(d.credential_sources) ? d.credential_sources : null;
  } catch {
    return null;
  }
}

/**
 * Secrets the scan found on this host (vault, group `scan-*`): a section for
 * command lines (today's rows) and one for config files (credsources). A value
 * lives only in the vault; the admin tier (`scan-admin-*`) reveals to admins only.
 */
export default function HostSecretsTab({ hostId, scanText }: { hostId: number; scanText: string }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const hasReveal = isAdmin || !!user?.permissions?.includes("hosts.secrets.reveal");
  const { data, isLoading, error } = useQuery({
    queryKey: ["host-scan-secrets", hostId],
    // No type filter: command-line (env_var) and config-file (cred/sshkey/env_var) rows both.
    queryFn: () => secretsAPI.list({ scope: "host", parent_id: hostId }),
  });

  const { cmdSecrets, sources, goneConfig } = useMemo(() => {
    const secrets = (data ?? []).filter((s) => s.group_label?.startsWith("scan-"));
    const srcs = parseSources(scanText);
    const configIds = new Set<number>();
    for (const s of srcs ?? []) {
      const id = tokenId(s.token);
      if (id) configIds.add(id);
    }
    return {
      sources: srcs,
      // Command lines: env_var rows the scan embeds in a command, not a config source.
      cmdSecrets: secrets.filter((s) => s.type === "env_var" && !configIds.has(s.id)),
      // Config logins (cred/sshkey) no longer in the latest scan: the file is gone.
      goneConfig: secrets.filter((s) => (s.type === "cred" || s.type === "sshkey") && !configIds.has(s.id)),
    };
  }, [data, scanText]);

  const byId = useMemo(
    () => new Map((data ?? []).map((s) => [s.id, s] as const)),
    [data],
  );

  return (
    <div className="space-y-5 animate-fade-in">
      {error && (
        <StatusAlert variant="error">
          {t("host.secrets.loadError")}: {(error as Error).message}
        </StatusAlert>
      )}

      {/* Command lines (existing) */}
      <section className="space-y-3">
        <SectionHeading variant="section" icon={ICON_PATHS.lock} count={cmdSecrets.length} hint={t("host.secrets.hint")}>
          {t("host.secrets.sectionCmd")}
        </SectionHeading>
        {!isLoading && cmdSecrets.length === 0 ? (
          <EmptyState compact icon="search" title={t("host.secrets.empty")} />
        ) : (
          <div className="space-y-2">
            {cmdSecrets.map((s) => (
              <SecretRow key={s.id} secret={s} inScan={scanText.includes(`{{secret:${s.id}}}`)} canReveal={hasReveal} />
            ))}
          </div>
        )}
      </section>

      {/* Config files (new) */}
      <section className="space-y-3">
        <SectionHeading
          variant="section"
          icon={ICON_PATHS.document}
          count={(sources?.length ?? 0) + goneConfig.length}
          hint={t("host.secrets.configHint")}
        >
          {t("host.secrets.sectionConfig")}
        </SectionHeading>
        {sources === null ? (
          <EmptyState compact icon="key" title={t("host.secrets.noRoot")} />
        ) : sources.length === 0 && goneConfig.length === 0 ? (
          <EmptyState compact icon="search" title={t("host.secrets.configEmpty")} />
        ) : (
          <div className="space-y-2">
            {sources.map((src, i) => (
              <ConfigRow
                key={`${src.path}:${src.locator}:${i}`}
                source={src}
                secret={byId.get(tokenId(src.token) ?? -1)}
                isAdmin={isAdmin}
                hasReveal={hasReveal}
              />
            ))}
            {goneConfig.map((s) => (
              <ConfigRow key={`gone-${s.id}`} secret={s} isAdmin={isAdmin} hasReveal={hasReveal} gone />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function SecretRow({ secret, inScan, canReveal }: { secret: Secret; inScan: boolean; canReveal: boolean }) {
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
        {canReveal && (
          <PillButton
            active={r.revealed}
            disabled={r.loading}
            onClick={() => (r.revealed ? r.hide() : r.reveal(secret.id))}
            lead={<Icon path={ICON_PATHS.lock} className="w-3 h-3" />}
          >
            {r.revealed ? t("host.secrets.hide") : t("host.secrets.reveal")}
          </PillButton>
        )}
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

/**
 * One config-file credential. `source` is its scan provenance (detector, path,
 * locator, field names); `secret` the vault row (for reveal + change date).
 * A gone row has no source. Field NAMES only — the value lives in the vault.
 */
function ConfigRow({
  source,
  secret,
  isAdmin,
  hasReveal,
  gone = false,
}: {
  source?: CredentialSource;
  secret?: Secret;
  isAdmin: boolean;
  hasReveal: boolean;
  gone?: boolean;
}) {
  const { t, formatDateTime } = useLocale();
  const flag = useFlag();
  const r = useSecretReveal();

  useEffect(() => {
    if (r.error) flag({ appearance: "error", title: t("host.secrets.revealError"), description: r.error });
  }, [r.error]); // eslint-disable-line react-hooks/exhaustive-deps

  const detector = source?.detector ?? "";
  const label = detector ? t(`host.secrets.detector.${detector}`) : secret?.description ?? secret?.name ?? "";
  const path = source?.path ?? secret?.description ?? "";
  const admin = source?.admin ?? secret?.group_label?.startsWith("scan-admin-") ?? false;
  const fieldNames = Object.keys(source?.fields ?? {});
  const indirect = !!source && !tokenId(source.token);
  const changed = secret && secret.updated_at !== secret.created_at;

  // Admin tier reveals to admins only; the rest keeps hosts.secrets.reveal.
  const canReveal = !!secret && !indirect && (isAdmin || (!admin && hasReveal));
  const lockOnly = !!secret && !indirect && admin && !isAdmin;

  return (
    <Card hover={false} padding="sm">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="font-medium text-[var(--text-primary)] truncate">{label}</span>
            {admin && (
              <span className="inline-flex items-center gap-1 rounded-full bg-[var(--warning)]/10 px-1.5 py-0.5 text-2xs text-[var(--warning)]">
                <Icon path={ICON_PATHS.lock} className="w-2.5 h-2.5" />
                {t("host.secrets.adminOnly")}
              </span>
            )}
          </div>
          <div className="font-mono text-2xs text-[var(--text-muted)] truncate" title={path}>
            {path}
            {source?.locator ? ` [${source.locator}]` : ""}
          </div>
          {fieldNames.length > 0 && (
            <div className="mt-0.5 flex flex-wrap gap-1">
              {fieldNames.map((f) => (
                <span key={f} className="rounded bg-[var(--bg-elevated)] px-1.5 py-0.5 font-mono text-2xs text-[var(--text-secondary)]">{f}</span>
              ))}
            </div>
          )}
        </div>
        <span className="inline-flex items-center gap-1.5 text-[var(--text-secondary)]">
          <StatusDot color={gone ? "muted" : "success"} />
          {gone ? t("host.secrets.gone") : t("host.secrets.inScan")}
        </span>
        {changed && (
          <span className="text-2xs text-[var(--text-faint)]">{t("host.secrets.updated", { when: formatDateTime(secret!.updated_at) })}</span>
        )}
        {indirect && <span className="text-2xs text-[var(--text-faint)]">{t("host.secrets.indirect")}</span>}
        {lockOnly && (
          <span className="inline-flex items-center gap-1 text-2xs text-[var(--text-muted)]">
            <Icon path={ICON_PATHS.lock} className="w-3 h-3" />
            {t("host.secrets.adminOnly")}
          </span>
        )}
        {canReveal && (
          <PillButton
            active={r.revealed}
            disabled={r.loading}
            onClick={() => (r.revealed ? r.hide() : r.reveal(secret!.id))}
            lead={<Icon path={ICON_PATHS.lock} className="w-3 h-3" />}
          >
            {r.revealed ? t("host.secrets.hide") : t("host.secrets.reveal")}
          </PillButton>
        )}
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
