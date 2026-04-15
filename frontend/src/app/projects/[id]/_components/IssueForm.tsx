"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { issuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import type { Issue, Service } from "@/lib/types";

const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;
const PRIORITIES = ["low", "medium", "high", "critical"] as const;

const statusLabels: Record<string, string> = {
  backlog: "issue.backlog",
  todo: "issue.todo",
  in_progress: "issue.inProgress",
  review: "issue.review",
  done: "issue.done",
};

interface IssueFormProps {
  projectId: number;
  services: Service[];
  issue?: Issue;
  onSuccess: () => void;
  onDelete?: () => void;
}

export default function IssueForm({ projectId, services, issue, onSuccess, onDelete }: IssueFormProps) {
  const { t } = useLocale();
  const [form, setForm] = useState({
    title: issue?.title || "",
    description: issue?.description || "",
    status: issue?.status || "backlog",
    priority: issue?.priority || "medium",
    assignee: issue?.assignee || "",
    service_id: issue?.service_id || null as number | null,
  });
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () => {
      if (issue) {
        return issuesAPI.update(projectId, issue.id, { ...form, position: issue.position });
      }
      return issuesAPI.create(projectId, form);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  return (
    <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
      {error && (
        <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3 animate-slide-down">{error}</div>
      )}
      <Input label={t("issue.titleField")} value={form.title} onChange={(e) => set("title", e.target.value)} required autoFocus />
      <MarkdownEditor
        label={t("common.description")}
        value={form.description}
        onChange={(v) => set("description", v)}
        rows={5}
        placeholder="Supports **markdown** formatting..."
      />
      <div className="grid grid-cols-2 gap-4">
        <Select
          label={t("issue.priority")}
          value={form.priority}
          onChange={(e) => set("priority", e.target.value)}
          options={PRIORITIES.map((p) => ({ value: p, label: t(`issue.${p}`) }))}
        />
        <Select
          label={t("common.status")}
          value={form.status}
          onChange={(e) => set("status", e.target.value)}
          options={STATUSES.map((s) => ({ value: s, label: t(statusLabels[s]) }))}
        />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Input label={t("issue.assignee")} value={form.assignee} onChange={(e) => set("assignee", e.target.value)} />
        {services.length > 0 && (
          <Select
            label={t("issue.serviceScope")}
            value={form.service_id?.toString() || ""}
            onChange={(e) => set("service_id", e.target.value ? parseInt(e.target.value) : null)}
            options={[
              { value: "", label: "-" },
              ...services.map((s) => ({ value: s.id.toString(), label: s.nickname })),
            ]}
          />
        )}
      </div>
      <div className="flex items-center justify-between pt-2">
        {onDelete ? (
          <button
            type="button"
            onClick={() => { if (confirm("Delete this issue?")) onDelete(); }}
            className="text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors"
          >
            {t("common.delete")}
          </button>
        ) : <div />}
        <Button type="submit" loading={mutation.isPending}>
          {issue ? t("common.save") : t("common.create")}
        </Button>
      </div>
    </form>
  );
}
