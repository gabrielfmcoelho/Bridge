"use client";

import { useState } from "react";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { coolifyAPI, sshKeysAPI } from "@/lib/api";
import type { CoolifyServer } from "@/lib/api";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

type Props = {
  slug: string;
  coolifyUUID?: string | null;
  available: boolean;
  t: (key: string) => string;
  isAdmin: boolean;
};

export default function CoolifyIntegration({ slug, coolifyUUID, available, t, isAdmin }: Props) {
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const [server, setServer] = useState<CoolifyServer | null>(null);
  const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
  // Empty string == "auto": backend picks from host_remote_users or host key.
  const [selectedKeyId, setSelectedKeyId] = useState<string>("");

  const keysQuery = useQuery({
    queryKey: ["ssh-keys"],
    queryFn: () => sshKeysAPI.list(),
    enabled: available,
    staleTime: 60_000,
  });
  const eligibleKeys = (keysQuery.data ?? []).filter(k => k.credential_type === "key" && k.has_private_key);

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
    mutationFn: () => coolifyAPI.registerHost(slug, selectedKeyId ? parseInt(selectedKeyId, 10) : undefined),
    onSuccess: () => {
      setMessage({ type: "success", text: t("operation.coolifyRegistered") });
      setServer(null);
      invalidate();
    },
    onError: (err: Error) => setMessage({ type: "error", text: err.message }),
  });

  const updateKeyMutation = useMutation({
    mutationFn: () => {
      const id = parseInt(selectedKeyId, 10);
      if (!id) throw new Error("no key selected");
      return coolifyAPI.updateServerKey(slug, id);
    },
    onSuccess: () => {
      setMessage({ type: "success", text: t("operation.coolifyKeyUpdated") });
      queryClient.invalidateQueries({ queryKey: ["coolify-server-status", slug] });
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
  const loading = checkMutation.isPending || registerMutation.isPending || validateMutation.isPending || syncMutation.isPending || deleteMutation.isPending || updateKeyMutation.isPending;
  const sv = statusQuery.data?.server ?? server;

  const keyPicker = (
    <label className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
      <span className="shrink-0">{t("operation.coolifyKeyPickerLabel")}</span>
      <select
        value={selectedKeyId}
        onChange={(e) => setSelectedKeyId(e.target.value)}
        disabled={loading || eligibleKeys.length === 0}
        className="flex-1 min-w-0 bg-[var(--bg-input)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2 py-1 text-xs text-[var(--text-primary)]"
      >
        <option value="">{t("operation.coolifyKeyPickerAuto")}</option>
        {eligibleKeys.map(k => (
          <option key={k.id} value={k.id.toString()}>
            {k.name}{k.fingerprint ? ` · ${k.fingerprint.slice(0, 24)}…` : ""}
          </option>
        ))}
      </select>
    </label>
  );

  return (
    // A row of the Integrations SectionCard (flush body): brings its own px-5.
    <div>
      <div className="px-5 py-4 space-y-3">
        {/* Header */}
        <div className="flex items-center gap-2">
          <Icon path={ICON_PATHS.serverStack} className="w-4 h-4 text-[var(--accent)] shrink-0" />
          <span className="text-sm font-medium text-[var(--text-primary)]">Coolify</span>
          {linked && (
            <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-[var(--accent)]/15 text-2xs text-[var(--accent)] border border-[var(--accent)]/20">
              {coolifyUUID}
            </span>
          )}
          {!available && (
            <span className="text-2xs text-[var(--text-faint)] ml-auto">{t("operation.coolifyDisabled")}</span>
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
                <span className={`w-2 h-2 rounded-full shrink-0 ${sv.is_reachable ? "bg-[var(--success)]" : "bg-[var(--danger)]"}`} title={sv.is_reachable ? t("operation.coolifyReachable") : t("operation.coolifyUnreachable")} />
                <span className={`text-2xs ${sv.is_reachable ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
                  {(sv.is_reachable ? t("operation.coolifyReachable") : t("operation.coolifyUnreachable")).toLowerCase()}
                </span>
                <span className={`w-2 h-2 rounded-full shrink-0 ${sv.is_usable ? "bg-[var(--success)]" : "bg-[var(--warning)]"}`} title={sv.is_usable ? t("operation.coolifyUsable") : t("operation.coolifyNotUsable")} />
                <span className={`text-2xs ${sv.is_usable ? "text-[var(--success)]" : "text-[var(--warning)]"}`}>
                  {(sv.is_usable ? t("operation.coolifyUsable") : t("operation.coolifyNotUsable")).toLowerCase()}
                </span>
              </div>
            )}
            {isAdmin && (eligibleKeys.length > 0 ? keyPicker : (
              <p className="text-xs text-[var(--text-faint)]">{t("operation.coolifyKeyPickerEmpty")}</p>
            ))}
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
                <Button size="sm" variant="secondary" onClick={() => updateKeyMutation.mutate()} loading={updateKeyMutation.isPending} disabled={loading || !selectedKeyId}>
                  {t("operation.coolifyUpdateKey")}
                </Button>
              )}
              {isAdmin && (
                <Button size="sm" variant="secondary" onClick={async () => { if (await confirm({ title: t("operation.coolifyDeleteConfirm"), danger: true, confirmLabel: t("common.remove") })) deleteMutation.mutate(); }} loading={deleteMutation.isPending} disabled={loading}>
                  {t("operation.coolifyDelete")}
                </Button>
              )}
            </div>
          </div>
        ) : (
          /* Initial state — offer check (+ optional key override) */
          <div className="space-y-2">
            <p className="text-xs text-[var(--text-muted)]">{t("operation.coolifyCheckDesc")}</p>
            {eligibleKeys.length > 0 ? keyPicker : (
              <p className="text-xs text-[var(--text-faint)]">{t("operation.coolifyKeyPickerEmpty")}</p>
            )}
            <div className="flex justify-end">
              <Button size="sm" variant="secondary" onClick={() => checkMutation.mutate()} loading={checkMutation.isPending} disabled={loading}>
                {t("operation.coolifyCheck")}
              </Button>
            </div>
          </div>
        )}

        {/* Inline message */}
        {message && (
          <div className={`rounded-[var(--radius-sm)] px-3 py-2 text-xs ${
            message.type === "success"
              ? "bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)]"
              : "bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]"
          }`}>
            {message.text}
          </div>
        )}
      </div>
    </div>
  );
}
