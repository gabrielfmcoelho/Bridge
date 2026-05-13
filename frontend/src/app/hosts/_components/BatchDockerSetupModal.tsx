"use client";

import { useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { sshAPI } from "@/lib/api";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import type { Host } from "@/lib/types";
import BatchOperationShell, { type ScopeOption } from "./BatchOperationShell";
import { useBatchRunner } from "./useBatchRunner";

type DockerScope = "all" | "needs_setup" | "ok";

// A docker_group_status of ok/fixed means docker is installed AND the user can
// run `docker ps` without sudo. Anything else (including null/undefined and
// "needs_relogin") still benefits from running setup again.
function dockerStatusBucket(h: Host): "ok" | "needs_setup" {
  return h.docker_group_status === "ok" || h.docker_group_status === "fixed" ? "ok" : "needs_setup";
}

export default function BatchDockerSetupModal({
  hosts,
  onClose,
  t,
}: {
  hosts: Host[];
  onClose: () => void;
  t: (key: string) => string;
}) {
  const queryClient = useQueryClient();
  const [concurrency, setConcurrency] = useLocalStorage("hosts.dockerSetupConcurrency", 5);
  const [scope, setScope] = useLocalStorage<DockerScope>("hosts.dockerSetupScope", "needs_setup");
  const runner = useBatchRunner();

  const eligible = useMemo(() => hosts.filter(h => h.has_password || h.has_key), [hosts]);
  const needsSetupHosts = useMemo(() => eligible.filter(h => dockerStatusBucket(h) === "needs_setup"), [eligible]);
  const okHosts = useMemo(() => eligible.filter(h => dockerStatusBucket(h) === "ok"), [eligible]);

  const effectiveScope: DockerScope =
    scope === "needs_setup" && needsSetupHosts.length > 0 ? "needs_setup" :
    scope === "ok" && okHosts.length > 0 ? "ok" :
    "all";
  const targets = effectiveScope === "needs_setup" ? needsSetupHosts : effectiveScope === "ok" ? okHosts : eligible;

  const scopeOptions: ScopeOption[] = [
    { key: "all", label: t("host.scopeAll"), count: eligible.length },
    { key: "needs_setup", label: t("host.batchDockerScopeNeedsSetup"), count: needsSetupHosts.length },
    { key: "ok", label: t("host.batchDockerScopeOk"), count: okHosts.length },
  ];

  const handleStart = () => {
    runner.start({
      hosts: targets,
      concurrency,
      runOne: async (host) => {
        try {
          const res = await sshAPI.dockerSetup(host.oficial_slug, true);
          if (!res.success) return { success: false, error: res.error || "Failed" };
          const s = res.status;
          if (!s) return { success: true };
          if (!s.installed) return { success: false, error: t("host.batchDockerErrNotInstalled") };
          if (s.needs_sudo && !s.group_fix_applied) return { success: false, error: t("host.batchDockerErrNeedsSudo") };
          return { success: true };
        } catch (err) {
          return { success: false, error: err instanceof Error ? err.message : "Failed" };
        }
      },
    }).then(() => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["dashboard"] });
    });
  };

  return (
    <BatchOperationShell
      description={t("host.batchDockerDesc")}
      scopeLabel={t("host.scanScope")}
      scope={effectiveScope}
      onScopeChange={(s) => setScope(s as DockerScope)}
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
      startLabel={t("host.batchDockerStart")}
      rerunLabel={t("host.rescan")}
      stopLabel={t("host.stopScan")}
      cancelLabel={t("common.cancel")}
      progressLabel={t("host.scanProgress")}
      runningLabel={t("host.batchRunning")}
      emptyHint={eligible.length === 0 ? t("host.batchDockerEmpty") : undefined}
      onStart={handleStart}
      onStop={runner.stop}
      onClose={onClose}
    />
  );
}
