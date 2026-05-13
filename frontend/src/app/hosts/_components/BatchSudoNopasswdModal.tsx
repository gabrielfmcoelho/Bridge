"use client";

import { useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import type { Host } from "@/lib/types";
import BatchOperationShell, { type ScopeOption } from "./BatchOperationShell";
import { useBatchRunner } from "./useBatchRunner";

type SudoScope = "all" | "password_only" | "both";

export default function BatchSudoNopasswdModal({
  hosts,
  onClose,
  t,
}: {
  hosts: Host[];
  onClose: () => void;
  t: (key: string) => string;
}) {
  const queryClient = useQueryClient();
  const [concurrency, setConcurrency] = useLocalStorage("hosts.sudoNopasswdConcurrency", 5);
  const [scope, setScope] = useLocalStorage<SudoScope>("hosts.sudoNopasswdScope", "all");
  const runner = useBatchRunner();

  // setup-sudo-nopasswd needs the stored password to feed `sudo -S`. Hosts
  // without a stored password are not eligible regardless of key auth.
  const eligible = useMemo(() => hosts.filter(h => h.has_password), [hosts]);
  const passwordOnly = useMemo(() => eligible.filter(h => !h.has_key), [eligible]);
  const both = useMemo(() => eligible.filter(h => h.has_key), [eligible]);

  const effectiveScope: SudoScope =
    scope === "password_only" && passwordOnly.length > 0 ? "password_only" :
    scope === "both" && both.length > 0 ? "both" :
    "all";
  const targets = effectiveScope === "password_only" ? passwordOnly : effectiveScope === "both" ? both : eligible;

  const scopeOptions: ScopeOption[] = [
    { key: "all", label: t("host.scopeAll"), count: eligible.length },
    { key: "password_only", label: t("host.batchSudoScopePasswordOnly"), count: passwordOnly.length },
    { key: "both", label: t("host.batchSudoScopeBoth"), count: both.length },
  ];

  const handleStart = () => {
    runner.start({
      hosts: targets,
      concurrency,
      runOne: async (host) => {
        try {
          const res = await sshAPI.setupSudoNopasswd(host.oficial_slug);
          return { success: res.success, error: res.error };
        } catch (err) {
          return { success: false, error: err instanceof Error ? err.message : "Failed" };
        }
      },
    }).then(() => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
    });
  };

  return (
    <BatchOperationShell
      description={t("host.batchSudoDesc")}
      scopeLabel={t("host.scanScope")}
      scope={effectiveScope}
      onScopeChange={(s) => setScope(s as SudoScope)}
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
      startLabel={t("host.batchSudoStart")}
      rerunLabel={t("host.rescan")}
      stopLabel={t("host.stopScan")}
      cancelLabel={t("common.cancel")}
      progressLabel={t("host.scanProgress")}
      runningLabel={t("host.batchRunning")}
      emptyHint={eligible.length === 0 ? t("host.batchSudoEmpty") : undefined}
      onStart={handleStart}
      onStop={runner.stop}
      onClose={onClose}
    />
  );
}
