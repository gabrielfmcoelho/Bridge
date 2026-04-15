"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { coolifyAPI } from "@/lib/api";
import type { CoolifyServer } from "@/lib/api";
import Button from "@/components/ui/Button";

type Props = {
  slug: string;
  coolifyUUID?: string | null;
  available: boolean;
  t: (key: string) => string;
  isAdmin: boolean;
};

export default function CoolifyIntegration({ slug, coolifyUUID, available, t, isAdmin }: Props) {
  const queryClient = useQueryClient();
  const [server, setServer] = useState<CoolifyServer | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["host", slug] });

  const checkMutation = useMutation({
    mutationFn: () => coolifyAPI.checkHost(slug),
    onSuccess: (data) => {
      if (data.found && data.server) {
        setServer(data.server);
        setMessage({ type: "success", text: t("operation.coolifyFound") });
        invalidate();
      } else {
        // Not found — auto-register
        setMessage({ type: "success", text: t("operation.coolifyNotFound") + " " + t("operation.coolifyRegistering") });
        registerMutation.mutate();
      }
    },
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const registerMutation = useMutation({
    mutationFn: () => coolifyAPI.registerHost(slug),
    onSuccess: () => {
      setMessage({ type: "success", text: t("operation.coolifyRegistered") });
      setServer(null);
      invalidate();
    },
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const validateMutation = useMutation({
    mutationFn: () => coolifyAPI.validateHost(slug),
    onSuccess: () => setMessage({ type: "success", text: t("operation.coolifyValidated") }),
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const syncMutation = useMutation({
    mutationFn: () => coolifyAPI.syncHost(slug),
    onSuccess: () => setMessage({ type: "success", text: t("operation.coolifySynced") }),
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const deleteMutation = useMutation({
    mutationFn: () => coolifyAPI.deleteHost(slug),
    onSuccess: () => {
      setMessage({ type: "success", text: t("operation.coolifyDeleted") });
      setServer(null);
      invalidate();
    },
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const statusQuery = useQuery({
    queryKey: ["coolify-server-status", slug],
    queryFn: () => coolifyAPI.getServerStatus(slug),
    enabled: !!coolifyUUID,
    staleTime: 30_000,
  });

  const linked = !!coolifyUUID;
  const loading = checkMutation.isPending || registerMutation.isPending || validateMutation.isPending || syncMutation.isPending || deleteMutation.isPending;
  const sv = statusQuery.data?.server ?? server;

  return (
    <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
      <div className="px-4 py-3 space-y-3">
        {/* Header */}
        <div className="flex items-center gap-2">
          <svg className="w-4 h-4 text-violet-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
          </svg>
          <span className="text-sm font-medium text-[var(--text-primary)]">Coolify</span>
          {linked && (
            <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-violet-500/15 text-[10px] text-violet-400 border border-violet-500/20">
              {coolifyUUID}
            </span>
          )}
          {!available && (
            <span className="text-[10px] text-[var(--text-faint)] ml-auto">{t("operation.coolifyDisabled")}</span>
          )}
        </div>

        {!available ? (
          <p className="text-xs text-[var(--text-muted)]">{t("operation.coolifyDisabledDesc")}</p>
        ) : linked ? (
          /* Server is linked — show status + actions */
          <div className="space-y-2">
            {sv && (
              <div className="flex items-center gap-2 text-xs flex-wrap">
                <span className="text-[var(--text-muted)]">{sv.name} ({sv.ip}:{sv.port})</span>
                <span className={`w-2 h-2 rounded-full shrink-0 ${sv.is_reachable ? "bg-emerald-400" : "bg-red-400"}`} title={sv.is_reachable ? "Reachable" : "Unreachable"} />
                <span className={`text-[10px] ${sv.is_reachable ? "text-emerald-400" : "text-red-400"}`}>
                  {sv.is_reachable ? "reachable" : "unreachable"}
                </span>
                <span className={`w-2 h-2 rounded-full shrink-0 ${sv.is_usable ? "bg-emerald-400" : "bg-amber-400"}`} title={sv.is_usable ? "Usable" : "Not usable"} />
                <span className={`text-[10px] ${sv.is_usable ? "text-emerald-400" : "text-amber-400"}`}>
                  {sv.is_usable ? "usable" : "not usable"}
                </span>
              </div>
            )}
            <div className="flex items-center gap-2 flex-wrap">
              <Button size="sm" variant="secondary" onClick={() => queryClient.invalidateQueries({ queryKey: ["coolify-server-status", slug] })} loading={statusQuery.isFetching} disabled={loading}>
                {t("operation.coolifyStatus")}
              </Button>
              <Button size="sm" variant="secondary" onClick={() => validateMutation.mutate()} loading={validateMutation.isPending} disabled={loading}>
                {t("operation.coolifyValidate")}
              </Button>
              <Button size="sm" variant="secondary" onClick={() => syncMutation.mutate()} loading={syncMutation.isPending} disabled={loading}>
                {t("operation.coolifySync")}
              </Button>
              {isAdmin && (
                <Button size="sm" variant="secondary" onClick={() => { if (confirm(t("operation.coolifyDeleteConfirm"))) deleteMutation.mutate(); }} loading={deleteMutation.isPending} disabled={loading}>
                  {t("operation.coolifyDelete")}
                </Button>
              )}
            </div>
          </div>
        ) : (
          /* Initial state — offer check */
          <div className="flex items-center gap-2">
            <p className="text-xs text-[var(--text-muted)] flex-1">{t("operation.coolifyCheckDesc")}</p>
            <Button size="sm" variant="secondary" onClick={() => checkMutation.mutate()} loading={checkMutation.isPending} disabled={loading}>
              {t("operation.coolifyCheck")}
            </Button>
          </div>
        )}

        {/* Inline message */}
        {message && (
          <div className={`rounded-[var(--radius-sm)] px-3 py-2 text-xs ${
            message.type === "success"
              ? "bg-emerald-500/10 border border-emerald-500/25 text-emerald-400"
              : "bg-red-500/10 border border-red-500/25 text-red-400"
          }`}>
            {message.text}
          </div>
        )}
      </div>
    </div>
  );
}
