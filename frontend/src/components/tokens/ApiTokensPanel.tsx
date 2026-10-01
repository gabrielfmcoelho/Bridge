"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiTokensAPI, usersAPI, type APIToken } from "@/lib/api";
import type { ApiKeyScope } from "@/lib/types";
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
import Modal from "@/components/ui/Modal";
import RowActions from "@/components/ui/RowActions";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import Toggle from "@/components/ui/Toggle";
import ScopePicker from "@/components/tokens/ScopePicker";
import { ICON_PATHS } from "@/lib/icon-paths";

const EXPIRY_DAYS = [30, 90, 365, 0]; // 0 = never
const ROLE_RANK: Record<string, number> = { viewer: 0, editor: 1, admin: 2 };
const NOTICE_KEY = "bridge.tokens.scopesNotice";

function tokenState(tok: APIToken): "active" | "expired" | "revoked" {
  if (tok.revoked_at) return "revoked";
  if (tok.expires_at && new Date(tok.expires_at) <= new Date()) return "expired";
  return "active";
}

const STATE_APPEARANCE = { active: "success", expired: "moved", revoked: "removed" } as const;

function readNotice(): boolean {
  try { return localStorage.getItem(NOTICE_KEY) !== "dismissed"; } catch { return true; }
}

interface ApiTokensPanelProps {
  /** Overrides the section title / description (Settings uses the defaults). */
  title?: string;
  description?: string;
  /** Shown before each scope (e.g. "bridge:" on Bridge's API page, to read like its Keycloak scopes). */
  scopeDisplayPrefix?: string;
}

// API tokens: scripts send "Authorization: Bearer brg_…" and act as the
// token's owner, narrowed to the token's scopes. Every user manages their own;
// admins also see everyone's and issue tokens for service accounts. Rendered in
// Settings → Tokens de API and on Bridge's own API page (Chaves de acesso).
export default function ApiTokensPanel({ title, description, scopeDisplayPrefix = "" }: ApiTokensPanelProps = {}) {
  const { t, formatDate, formatDateTime } = useLocale();
  const { user } = useAuth();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";

  const [showAll, setShowAll] = useState(false);
  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState(90);
  const [scopes, setScopes] = useState<string[]>([]);
  const [rateLimit, setRateLimit] = useState("");
  const [ownerId, setOwnerId] = useState<number | null>(null); // null = me
  const [attempted, setAttempted] = useState(false);
  const [error, setError] = useState("");
  const [created, setCreated] = useState<{ token: string; name: string } | null>(null);
  const [usageOf, setUsageOf] = useState<APIToken | null>(null);
  const [notice, setNotice] = useState(readNotice);

  const all = isAdmin && showAll;
  const { data: tokens = [], isLoading, error: loadError } = useQuery({
    queryKey: ["api-tokens", all],
    queryFn: () => apiTokensAPI.list(all),
  });
  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: usersAPI.list, enabled: isAdmin });
  const serviceAccounts = users.filter((u) => u.kind === "service");
  const owner = ownerId ? serviceAccounts.find((u) => u.id === ownerId) : user;

  // Mirrors the server's check: a token can't get more than its owner has.
  const isUsable = (s: ApiKeyScope) => {
    if (!owner) return false;
    if (s.permission) return owner.role === "admin" || (!ownerId && !!user?.permissions?.includes(s.permission));
    return (ROLE_RANK[owner.role] ?? 0) >= (ROLE_RANK[s.min_role ?? "viewer"] ?? 0);
  };

  const createMutation = useMutation({
    mutationFn: apiTokensAPI.create,
    onSuccess: (res) => {
      setCreated({ token: res.token, name: res.api_token.name });
      setName("");
      setScopes([]);
      setRateLimit("");
      setAttempted(false);
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
    setAttempted(true);
    if (!name.trim()) {
      setError(t("settings.apiTokens.nameRequired"));
      return;
    }
    if (scopes.length === 0) return;
    setError("");
    createMutation.mutate({
      name: name.trim(),
      expires_in_days: expiresInDays,
      scopes,
      rate_limit_per_minute: rateLimit.trim() ? Number(rateLimit) : undefined,
      user_id: ownerId ?? undefined,
    });
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

  const dismissNotice = () => {
    setNotice(false);
    try { localStorage.setItem(NOTICE_KEY, "dismissed"); } catch { /* per-viewer convenience only */ }
  };

  return (
    <SectionCard
      as="h3"
      title={title ?? t("settings.apiTokens.title")}
      description={description ?? t("settings.apiTokens.intro")}
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
                    <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)] max-w-[16rem] truncate`} title={tok.scopes.map((s) => scopeDisplayPrefix + s).join(", ")}>
                      {tok.scopes.length ? tok.scopes.map((s) => scopeDisplayPrefix + s).join(", ") : t("settings.apiTokens.noScopes")}
                    </td>
                    <td className={`${tableClasses.compact.td} text-[var(--text-secondary)]`}>
                      {tok.last_used_at ? formatDateTime(tok.last_used_at) : t("settings.apiTokens.neverUsed")}
                      {tok.today_requests > 0 && <span className="block text-2xs text-[var(--text-muted)]">{t("settings.apiTokens.todayRequests", { count: String(tok.today_requests) })}</span>}
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
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <Input
            label={t("settings.apiTokens.name")}
            value={name}
            maxLength={100}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("settings.apiTokens.namePlaceholder")}
            required
          />
          {isAdmin && (
            <NativeSelect
              label={t("settings.apiTokens.owner")}
              value={ownerId ?? ""}
              onChange={(e) => { setOwnerId(e.target.value ? Number(e.target.value) : null); setScopes([]); }}
            >
              <option value="">{t("settings.apiTokens.ownerMe")}</option>
              {serviceAccounts.map((u) => <option key={u.id} value={u.id}>{u.display_name || u.username} ({t("settings.apiTokens.serviceAccount")})</option>)}
            </NativeSelect>
          )}
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
          <Input
            label={t("settings.apiTokens.rateLimit")}
            hint={t("settings.apiTokens.rateLimitHint")}
            type="number"
            min={0}
            value={rateLimit}
            onChange={(e) => setRateLimit(e.target.value)}
            placeholder="600"
          />
        </div>
        <ScopePicker
          queryKey={["token-scopes"]}
          load={apiTokensAPI.scopes}
          value={scopes}
          onChange={setScopes}
          isUsable={isUsable}
          error={attempted && scopes.length === 0 ? t("settings.apiTokens.scopesRequired") : undefined}
        />
        <div className="flex justify-end">
          <Button onClick={handleCreate} loading={createMutation.isPending} disabled={!name.trim()}>
            {t("settings.apiTokens.create")}
          </Button>
        </div>
        <FormError message={error} />
      </div>

      {usageOf && <TokenUsageModal token={usageOf} onClose={() => setUsageOf(null)} />}
    </SectionCard>
  );
}

// TokenUsageModal shows a token's requests per day for the last 30 days.
function TokenUsageModal({ token, onClose }: { token: APIToken; onClose: () => void }) {
  const { t } = useLocale();
  const { data, error, isLoading } = useQuery({
    queryKey: ["api-tokens", "usage", token.id],
    queryFn: () => apiTokensAPI.usage(token.id, 30),
    retry: false,
  });
  const days = Object.entries(data?.daily ?? {}).sort(([a], [b]) => b.localeCompare(a));
  const max = Math.max(1, ...days.map(([, n]) => n));
  return (
    <Modal open onClose={onClose} title={t("settings.apiTokens.usageTitle", { name: token.name })}>
      <div className="space-y-3">
        {isLoading && <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>}
        {!!error && <StatusAlert variant="error">{(error as Error).message}</StatusAlert>}
        {data && (
          <>
            <p className="text-sm text-[var(--text-secondary)]">{t("settings.apiTokens.usageLifetime", { count: String(data.lifetime) })}</p>
            {days.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("settings.apiTokens.usageNone")}</p>
            ) : (
              <ul className="space-y-1">
                {days.map(([d, n]) => (
                  <li key={d} className="flex items-center gap-3 text-xs">
                    <span className="w-12 shrink-0 font-mono text-[var(--text-muted)]">{`${d.slice(8, 10)}/${d.slice(5, 7)}`}</span>
                    <span className="flex-1 h-2 rounded-full bg-[var(--bg-elevated)] overflow-hidden">
                      <span className="block h-full bg-[var(--accent)]" style={{ width: `${(n / max) * 100}%` }} />
                    </span>
                    <span className="w-12 shrink-0 text-right tabular-nums text-[var(--text-secondary)]">{n}</span>
                  </li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
