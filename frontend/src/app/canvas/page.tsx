"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { canvasAPI, entidadesAPI } from "@/lib/api";
import { creatorOptions, indentedLabel } from "@/lib/entidades";
import { getTimeAgo } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Select from "@/components/ui/Select";
import Input from "@/components/ui/Input";
import Modal from "@/components/ui/Modal";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import EmptyState from "@/components/ui/EmptyState";
import Button from "@/components/ui/Button";
import { SkeletonCard } from "@/components/ui/Skeleton";
import IssuesBoard from "@/components/issues/IssuesBoard";

/**
 * Quadros de ideias: each entidade's free-form boards. Picking an entidade
 * also shows its backlog — the issues its boards sent there.
 */
export default function CanvasListPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const [entidadeId, setEntidadeId] = useState(0);
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState("");
  const [formEntidade, setFormEntidade] = useState("");
  const [error, setError] = useState("");

  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });
  // The visible set: what the user may open boards for.
  const options = creatorOptions(entidades, user);
  const { data: boards = [], isLoading } = useQuery({
    queryKey: ["canvases", entidadeId],
    queryFn: () => canvasAPI.list(entidadeId || undefined),
  });

  const openCreate = () => {
    const primary = user?.entidades?.find((e) => e.is_primary) ?? user?.entidades?.[0];
    setTitle("");
    setFormEntidade(String(entidadeId || primary?.id || options[0]?.id || ""));
    setError("");
    setCreating(true);
  };
  const create = useMutation({
    mutationFn: () => canvasAPI.create({ entidade_id: Number(formEntidade), title: title.trim() }),
    onSuccess: (c) => router.push(`/canvas/${c.id}`),
    onError: (e) => setError(e instanceof Error ? e.message : t("form.saveFailed")),
  });
  const submit = () => {
    if (!title.trim()) return setError(t("canvas.titleRequired"));
    if (!formEntidade) return setError(t("canvas.entidadeRequired"));
    create.mutate();
  };

  return (
    <PageShell>
      <PageHeader
        title={t("nav.canvas")}
        description={t("canvas.pageDescription")}
        addLabel={canEdit ? t("canvas.create") : undefined}
        onAdd={canEdit ? openCreate : undefined}
      />

      <div className="max-w-xs mb-6">
        <Select
          label={t("canvas.entidade")}
          value={String(entidadeId || "")}
          onChange={(e) => setEntidadeId(Number(e.target.value) || 0)}
          options={[{ value: "", label: t("common.all") }, ...options.map((e) => ({ value: String(e.id), label: indentedLabel(e) }))]}
        />
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">{[0, 1, 2].map((i) => <SkeletonCard key={i} />)}</div>
      ) : boards.length === 0 ? (
        <EmptyState icon="folder" title={t("canvas.emptyTitle")} description={t("canvas.emptyHint")}
          action={canEdit ? <Button size="sm" onClick={openCreate}>{t("canvas.create")}</Button> : undefined} />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {boards.map((b) => (
            <Link key={b.id} href={`/canvas/${b.id}`} className="block">
              <Card className="h-full hover:border-[var(--accent)]/40 transition-colors">
                <div className="font-medium text-[var(--text-primary)] truncate">{b.title}</div>
                <div className="mt-1 text-xs text-[var(--text-muted)]">
                  {b.entidade_name} · {t("canvas.editedAgo", { when: getTimeAgo(b.updated_at, locale) })}
                </div>
              </Card>
            </Link>
          ))}
        </div>
      )}

      {entidadeId > 0 && (
        <section className="mt-10">
          <SectionHeading>{t("canvas.backlog")}</SectionHeading>
          <IssuesBoard entityType="entidade" entityId={entidadeId} canEdit={canEdit} />
        </section>
      )}

      <Modal open={creating} onClose={() => setCreating(false)} title={t("canvas.create")} size="sm"
        footer={<FormFooter onCancel={() => setCreating(false)} submitLabel={t("canvas.create")} onSubmit={submit} loading={create.isPending} />}>
        <div className="space-y-4">
          <Input label={t("canvas.titleLabel")} value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus
            onKeyDown={(e) => { if (e.key === "Enter") submit(); }} />
          <Select label={t("canvas.entidade")} value={formEntidade} onChange={(e) => setFormEntidade(e.target.value)}
            options={options.map((e) => ({ value: String(e.id), label: indentedLabel(e) }))} />
          <FormError message={error} />
        </div>
      </Modal>
    </PageShell>
  );
}
