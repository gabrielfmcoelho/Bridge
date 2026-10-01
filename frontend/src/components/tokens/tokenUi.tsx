"use client";

import { useQuery } from "@tanstack/react-query";
import { apiTokensAPI, type APIToken } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Modal from "@/components/ui/Modal";
import StatusAlert from "@/components/ui/StatusAlert";

/** A personal token's state, computed like the server does (revoked wins over expired). */
export function tokenState(tok: APIToken): "active" | "expired" | "revoked" {
  if (tok.revoked_at) return "revoked";
  if (tok.expires_at && new Date(tok.expires_at) <= new Date()) return "expired";
  return "active";
}

export const TOKEN_STATE_APPEARANCE = { active: "success", expired: "moved", revoked: "removed" } as const;

// TokenUsageModal shows a token's requests per day for the last 30 days.
export function TokenUsageModal({ token, onClose }: { token: APIToken; onClose: () => void }) {
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
