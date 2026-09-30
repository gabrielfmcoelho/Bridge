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
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import Lozenge from "@/components/ui/Lozenge";
import RowActions from "@/components/ui/RowActions";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import Toggle from "@/components/ui/Toggle";
import { ICON_PATHS } from "@/lib/icon-paths";

const EXPIRY_DAYS = [30, 90, 365, 0]; // 0 = never

function tokenState(tok: APIToken): "active" | "expired" | "revoked" {
  if (tok.revoked_at) return "revoked";
  if (tok.expires_at && new Date(tok.expires_at) <= new Date()) return "expired";
  return "active";
}

const STATE_APPEARANCE = { active: "success", expired: "moved", revoked: "removed" } as const;

// Personal API tokens: scripts send "Authorization: Bearer brg_…" and act as
// the token's owner. Every user manages their own; admins can switch to
// everyone's to revoke a leaked one.
export default function ApiTokensTab() {
  const { t, formatDate, formatDateTime } = useLocale();
  const { user } = useAuth();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";

  const [showAll, setShowAll] = useState(false);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState(90);
  const [error, setError] = useState("");
  const [created, setCreated] = useState<{ token: string; name: string } | null>(null);

  const all = isAdmin && showAll;
  const { data: tokens = [], isLoading, error: loadError } = useQuery({
    queryKey: ["api-tokens", all],
    queryFn: () => apiTokensAPI.list(all),
  });

  const createMutation = useMutation({
    mutationFn: apiTokensAPI.create,
    onSuccess: (res) => {
      setCreated({ token: res.token, name: res.api_token.name });
      setName("");
      setError("");
      queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : t("settings.apiTokens.createError")),
  });

  const revokeMutation = useMutation({
    mutationFn: apiTokensAPI.revoke,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
      flag({ appearance: "success", title: t("settings.apiTokens.revoked") });
    },
    onError: (err) => flag({ appearance: "error", title: t("settings.apiTokens.revokeError"), description: err instanceof Error ? err.message : undefined }),
  });

  const handleCreate = () => {
    if (!name.trim()) {
      setError(t("settings.apiTokens.nameRequired"));
      return;
    }
    createMutation.mutate({ name: name.trim(), expires_in_days: expiresInDays });
  };

  const handleRevoke = async (tok: APIToken) => {
    const ok = await confirm({
      title: t("settings.apiTokens.revokeConfirm", { name: tok.name }),
      message: t("settings.apiTokens.revokeConfirmBody"),
      danger: true,
      confirmLabel: t("settings.apiTokens.revoke"),
    });
    if (ok) revokeMutation.mutate(tok.id);
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
                <th className={tableClasses.compact.th}>{t("settings.apiTokens.createdAt")}</th>
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
                    <td className={tableClasses.compact.td}>{tok.name}</td>
                    <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)]`}>{tok.prefix}…</td>
                    {all && <td className={tableClasses.compact.td}>{tok.username}</td>}
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>{formatDate(tok.created_at)}</td>
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>
                      {tok.last_used_at ? formatDateTime(tok.last_used_at) : t("settings.apiTokens.neverUsed")}
                    </td>
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>
                      {tok.expires_at ? formatDate(tok.expires_at) : t("settings.apiTokens.noExpiry")}
                    </td>
                    <td className={tableClasses.compact.td}>
                      <Lozenge appearance={STATE_APPEARANCE[state]}>{t(`settings.apiTokens.state.${state}`)}</Lozenge>
                    </td>
                    <td className={tableClasses.compact.td}>
                      <RowActions
                        name={tok.name}
                        actions={[{
                          label: t("settings.apiTokens.revoke"),
                          icon: ICON_PATHS.trash,
                          danger: true,
                          hidden: state !== "active",
                          onClick: () => handleRevoke(tok),
                        }]}
                      />
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="border-t border-[var(--border-subtle)] pt-4 space-y-3">
        <p className="text-xs font-semibold text-[var(--text-muted)]">{t("settings.apiTokens.new")}</p>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 items-end">
          <Input
            label={t("settings.apiTokens.name")}
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
            placeholder={t("settings.apiTokens.namePlaceholder")}
          />
          <NativeSelect
            label={t("settings.apiTokens.expiry")}
            value={expiresInDays}
            onChange={(e) => setExpiresInDays(Number(e.target.value))}
          >
            {EXPIRY_DAYS.map((d) => (
              <option key={d} value={d}>
                {d === 0 ? t("settings.apiTokens.noExpiry") : t("settings.apiTokens.days", { n: String(d) })}
              </option>
            ))}
          </NativeSelect>
          <Button onClick={handleCreate} loading={createMutation.isPending} disabled={!name.trim()}>
            {t("settings.apiTokens.create")}
          </Button>
        </div>
        <FormError message={error} />
      </div>
    </SectionCard>
  );
}
