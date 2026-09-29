"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { releasesAPI, issuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import CheckboxList from "@/components/ui/CheckboxList";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import Button from "@/components/ui/Button";
import { RELEASE_STATUSES } from "@/lib/releases";
import type { Release } from "@/lib/types";

/** Create/edit one of a project's releases; the project is fixed. */
export default function ReleaseForm({ projectId, release, onDone, onCancel, canDelete }: {
  projectId: number;
  release?: Release & { issue_ids?: number[] };
  onDone: () => void;
  onCancel: () => void;
  canDelete?: boolean;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const [form, setForm] = useState({
    title: release?.title ?? "",
    description: release?.description ?? "",
    status: release?.status ?? "pending",
    target_date: release?.target_date ?? "",
    live_date: release?.live_date ?? "",
  });
  const [issueIds, setIssueIds] = useState<number[]>(release?.issue_ids ?? []);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);
  const set = (k: keyof typeof form, v: string) => setForm((f) => ({ ...f, [k]: v }));

  const { data: issues = [] } = useQuery({ queryKey: ["issues", "project", projectId], queryFn: () => issuesAPI.listByProject(projectId) });

  const onError = (err: unknown) => setError(err instanceof Error ? err.message : t("form.saveFailed"));
  const save = useMutation({
    mutationFn: () => {
      const payload = { ...form, title: form.title.trim(), project_id: projectId, issue_ids: issueIds };
      return release ? releasesAPI.update(release.id, payload) : releasesAPI.create(payload);
    },
    onSuccess: onDone,
    onError,
  });
  const remove = useMutation({ mutationFn: () => releasesAPI.delete(release!.id), onSuccess: onDone, onError });

  const titleError = attempted && !form.title.trim() ? t("release.titleRequired") : undefined;
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    setAttempted(true);
    if (!form.title.trim()) return;
    setError("");
    save.mutate();
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <FormError message={error} />
      <Input label={t("release.titleField")} value={form.title} onChange={(e) => set("title", e.target.value)} required autoFocus
        error={titleError} aria-invalid={!!titleError} />
      <Select label={t("common.status")} value={form.status} onChange={(e) => set("status", e.target.value)}
        options={RELEASE_STATUSES.map((s) => ({ value: s, label: t(`release.${s}`) }))} />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Input label={t("release.targetDate")} type="date" value={form.target_date} onChange={(e) => set("target_date", e.target.value)} />
        <Input label={t("release.liveDate")} type="date" value={form.live_date} onChange={(e) => set("live_date", e.target.value)}
          hint={t("release.liveDateHint")} />
      </div>
      <MarkdownEditor label={t("common.description")} value={form.description} onChange={(v) => set("description", v)} rows={4} />
      <CheckboxList label={t("release.linkedIssues")} items={issues.map((i) => ({ id: i.id, name: i.title }))} selected={issueIds} onChange={setIssueIds} />
      <FormFooter
        onCancel={onCancel}
        submitType="submit"
        submitLabel={release ? t("form.saveChanges") : t("release.create")}
        loading={save.isPending}
        start={release && canDelete ? (
          <Button type="button" size="sm" variant="ghost" loading={remove.isPending}
            onClick={async () => { if (await confirm({ title: t("confirm.deleteRelease"), danger: true, confirmLabel: t("common.delete") })) remove.mutate(); }}>
            {t("common.delete")}
          </Button>
        ) : undefined}
      />
    </form>
  );
}
