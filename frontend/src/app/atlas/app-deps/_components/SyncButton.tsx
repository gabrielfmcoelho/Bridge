"use client";

import { useEffect, useRef } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { appDepsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";

/** Reads every repository's catalog from GitLab in the background: POST starts the run,
 *  the status query polls while it runs and a flag reports how it ended. */
export default function SyncButton() {
  const { t } = useLocale();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const { data: status } = useQuery({
    queryKey: ["app-deps-sync"],
    queryFn: appDepsAPI.syncStatus,
    refetchInterval: (query) => (query.state.data?.running ? 3000 : false),
  });
  const sync = useMutation({
    mutationFn: appDepsAPI.sync,
    onSuccess: (st) => queryClient.setQueryData(["app-deps-sync"], st),
    onError: (err) => flag({ appearance: "error", title: t("appDeps.sync.error"), description: err instanceof Error ? err.message : undefined }),
  });
  const running = sync.isPending || !!status?.running;

  const wasRunning = useRef(false);
  useEffect(() => {
    if (!status) return;
    if (wasRunning.current && !status.running) {
      queryClient.invalidateQueries({ queryKey: ["app-deps"] });
      const r = status.result;
      if (status.error || !r) {
        flag({ appearance: "error", title: t("appDeps.sync.error"), description: status.error });
      } else {
        const errors = r.errors ?? [];
        const notes = [
          r.truncated ? t("appDeps.sync.truncated") : "",
          errors.length ? t("appDeps.sync.errors", { errors: errors.slice(0, 3).join("; ") + (errors.length > 3 ? ` (+${errors.length - 3})` : "") }) : "",
        ].filter(Boolean);
        flag({
          appearance: errors.length || r.failed || r.truncated ? "warning" : "success",
          title: t("appDeps.sync.done", { found: String(r.found), repos: String(r.repos), failed: String(r.failed), removed: String(r.removed) }),
          description: notes.length ? notes.join(" ") : undefined,
        });
      }
    }
    wasRunning.current = status.running;
  }, [status, flag, queryClient, t]);

  return (
    <Button size="sm" variant="secondary" loading={running} disabled={running} onClick={() => sync.mutate()}>
      {!running && <Icon path={ICON_PATHS.refresh} className="w-4 h-4" />}
      {running ? t("appDeps.sync.running") : t("appDeps.sync.button")}
    </Button>
  );
}
