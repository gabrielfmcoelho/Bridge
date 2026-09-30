"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import Drawer from "@/components/ui/Drawer";
import Checkbox from "@/components/ui/Checkbox";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";

export const consolidationKey = ["secrets-all", "consolidation"];

/**
 * Admin: the same password or key stored separately on several hosts. Each
 * group becomes one shared credential the hosts link to (or joins the shared
 * one that already holds that value); the per-host copies go to the trash.
 */
export default function ConsolidateDrawer({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();
  const { data: groups = [], isLoading } = useQuery({ queryKey: consolidationKey, queryFn: secretsAPI.consolidationPlan, enabled: open });
  const [picked, setPicked] = useState<Set<string> | null>(null);
  const selected = picked ?? new Set(groups.map((g) => g.key)); // all by default
  const [error, setError] = useState("");

  const apply = useMutation({
    mutationFn: () => secretsAPI.consolidate([...selected]),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
      flag({ appearance: "success", title: t("vault.consolidatedTitle"), description: t("vault.consolidatedBody", { hosts: String(r.hosts), groups: String(r.groups) }) });
      setPicked(null);
      onClose();
    },
    onError: (e) => setError(e instanceof Error ? e.message : t("form.saveFailed")),
  });

  const submit = async () => {
    if (!selected.size) return;
    if (await confirm({ title: t("vault.consolidateConfirm", { count: String(selected.size) }), message: t("vault.consolidateConfirmBody"), confirmLabel: t("vault.consolidate") })) apply.mutate();
  };

  return (
    <Drawer open={open} onClose={onClose} title={t("vault.consolidateTitle")}
      footer={groups.length > 0 ? <FormFooter onCancel={onClose} submitLabel={t("vault.consolidateN", { count: String(selected.size) })} onSubmit={submit} loading={apply.isPending} disabled={!selected.size} /> : undefined}>
      <div className="space-y-4">
        <p className="text-sm text-[var(--text-secondary)]">{t("vault.consolidateIntro")}</p>
        <FormError message={error} />
        {isLoading ? (
          <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-16 w-full" />)}</div>
        ) : groups.length === 0 ? (
          <EmptyState icon="key" title={t("vault.consolidateNone")} description={t("vault.consolidateNoneDesc")} />
        ) : (
          <>
            <StatusAlert variant="info">{t("vault.consolidateVisibility")}</StatusAlert>
            <ul className="space-y-2">
              {groups.map((g) => (
                <li key={g.key} className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-3">
                  <Checkbox
                    checked={selected.has(g.key)}
                    onChange={(v) => { const n = new Set(selected); if (v) n.add(g.key); else n.delete(g.key); setPicked(n); }}
                    label={t(g.type === "sshkey" ? "vault.consolidateKeyGroup" : "vault.consolidatePwGroup", { user: g.username || "—", count: String(g.host_ids.length) })}
                  />
                  <p className="mt-1.5 pl-6 text-xs text-[var(--text-muted)]">{g.host_names.join(", ")}</p>
                  <p className="mt-1 pl-6 text-xs text-[var(--text-secondary)]">
                    {g.target_name ? t("vault.consolidateInto", { name: g.target_name }) : t("vault.consolidateNew")}
                  </p>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
    </Drawer>
  );
}
