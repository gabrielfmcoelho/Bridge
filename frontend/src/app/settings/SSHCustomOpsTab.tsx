"use client";

import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { sshCustomOpsAPI } from "@/lib/api";
import { scriptParams } from "@/lib/sshCustomOps";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import type { SSHCustomOperation } from "@/lib/types";
import { ICON_PATHS } from "@/lib/icon-paths";
import RowActions from "@/components/ui/RowActions";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import { SkeletonTable } from "@/components/ui/Skeleton";
import SortableTable, { sortRows, type SortableColumn } from "@/components/ui/SortableTable";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Toggle from "@/components/ui/Toggle";

type ColKey = "name" | "params" | "use_sudo" | "actions";

// Admin CRUD over the custom SSH operations editors run from a host's Operações tab.
export default function SSHCustomOpsTab() {
  const { t } = useLocale();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const { data: ops = [], isLoading, isError, error } = useQuery({ queryKey: ["ssh-custom-ops"], queryFn: sshCustomOpsAPI.list });
  const [editing, setEditing] = useState<SSHCustomOperation | "new" | null>(null);
  const [deleteError, setDeleteError] = useState("");

  const deleteMutation = useMutation({
    mutationFn: (id: number) => sshCustomOpsAPI.delete(id),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["ssh-custom-ops"] }); setDeleteError(""); },
    onError: (err: unknown) => setDeleteError(err instanceof Error ? err.message : t("common.requestFailed")),
  });

  const columns: SortableColumn<ColKey>[] = [
    { key: "name", label: t("settings.sshOps.name") },
    { key: "params", label: t("settings.sshOps.params") },
    { key: "use_sudo", label: "sudo" },
    { key: "actions", label: "", sortable: false },
  ];
  const comparators: Record<ColKey, (a: SSHCustomOperation, b: SSHCustomOperation) => number> = {
    name: (a, b) => a.name.localeCompare(b.name),
    params: (a, b) => scriptParams(a.script).length - scriptParams(b.script).length,
    use_sudo: (a, b) => Number(b.use_sudo) - Number(a.use_sudo),
    actions: () => 0,
  };

  const addButton = <Button size="sm" onClick={() => setEditing("new")}>{t("settings.sshOps.add")}</Button>;

  return (
    <div className="space-y-4">
      <SectionCard as="h3" title={t("settings.sshOps.title")} description={t("settings.sshOps.intro")} controls={ops.length > 0 ? addButton : undefined}>
        <FormError message={deleteError} />
        {isLoading ? (
          <SkeletonTable rows={4} />
        ) : isError ? (
          <StatusAlert variant="error">{(error as Error)?.message || t("settings.sshOps.loadError")}</StatusAlert>
        ) : ops.length === 0 ? (
          <EmptyState icon="server" title={t("settings.sshOps.empty")} description={t("settings.sshOps.emptyHint")} action={addButton} compact />
        ) : (
          <SortableTable columns={columns} defaultSort="name">
            {(sortKey, sortDir) =>
              sortRows(ops, sortKey, sortDir, comparators).map((o, i) => (
                <tr key={o.id} className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                  <td className={tableClasses.td}>
                    <div className="font-medium text-[var(--text-primary)]">{o.name}</div>
                    {o.description && <div className="text-xs text-[var(--text-muted)]">{o.description}</div>}
                  </td>
                  <td className={`${tableClasses.td} font-mono text-xs text-[var(--text-secondary)]`}>{scriptParams(o.script).join(", ") || "—"}</td>
                  <td className={tableClasses.td}>{o.use_sudo ? <Badge color="amber">sudo</Badge> : <span className="text-[var(--text-faint)]">—</span>}</td>
                  <td className={`${tableClasses.td} text-right whitespace-nowrap`}>
                    <div className="flex justify-end">
                      <RowActions
                        name={o.name}
                        actions={[
                          { label: t("common.edit"), icon: ICON_PATHS.pencil, onClick: () => setEditing(o) },
                          { label: t("common.delete"), icon: ICON_PATHS.trash, danger: true, onClick: async () => { if (await confirm({ title: t("confirm.deleteTitle", { name: `"${o.name}"` }), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(o.id); } },
                        ]}
                      />
                    </div>
                  </td>
                </tr>
              ))
            }
          </SortableTable>
        )}
      </SectionCard>

      {editing && <SSHCustomOpForm op={editing === "new" ? null : editing} onClose={() => setEditing(null)} />}
    </div>
  );
}

function SSHCustomOpForm({ op, onClose }: { op: SSHCustomOperation | null; onClose: () => void }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [form, setForm] = useState({
    name: op?.name ?? "", description: op?.description ?? "", script: op?.script ?? "", use_sudo: op?.use_sudo ?? false,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");
  const set = <K extends keyof typeof form>(k: K, v: (typeof form)[K]) => setForm((f) => ({ ...f, [k]: v }));
  const params = scriptParams(form.script);

  const save = useMutation({
    mutationFn: () => (op ? sshCustomOpsAPI.update(op.id, form) : sshCustomOpsAPI.create(form)),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["ssh-custom-ops"] }); onClose(); },
    onError: (err: unknown) => setSubmitError(err instanceof Error ? err.message : t("common.requestFailed")),
  });

  const submit = (e: FormEvent) => {
    e.preventDefault();
    const next: Record<string, string> = {};
    if (!form.name.trim()) next.name = t("settings.sshOps.errName");
    if (!form.script.trim()) next.script = t("settings.sshOps.errScript");
    setErrors(next);
    setSubmitError("");
    if (Object.keys(next).length === 0) save.mutate();
  };

  return (
    <ResponsiveModal
      open
      onClose={onClose}
      title={op ? t("settings.sshOps.edit") : t("settings.sshOps.add")}
      footer={<FormFooter onCancel={onClose} submitLabel={op ? t("common.save") : t("common.create")} submitType="submit" form="ssh-op-form" loading={save.isPending} />}
    >
      <form id="ssh-op-form" onSubmit={submit} className="space-y-4">
        <FormError message={submitError} />
        <Input label={`${t("settings.sshOps.name")} *`} value={form.name} onChange={(e) => set("name", e.target.value)} error={errors.name} />
        <Input label={t("settings.sshOps.description")} value={form.description} onChange={(e) => set("description", e.target.value)} />
        <Textarea
          label={`${t("settings.sshOps.script")} *`}
          rows={12}
          className="font-mono text-xs"
          spellCheck={false}
          placeholder="docker rm -f {{container}}"
          value={form.script}
          onChange={(e) => set("script", e.target.value)}
          error={errors.script}
          hint={params.length > 0 ? t("settings.sshOps.paramsDetected", { params: params.join(", ") }) : t("settings.sshOps.paramsHint")}
        />
        <label className="flex items-center gap-3 text-sm text-[var(--text-secondary)]">
          <Toggle checked={form.use_sudo} onChange={(v) => set("use_sudo", v)} ariaLabel={t("settings.sshOps.useSudo")} />
          {t("settings.sshOps.useSudo")}
        </label>
      </form>
    </ResponsiveModal>
  );
}
