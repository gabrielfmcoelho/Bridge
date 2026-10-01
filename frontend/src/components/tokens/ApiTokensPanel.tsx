"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiTokensAPI, type APIToken } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import Lozenge from "@/components/ui/Lozenge";
import RowActions from "@/components/ui/RowActions";
import StatusAlert from "@/components/ui/StatusAlert";
import Toggle from "@/components/ui/Toggle";
import ApiTokenForm from "@/components/tokens/ApiTokenForm";
import { TOKEN_STATE_APPEARANCE, TokenUsageModal, tokenState } from "@/components/tokens/tokenUi";
import { ICON_PATHS } from "@/lib/icon-paths";

const NOTICE_KEY = "bridge.tokens.scopesNotice";

function readNotice(): boolean {
  try { return localStorage.getItem(NOTICE_KEY) !== "dismissed"; } catch { return true; }
}

// API tokens (Settings → Tokens de API): scripts send "Authorization: Bearer
// brg_…" and act as the token's owner, narrowed to the token's scopes. Every
// user manages their own; admins also see everyone's and issue tokens for
// service accounts. Bridge's own API page lists them too, merged with its
// Keycloak keys (atlas ApiKeysTab).
export default function ApiTokensPanel() {
  const { t, formatDate, formatDateTime } = useLocale();
  const { user } = useAuth();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";

  const [showAll, setShowAll] = useState(false);
  const [created, setCreated] = useState<{ token: string; name: string } | null>(null);
  const [usageOf, setUsageOf] = useState<APIToken | null>(null);
  const [notice, setNotice] = useState(readNotice);

  const all = isAdmin && showAll;
  const { data: tokens = [], isLoading, error: loadError } = useQuery({
    queryKey: ["api-tokens", all],
    queryFn: () => apiTokensAPI.list(all),
  });

  const revokeMutation = useMutation({
    mutationFn: apiTokensAPI.revoke,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
      flag({ appearance: "success", title: t("settings.apiTokens.revoked") });
    },
    onError: (err) => flag({ appearance: "error", title: t("settings.apiTokens.revokeError"), description: err instanceof Error ? err.message : undefined }),
  });

  const handleRevoke = async (tok: APIToken) => {
    const ok = await confirm({
      title: t("settings.apiTokens.revokeConfirm", { name: tok.name }),
      message: t("settings.apiTokens.revokeConfirmBody"),
      danger: true,
      confirmLabel: t("settings.apiTokens.revoke"),
    });
    if (ok) revokeMutation.mutate(tok.id);
  };

  const dismissNotice = () => {
    setNotice(false);
    try { localStorage.setItem(NOTICE_KEY, "dismissed"); } catch { /* per-viewer convenience only */ }
  };

  return (
    <SectionCard
      as="h3"
      title={t("settings.apiTokens.title")}
      description={t("settings.apiTokens.intro")}
      controls={isAdmin ? (
        <label className="inline-flex items-center gap-2 text-xs text-[var(--text-secondary)]">
          <Toggle checked={showAll} onChange={setShowAll} ariaLabel={t("settings.apiTokens.showAll")} />
          {t("settings.apiTokens.showAll")}
        </label>
      ) : undefined}
    >
      {notice && (
        <div className="mb-4 space-y-2">
          <StatusAlert variant="info">{t("settings.apiTokens.scopesNotice")}</StatusAlert>
          <Button size="sm" variant="ghost" onClick={dismissNotice}>{t("settings.apiTokens.dismiss")}</Button>
        </div>
      )}

      {created && (
        <div className="mb-4 space-y-3">
          <StatusAlert variant="warning">{t("settings.apiTokens.copyNow", { name: created.name })}</StatusAlert>
          <div className="flex flex-col sm:flex-row gap-2 sm:items-center">
            <code className="flex-1 min-w-0 break-all font-mono text-xs px-3 py-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)] text-[var(--text-primary)]">
              {created.token}
            </code>
            <div className="flex gap-2 shrink-0">
              <CopyButton value={created.token} icon />
              <Button variant="ghost" onClick={() => setCreated(null)}>{t("settings.apiTokens.done")}</Button>
            </div>
          </div>
          <p className="text-xs text-[var(--text-muted)]">
            {t("settings.apiTokens.usage")} <code className="font-mono">Authorization: Bearer {created.token.slice(0, 10)}…</code>
          </p>
        </div>
      )}

      {loadError ? (
        <StatusAlert variant="error">{(loadError as Error).message || t("settings.apiTokens.loadError")}</StatusAlert>
      ) : isLoading ? (
        <div className="animate-pulse space-y-3 mb-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-10 bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" />
          ))}
        </div>
      ) : tokens.length === 0 ? (
        <div className="text-center py-6 text-sm text-[var(--text-muted)] mb-4">{t("settings.apiTokens.empty")}</div>
      ) : (
        <div className="overflow-x-auto mb-4">
          <table className={tableClasses.compact.table}>
            <thead>
              <tr className={tableClasses.compact.headRow}>
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.name")}</th>
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.token")}</th>
                {all && <th className={tableClasses.compact.th}>{t("settings.apiTokens.owner")}</th>}
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.scopes")}</th>
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.lastUsed")}</th>
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.expiresAt")}</th>
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.status")}</th>
                <th className="w-10" />
              </tr>
            </thead>
            <tbody>
              {tokens.map((tok) => {
                const state = tokenState(tok);
                return (
                  <tr key={tok.id} className={tableClasses.compact.row}>
                    <td className={tableClasses.compact.td}>
                      {tok.name}
                      {tok.owner_kind === "service" && !all && <span className="ml-2"><Lozenge appearance="new">{t("settings.apiTokens.serviceAccount")}</Lozenge></span>}
                    </td>
                    <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)]`}>{tok.prefix}…</td>
                    {all && (
                      <td className={tableClasses.compact.td}>
                        {tok.username}
                        {tok.owner_kind === "service" && <span className="ml-2"><Lozenge appearance="new">{t("settings.apiTokens.serviceAccount")}</Lozenge></span>}
                      </td>
                    )}
                    <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)] max-w-[16rem] truncate`} title={tok.scopes.join(", ")}>
                      {tok.scopes.length ? tok.scopes.join(", ") : t("settings.apiTokens.noScopes")}
                    </td>
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>
                      {tok.last_used_at ? formatDateTime(tok.last_used_at) : t("settings.apiTokens.neverUsed")}
                      {tok.today_requests > 0 && <span className="block text-2xs text-[var(--text-muted)]">{t("settings.apiTokens.todayRequests", { count: String(tok.today_requests) })}</span>}
                    </td>
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>
                      {tok.expires_at ? formatDate(tok.expires_at) : t("settings.apiTokens.noExpiry")}
                    </td>
                    <td className={tableClasses.compact.td}>
                      <Lozenge appearance={TOKEN_STATE_APPEARANCE[state]}>{t(`settings.apiTokens.state.${state}`)}</Lozenge>
                    </td>
                    <td className={tableClasses.compact.td}>
                      <RowActions
                        name={tok.name}
                        actions={[
                          { label: t("settings.apiTokens.usageAction"), icon: ICON_PATHS.clock, onClick: () => setUsageOf(tok) },
                          { label: t("settings.apiTokens.revoke"), icon: ICON_PATHS.trash, danger: true, hidden: state !== "active", onClick: () => handleRevoke(tok) },
                        ]}
                      />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="border-t border-[var(--border-subtle)] pt-4 space-y-4">
        <p className="text-xs font-semibold text-[var(--text-muted)]">{t("settings.apiTokens.new")}</p>
        <ApiTokenForm onCreated={setCreated} />
      </div>

      {usageOf && <TokenUsageModal token={usageOf} onClose={() => setUsageOf(null)} />}
    </SectionCard>
  );
}
