"use client";

import RowActions from "@/components/ui/RowActions";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { entidadesAPI } from "@/lib/api";
import { indentedLabel, withDepth } from "@/lib/entidades";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import type { AssetGrantsInput, AssetType, Entidade } from "@/lib/types";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Checkbox from "@/components/ui/Checkbox";
import FormError from "@/components/ui/FormError";
import Pagination from "@/components/ui/Pagination";
import EntidadeScopeFields from "@/components/entidades/EntidadeScopeFields";

const ASSET_TYPES: AssetType[] = ["host", "dns", "service", "project", "contact", "tool", "ssh_key", "api_catalog", "secret"];

type EntidadeForm = { id: number | null; name: string; slug: string; parent_id: number | null; description: string };
const emptyForm: EntidadeForm = { id: null, name: "", slug: "", parent_id: null, description: "" };

export default function EntidadesTab() {
  const confirm = useConfirm();
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const { data: entidades = [], isLoading } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });
  const [form, setForm] = useState<EntidadeForm>(emptyForm);
  const [error, setError] = useState("");

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["entidades"] });
  const fail = (err: unknown) => setError(err instanceof Error ? err.message : t("common.requestFailed"));

  const saveMutation = useMutation({
    mutationFn: () =>
      form.id
        ? entidadesAPI.update(form.id, { name: form.name, slug: form.slug, parent_id: form.parent_id, description: form.description })
        : entidadesAPI.create({ name: form.name, slug: form.slug || undefined, parent_id: form.parent_id, description: form.description }),
    onSuccess: () => { invalidate(); setForm(emptyForm); setError(""); },
    onError: fail,
  });
  const deleteMutation = useMutation({
    mutationFn: (id: number) => entidadesAPI.delete(id),
    onSuccess: () => { invalidate(); setError(""); },
    onError: fail,
  });

  const nodes = withDepth(entidades);
  const edit = (e: Entidade) => setForm({ id: e.id, name: e.name, slug: e.slug, parent_id: e.parent_id, description: e.description });

  return (
    <div className="space-y-4">
      <SectionCard as="h3" title={t("entidades.title")} description={t("entidades.intro")}>
        <FormError message={error} />

        {isLoading ? (
          <div className="animate-pulse space-y-2">{[...Array(4)].map((_, i) => <div key={i} className="h-8 bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" />)}</div>
        ) : (
          <div className="overflow-x-auto mb-4">
            <table className={tableClasses.compact.table}>
              <thead>
                <tr className={tableClasses.compact.headRow}>
                  <th className={tableClasses.compact.th}>{t("common.name")}</th>
                  <th className={tableClasses.compact.th}>{t("settings.offerings.slug")}</th>
                  <th className="w-24" />
                </tr>
              </thead>
              <tbody>
                {nodes.map((e) => (
                  <tr key={e.id} className={tableClasses.compact.row}>
                    <td className={`${tableClasses.compact.td} text-[var(--text-primary)]`} style={{ paddingLeft: `${8 + e.depth * 18}px` }}>
                      {e.depth > 0 && <span className="text-[var(--text-faint)] mr-1">↳</span>}{e.name}
                      {e.description && <span className="ml-2 text-xs text-[var(--text-faint)]">{e.description}</span>}
                    </td>
                    <td className={`${tableClasses.compact.td} font-mono text-xs text-[var(--text-secondary)]`}>{e.slug}</td>
                    <td className={`${tableClasses.compact.td} text-right whitespace-nowrap`}>
                      <div className="flex justify-end">
                        <RowActions
                          name={e.name}
                          actions={[
                            { label: t("common.edit"), icon: ICON_PATHS.pencil, onClick: () => edit(e) },
                            { label: t("common.delete"), icon: ICON_PATHS.trash, danger: true, onClick: async () => { if (await confirm({ title: t("confirm.deleteTitle", { name: e.name }), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(e.id); } },
                          ]}
                        />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <div className="border-t border-[var(--border-subtle)] pt-4">
          <p className="text-xs font-semibold text-[var(--text-muted)] mb-3">
            {form.id ? t("entidades.edit") : t("entidades.add")}
          </p>
          <form
            onSubmit={(e) => { e.preventDefault(); if (form.name.trim()) saveMutation.mutate(); }}
            className="grid grid-cols-1 md:grid-cols-4 gap-3 items-end"
          >
            <Input label={t("common.name")} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            <Input label={t("settings.offerings.slug")} value={form.slug} onChange={(e) => setForm({ ...form, slug: e.target.value })} placeholder={t("settings.offerings.slugAuto")} />
            <Select
              label={t("entidades.parent")}
              value={form.parent_id != null ? String(form.parent_id) : ""}
              onChange={(e) => setForm({ ...form, parent_id: e.target.value ? Number(e.target.value) : null })}
              options={[
                { value: "", label: t("entidades.root") },
                ...nodes.filter((n) => n.id !== form.id).map((n) => ({ value: String(n.id), label: indentedLabel(n) })),
              ]}
            />
            <div className="flex gap-2">
              <Button type="submit" loading={saveMutation.isPending}>{form.id ? t("common.save") : t("common.create")}</Button>
              {form.id && <Button type="button" variant="ghost" onClick={() => setForm(emptyForm)}>{t("common.cancel")}</Button>}
            </div>
            <Input
              className="md:col-span-4"
              label={t("common.description")}
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
          </form>
        </div>
      </SectionCard>

      <UnassignedTriage />
    </div>
  );
}

/** Admin triage: rows with no grants are admin-only; pick some, assign grants. */
function UnassignedTriage() {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [assetType, setAssetType] = useState<AssetType>("host");
  const [page, setPage] = useState(1);
  const [selected, setSelected] = useState<number[]>([]);
  const [grants, setGrants] = useState<AssetGrantsInput>({ creator_entidade_id: null, responsible_entidade_ids: [], is_global: false });
  const [error, setError] = useState("");
  const perPage = 50;

  const { data, isFetching } = useQuery({
    queryKey: ["entidades-unassigned", assetType, page],
    queryFn: () => entidadesAPI.unassigned(assetType, page, perPage),
  });
  const rows = data?.data ?? [];
  const total = data?.meta.total ?? 0;

  const assign = useMutation({
    mutationFn: () => entidadesAPI.bulkAssign({ asset_type: assetType, asset_ids: selected, ...grants }),
    onSuccess: () => {
      setSelected([]);
      setError("");
      queryClient.invalidateQueries({ queryKey: ["entidades-unassigned"] });
    },
    onError: (err) => setError(err instanceof Error ? err.message : t("common.requestFailed")),
  });

  const allChecked = rows.length > 0 && rows.every((r) => selected.includes(r.id));
  const canAssign = selected.length > 0 && (grants.creator_entidade_id != null || grants.is_global || (grants.responsible_entidade_ids?.length ?? 0) > 0);

  return (
    <SectionCard
      as="h3"
      title={t("entidades.unassigned")}
      description={t("entidades.unassignedIntro")}
      footer={total > perPage ? (
        <Pagination page={page} totalPages={Math.ceil(total / perPage)} total={total} perPage={perPage} onChange={setPage} bare />
      ) : undefined}
    >
      <FormError message={error} />

      <div className="flex flex-wrap gap-1.5 mb-4">
        {ASSET_TYPES.map((tp) => (
          <button
            key={tp}
            onClick={() => { setAssetType(tp); setPage(1); setSelected([]); }}
            className={`px-2.5 py-1 text-xs rounded-[var(--radius-md)] border transition-colors ${
              assetType === tp
                ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/30"
                : "text-[var(--text-secondary)] border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)]"
            }`}
          >
            {t(`entidades.type.${tp}`)}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div>
          <div className="flex items-center justify-between mb-2">
            <Checkbox
              label={`${t("entidades.selectAll")} (${total})`}
              checked={allChecked}
              onChange={(c) => setSelected(c ? rows.map((r) => r.id) : [])}
              disabled={rows.length === 0}
            />
            <span className="text-2xs text-[var(--text-faint)]">{selected.length} {t("entidades.selected")}</span>
          </div>
          <div className={`max-h-80 overflow-y-auto border border-[var(--border-subtle)] rounded-[var(--radius-md)] ${isFetching ? "opacity-60" : ""}`}>
            {rows.length === 0 ? (
              <div className="text-center py-6 text-sm text-[var(--text-muted)]">{t("entidades.nothingUnassigned")}</div>
            ) : (
              rows.map((r) => (
                <label key={r.id} className="flex items-center gap-2 px-3 py-1.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] cursor-pointer">
                  <input
                    type="checkbox"
                    className="w-4 h-4 rounded accent-[var(--accent)]"
                    checked={selected.includes(r.id)}
                    onChange={(e) => setSelected(e.target.checked ? [...selected, r.id] : selected.filter((id) => id !== r.id))}
                  />
                  <span className="truncate">{r.name || `#${r.id}`}</span>
                  <span className="ml-auto text-2xs text-[var(--text-faint)]">#{r.id}</span>
                </label>
              ))
            )}
          </div>
        </div>

        <div className="space-y-4">
          <EntidadeScopeFields value={grants} onChange={setGrants} compact />
          <Button onClick={() => assign.mutate()} disabled={!canAssign} loading={assign.isPending}>
            {t("entidades.assign")} ({selected.length})
          </Button>
        </div>
      </div>
    </SectionCard>
  );
}
