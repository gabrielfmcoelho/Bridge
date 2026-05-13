"use client";

import { useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import type { Host } from "@/lib/types";
import BatchOperationShell, { type ScopeOption } from "./BatchOperationShell";
import { useBatchRunner } from "./useBatchRunner";

type SetupKeyScope = "all" | "no_key" | "with_key";

export default function BatchSetupKeyModal({
  hosts,
  onClose,
  t,
}: {
  hosts: Host[];
  onClose: () => void;
  t: (key: string) => string;
}) {
  const queryClient = useQueryClient();
  const [concurrency, setConcurrency] = useLocalStorage("hosts.setupKeyConcurrency", 5);
  const [scope, setScope] = useLocalStorage<SetupKeyScope>("hosts.setupKeyScope", "no_key");
  const runner = useBatchRunner();

  // The setup-key endpoint dials with the stored password to copy the new
  // public key into authorized_keys, so a stored password is required.
  const eligible = useMemo(() => hosts.filter(h => h.has_password), [hosts]);
  const noKey = useMemo(() => eligible.filter(h => !h.has_key), [eligible]);
  const withKey = useMemo(() => eligible.filter(h => h.has_key), [eligible]);

  const effectiveScope: SetupKeyScope =
    scope === "no_key" && noKey.length > 0 ? "no_key" :
    scope === "with_key" && withKey.length > 0 ? "with_key" :
    "all";
  const targets = effectiveScope === "no_key" ? noKey : effectiveScope === "with_key" ? withKey : eligible;

  const scopeOptions: ScopeOption[] = [
    { key: "all", label: t("host.scopeAll"), count: eligible.length },
    { key: "no_key", label: t("host.batchKeyScopeNoKey"), count: noKey.length },
    { key: "with_key", label: t("host.batchKeyScopeWithKey"), count: withKey.length },
  ];

  const handleStart = () => {
    runner.start({
      hosts: targets,
      concurrency,
      runOne: async (host) => {
        try {
          const res = await sshAPI.setupKey(host.oficial_slug, { mode: "generate", use_saved_password: true });
          return { success: res.success };
        } catch (err) {
          return { success: false, error: err instanceof Error ? err.message : "Failed" };
        }
      },
    }).then(() => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
    });
  };

  return (
    <BatchOperationShell
      description={t("host.batchKeyDesc")}
      scopeLabel={t("host.scanScope")}
      scope={effectiveScope}
      onScopeChange={(s) => setScope(s as SetupKeyScope)}
      scopeOptions={scopeOptions}
      concurrency={concurrency}
      onConcurrencyChange={setConcurrency}
      concurrencyLabel={t("host.scanConcurrency")}
      targetHosts={targets}
      progress={runner.progress}
      running={runner.running}
      doneCount={runner.doneCount}
      successCount={runner.successCount}
      failedCount={runner.failedCount}
      startLabel={t("host.batchKeyStart")}
      rerunLabel={t("host.rescan")}
      stopLabel={t("host.stopScan")}
      cancelLabel={t("common.cancel")}
      progressLabel={t("host.scanProgress")}
      runningLabel={t("host.batchRunning")}
      emptyHint={eligible.length === 0 ? t("host.batchKeyEmpty") : undefined}
      onStart={handleStart}
      onStop={runner.stop}
      onClose={onClose}
    />
  );
}
