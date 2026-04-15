"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { issuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import Button from "@/components/ui/Button";
import Select from "@/components/ui/Select";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import IssueForm from "./IssueForm";
import type { Issue, Service } from "@/lib/types";

const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;

const priorityColors: Record<string, string> = {
  critical: "bg-red-400",
  high: "bg-amber-400",
  medium: "bg-cyan-400",
  low: "bg-[var(--text-faint)]",
};

const statusLabels: Record<string, string> = {
  backlog: "issue.backlog",
  todo: "issue.todo",
  in_progress: "issue.inProgress",
  review: "issue.review",
  done: "issue.done",
};

interface IssueBoardProps {
  projectId: number;
  services: Service[];
  canEdit: boolean;
}

export default function IssueBoard({ projectId, services, canEdit }: IssueBoardProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [serviceFilter, setServiceFilter] = useState<number | undefined>(undefined);
  const [showCreate, setShowCreate] = useState(false);
  const [editIssue, setEditIssue] = useState<Issue | null>(null);
  const [draggedIssue, setDraggedIssue] = useState<Issue | null>(null);

  const { data: issues = [] } = useQuery({
    queryKey: ["project-issues", projectId, serviceFilter],
    queryFn: () => issuesAPI.listByProject(projectId, serviceFilter),
  });

  const safeIssues = Array.isArray(issues) ? issues : [];

  const moveMutation = useMutation({
    mutationFn: ({ issueId, status, position }: { issueId: number; status: string; position: number }) =>
      issuesAPI.move(projectId, issueId, status, position),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["project-issues", projectId] }),
  });

  const deleteMutation = useMutation({
    mutationFn: (issueId: number) => issuesAPI.delete(projectId, issueId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["project-issues", projectId] }),
  });

  const issuesByStatus = (status: string) =>
    safeIssues.filter((i) => i.status === status).sort((a, b) => a.position - b.position);

  const handleDrop = (status: string) => {
    if (!draggedIssue || draggedIssue.status === status) {
      if (draggedIssue?.status === status) {
        setDraggedIssue(null);
        return;
      }
      setDraggedIssue(null);
      return;
    }
    const columnIssues = issuesByStatus(status);
    const newPosition = columnIssues.length > 0 ? columnIssues[columnIssues.length - 1].position + 1 : 0;
    moveMutation.mutate({ issueId: draggedIssue.id, status, position: newPosition });
    setDraggedIssue(null);
  };

  const serviceOptions = [
    { value: "", label: t("issue.allServices") },
    ...services.map((s) => ({ value: s.id.toString(), label: s.nickname })),
  ];

  return (
    <div className="space-y-4">
      {/* Toolbar */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          {services.length > 0 && (
            <Select
              value={serviceFilter?.toString() || ""}
              onChange={(e) => setServiceFilter(e.target.value ? parseInt(e.target.value) : undefined)}
              options={serviceOptions}
            />
          )}
        </div>
        {canEdit && (
          <Button size="sm" onClick={() => setShowCreate(true)}>+ {t("issue.create")}</Button>
        )}
      </div>

      {/* Kanban columns */}
      <div className="grid grid-cols-5 gap-3" style={{ minHeight: "400px" }}>
        {STATUSES.map((status) => {
          const columnIssues = issuesByStatus(status);
          return (
            <div
              key={status}
              className="flex flex-col rounded-[var(--radius-lg)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] overflow-hidden"
              onDragOver={(e) => e.preventDefault()}
              onDrop={() => handleDrop(status)}
            >
              {/* Column header */}
              <div className="px-3 py-2.5 border-b border-[var(--border-subtle)] flex items-center justify-between">
                <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]" style={{ fontFamily: "var(--font-display)" }}>
                  {t(statusLabels[status])}
                </span>
                <span className="text-[10px] font-medium text-[var(--text-faint)] bg-[var(--bg-elevated)] rounded-full px-1.5 py-0.5">
                  {columnIssues.length}
                </span>
              </div>

              {/* Cards */}
              <div className="flex-1 p-2 space-y-2 overflow-y-auto">
                {columnIssues.map((issue) => (
                  <div
                    key={issue.id}
                    draggable={canEdit}
                    onDragStart={() => setDraggedIssue(issue)}
                    onClick={() => canEdit && setEditIssue(issue)}
                    className={`p-2.5 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] transition-all duration-150 ${
                      canEdit ? "cursor-grab hover:border-[var(--border-default)] hover:shadow-[var(--shadow-sm)] active:cursor-grabbing" : ""
                    } ${draggedIssue?.id === issue.id ? "opacity-40" : ""}`}
                  >
                    <div className="flex items-start gap-1.5 mb-1.5">
                      <span className={`w-2 h-2 rounded-full shrink-0 mt-1 ${priorityColors[issue.priority]}`} />
                      <span className="text-xs font-medium text-[var(--text-primary)] line-clamp-2 leading-tight">{issue.title}</span>
                    </div>
                    <div className="flex items-center gap-1.5 flex-wrap">
                      {issue.assignee && (
                        <span className="text-[10px] text-[var(--text-muted)] bg-[var(--bg-overlay)] rounded px-1.5 py-0.5">{issue.assignee}</span>
                      )}
                      {issue.service_id && (
                        <span className="text-[10px] text-purple-400 bg-purple-500/10 rounded px-1.5 py-0.5">
                          {services.find((s) => s.id === issue.service_id)?.nickname || ""}
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>

      {/* Create modal */}
      <ResponsiveModal open={showCreate} onClose={() => setShowCreate(false)} title={t("issue.create")}>
        <IssueForm
          projectId={projectId}
          services={services}
          onSuccess={() => {
            setShowCreate(false);
            queryClient.invalidateQueries({ queryKey: ["project-issues", projectId] });
          }}
        />
      </ResponsiveModal>

      {/* Edit modal */}
      <ResponsiveModal open={!!editIssue} onClose={() => setEditIssue(null)} title={t("issue.editIssue")}>
        {editIssue && (
          <IssueForm
            projectId={projectId}
            services={services}
            issue={editIssue}
            onSuccess={() => {
              setEditIssue(null);
              queryClient.invalidateQueries({ queryKey: ["project-issues", projectId] });
            }}
            onDelete={user?.role === "admin" ? () => {
              deleteMutation.mutate(editIssue.id);
              setEditIssue(null);
            } : undefined}
          />
        )}
      </ResponsiveModal>
    </div>
  );
}
