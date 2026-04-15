"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { globalIssuesAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Drawer from "@/components/ui/Drawer";
import EmptyState from "@/components/ui/EmptyState";
import FormError from "@/components/ui/FormError";
import type { Issue } from "@/lib/types";

const priorityColors: Record<string, string> = {
  critical: "red",
  high: "amber",
  medium: "cyan",
  low: "gray",
};

const statusLabels: Record<string, string> = {
  backlog: "Backlog",
  todo: "To Do",
  in_progress: "In Progress",
  review: "Review",
  done: "Done",
};

interface IssuesTabProps {
  issues: Issue[];
  entityType: string;
  entityId: number;
  t: (key: string) => string;
  canEdit: boolean;
}

export default function IssuesTab({ issues, entityType, entityId, t, canEdit }: IssuesTabProps) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("medium");
  const [error, setError] = useState("");

  const createMutation = useMutation({
    mutationFn: () =>
      globalIssuesAPI.create({
        title,
        description,
        priority,
        status: "backlog",
        entity_type: entityType,
        entity_id: entityId,
        created_by: user?.id || 0,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["issues", entityType, entityId] });
      setShowCreate(false);
      setTitle("");
      setDescription("");
      setPriority("medium");
      setError("");
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const openIssues = issues.filter((i) => i.status !== "done" && !i.archived);
  const closedIssues = issues.filter((i) => i.status === "done" || i.archived);

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">
          {t("issue.title") || "Issues"} ({issues.length})
        </h2>
        {canEdit && (
          <Button size="sm" onClick={() => setShowCreate(true)}>
            + {t("common.add") || "Add"}
          </Button>
        )}
      </div>

      {issues.length === 0 ? (
        <EmptyState
          icon="search"
          title={t("issue.noIssues") || "No issues"}
          description={t("issue.noIssuesDesc") || "No issues have been created for this record."}
          compact
        />
      ) : (
        <div className="space-y-4">
          {/* Open issues */}
          {openIssues.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-xs text-[var(--text-muted)] font-medium">{t("issue.open") || "Open"} ({openIssues.length})</h3>
              {openIssues.map((issue) => (
                <Card key={issue.id} hover={false} className="!p-3">
                  <div className="flex items-start gap-3">
                    <div className={`w-2 h-2 rounded-full mt-1.5 shrink-0 bg-${priorityColors[issue.priority] || "gray"}-400`} />
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-[var(--text-primary)] truncate">{issue.title}</p>
                      {issue.description && (
                        <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-2">{issue.description}</p>
                      )}
                      <div className="flex items-center gap-2 mt-1.5">
                        <Badge color={priorityColors[issue.priority]}>{issue.priority}</Badge>
                        <Badge>{statusLabels[issue.status] || issue.status}</Badge>
                      </div>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}

          {/* Closed issues */}
          {closedIssues.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-xs text-[var(--text-muted)] font-medium">{t("issue.closed") || "Closed"} ({closedIssues.length})</h3>
              {closedIssues.map((issue) => (
                <Card key={issue.id} hover={false} className="!p-3 opacity-60">
                  <div className="flex items-start gap-3">
                    <div className="w-2 h-2 rounded-full mt-1.5 shrink-0 bg-emerald-400" />
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium text-[var(--text-primary)] truncate line-through">{issue.title}</p>
                      <div className="flex items-center gap-2 mt-1">
                        <Badge color="emerald">Done</Badge>
                      </div>
                    </div>
                  </div>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Create Issue Drawer */}
      <Drawer
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title={t("issue.create") || "Create Issue"}
        footer={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" className="flex-1" onClick={() => setShowCreate(false)}>
              {t("common.cancel")}
            </Button>
            <Button size="sm" className="flex-1" onClick={() => createMutation.mutate()} loading={createMutation.isPending} disabled={!title.trim()}>
              {t("common.create")}
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          <FormError message={error} />
          <Input label={t("issue.issueTitle") || "Title"} value={title} onChange={(e) => setTitle(e.target.value)} required placeholder="Issue title..." />
          <Input label={t("common.description")} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Description..." />
          <Select
            label={t("issue.priority") || "Priority"}
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            options={[
              { value: "low", label: "Low" },
              { value: "medium", label: "Medium" },
              { value: "high", label: "High" },
              { value: "critical", label: "Critical" },
            ]}
          />
        </div>
      </Drawer>
    </div>
  );
}
