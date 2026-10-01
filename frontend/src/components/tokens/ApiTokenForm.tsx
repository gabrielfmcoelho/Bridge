"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiTokensAPI, usersAPI } from "@/lib/api";
import type { ApiKeyScope } from "@/lib/types";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import FormError from "@/components/ui/FormError";
import ScopePicker from "@/components/tokens/ScopePicker";

const EXPIRY_DAYS = [30, 90, 365, 0]; // 0 = never
const ROLE_RANK: Record<string, number> = { viewer: 0, editor: 1, admin: 2 };

// Issue a personal brg_ token (Settings → Tokens de API and Bridge's own API
// page). The plaintext comes back once through onCreated.
export default function ApiTokenForm({ onCreated }: { onCreated: (created: { token: string; name: string }) => void }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";

  const [name, setName] = useState("");
  const [expiresInDays, setExpiresInDays] = useState(90);
  const [scopes, setScopes] = useState<string[]>([]);
  const [rateLimit, setRateLimit] = useState("");
  const [ownerId, setOwnerId] = useState<number | null>(null); // null = me
  const [attempted, setAttempted] = useState(false);
  const [error, setError] = useState("");

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
      onCreated({ token: res.token, name: res.api_token.name });
      setName("");
      setScopes([]);
      setRateLimit("");
      setAttempted(false);
      setError("");
      queryClient.invalidateQueries({ queryKey: ["api-tokens"] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : t("settings.apiTokens.createError")),
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

  return (
    <div className="space-y-4">
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
  );
}
