"use client";


import { DragDropContext, Droppable, Draggable, type DropResult } from "@hello-pangea/dnd";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import type { Issue } from "@/lib/types";
import { STATUSES, getStatusLabels } from "./issueStatus";
import { PRIORITY_DOT_COLOR } from "@/lib/alert-colors";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/* ─── Issues Kanban ─── */

export function IssuesKanban({ issues, users, onEdit, onMove }: {
  issues: Issue[];
  users: { id: number; display_name: string }[];
  onEdit: (issue: Issue) => void;
  onMove?: (issueId: number, newStatus: string, position: number) => void;
}) {
  const { t } = useLocale();
  const STATUS_LABELS = getStatusLabels(t);
  const getInitials = (name: string) => {
    const parts = name.trim().split(/\s+/);
    return parts.length === 1 ? parts[0].slice(0, 2).toUpperCase() : (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  };

  const handleDragEnd = (result: DropResult) => {
    if (!result.destination || !onMove) return;
    const newStatus = result.destination.droppableId;
    const issueId = parseInt(result.draggableId);
    const position = result.destination.index;
    if (result.source.droppableId === newStatus && result.source.index === position) return;
    onMove(issueId, newStatus, position);
  };

  // Six empty columns say less than one line.
  if (issues.length === 0) return <p className="py-4 text-sm text-[var(--text-muted)]">{t("issue.boardEmpty")}</p>;

  return (
    <DragDropContext onDragEnd={handleDragEnd}>
      <div className="overflow-x-auto -mx-1 px-1 pb-2">
        <div className="flex gap-3 min-w-[1200px]">
          {STATUSES.map((status) => {
            const items = issues.filter((i) => i.status === status);
            return (
              <div key={status} className="flex-1 min-w-[220px]">
                <div className="flex items-center gap-1.5 mb-2 px-1">
                  <span className="text-xs font-medium text-[var(--text-muted)]">{STATUS_LABELS[status]}</span>
                  {items.length > 0 && <span className="text-2xs font-mono tabular-nums text-[var(--text-muted)]">{items.length}</span>}
                </div>
                <Droppable droppableId={status}>
                  {(provided, snapshot) => (
                    <div
                      ref={provided.innerRef}
                      {...provided.droppableProps}
                      className={`space-y-2 min-h-[80px] p-1 rounded-[var(--radius-md)] border border-dashed transition-colors ${
                        snapshot.isDraggingOver
                          ? "bg-[var(--accent-muted)]/20 border-[var(--accent)]/30"
                          : "bg-[var(--bg-base)]/50 border-[var(--border-subtle)]/30"
                      }`}
                    >
                      {items.map((issue, index) => {
                        const assignees = (issue.assignee_ids || []).slice(0, 3).map((uid) => users.find((u) => u.id === uid)).filter(Boolean);
                        return (
                          <Draggable key={issue.id} draggableId={String(issue.id)} index={index}>
                            {(dragProvided, dragSnapshot) => (
                              <div
                                ref={dragProvided.innerRef}
                                {...dragProvided.draggableProps}
                                {...dragProvided.dragHandleProps}
                              >
                                {/* Title first; one quiet meta line under it. Entity is implied by the page, and
                                    empty fields are omitted instead of labelled "–" (the drawer has them all). */}
                                <Card
                                  onClick={() => onEdit(issue)}
                                  className={`!p-3 ${
                                    dragSnapshot.isDragging
                                      ? "shadow-lg !border-[var(--accent)]/40 ring-1 ring-[var(--accent)]/20"
                                      : ""
                                  }`}
                                >
                                  <div className="flex items-start gap-2">
                                    {issue.status === "done"
                                      ? <Icon path={ICON_PATHS.checkCircle} className="w-3.5 h-3.5 shrink-0 mt-0.5 text-[var(--success)]" />
                                      : <span className={`w-2 h-2 rounded-full shrink-0 mt-1.5 ${PRIORITY_DOT_COLOR[issue.priority] || PRIORITY_DOT_COLOR.medium}`} title={`${t("common.priority")}: ${t(`issue.${issue.priority}`)}`} />}
                                    <p className={`text-sm leading-snug line-clamp-3 ${issue.archived ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{issue.title}</p>
                                  </div>
                                  <div className="flex items-center gap-2.5 mt-2.5 pl-4 text-2xs text-[var(--text-muted)]">
                                    <span className="font-mono">#{issue.id}</span>
                                    {issue.expected_end_date && (
                                      <span className="inline-flex items-center gap-1 font-mono" title={t("issue.due")}>
                                        <Icon path={ICON_PATHS.clock} className="w-3 h-3" />{issue.expected_end_date}
                                      </span>
                                    )}
                                    {((issue.alert_ids?.length || 0) > 0 || issue.source === "alert") && (
                                      <span title={t("issue.entityAlert")}><Icon path={ICON_PATHS.alert} className="w-3 h-3 text-[var(--warning)]" /></span>
                                    )}
                                    {issue.archived && (
                                      <span title={t("issue.archived")}><Icon path={ICON_PATHS.archive} className="w-3 h-3" /></span>
                                    )}
                                    {assignees.length > 0 && (
                                      <span className="ml-auto flex -space-x-1">
                                        {assignees.map((user) => (
                                          <span key={user!.id} className="w-5 h-5 rounded-full bg-[var(--bg-elevated)] border border-[var(--bg-surface)] text-3xs font-semibold text-[var(--text-secondary)] flex items-center justify-center" title={user!.display_name}>
                                            {getInitials(user!.display_name)}
                                          </span>
                                        ))}
                                        {(issue.assignee_ids?.length || 0) > 3 && <span className="w-5 h-5 rounded-full bg-[var(--bg-elevated)] border border-[var(--bg-surface)] text-3xs text-[var(--text-muted)] flex items-center justify-center">+{issue.assignee_ids!.length - 3}</span>}
                                      </span>
                                    )}
                                  </div>
                                </Card>
                              </div>
                            )}
                          </Draggable>
                        );
                      })}
                      {provided.placeholder}
                    </div>
                  )}
                </Droppable>
              </div>
            );
          })}
        </div>
      </div>
    </DragDropContext>
  );
}

/* ─── Issues Table View ─── */

export function IssuesTableView({ issues, users, onEdit }: {
  issues: Issue[];
  users: { id: number; display_name: string }[];
  onEdit: (issue: Issue) => void;
}) {
  const { t } = useLocale();
  const STATUS_LABELS = getStatusLabels(t);

  const PRIORITY_ORDER: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 };

  return (
    <SortableTable
      columns={[
        { key: "priority" as const, label: "P" },
        { key: "title" as const, label: t("common.title") },
        { key: "status" as const, label: t("common.status") },
        { key: "assignees" as const, label: t("common.assignees") },
      ]}
      defaultSort="title"
    >
      {(sk, sd) => {
        const sorted = sortRows(issues, sk, sd, {
          priority: (a, b) => (PRIORITY_ORDER[a.priority] ?? 2) - (PRIORITY_ORDER[b.priority] ?? 2),
          title: (a, b) => a.title.localeCompare(b.title),
          status: (a, b) => a.status.localeCompare(b.status),
          assignees: (a, b) => (a.assignee_ids?.length || 0) - (b.assignee_ids?.length || 0),
        });
        return sorted.map((issue, i) => (
          <tr key={issue.id} className={`border-t border-[var(--border-subtle)] cursor-pointer hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`} onClick={() => onEdit(issue)}>
            <td className="px-4 py-2.5"><span className={`w-2 h-2 rounded-full inline-block ${PRIORITY_DOT_COLOR[issue.priority]}`} /></td>
            <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">{issue.title}</td>
            <td className="px-4 py-2.5"><Badge>{STATUS_LABELS[issue.status] || issue.status}</Badge></td>
            <td className="px-4 py-2.5 text-[var(--text-secondary)]">
              {issue.assignee_ids?.map((uid) => users.find((u) => u.id === uid)?.display_name).filter(Boolean).join(", ") || <span className="text-[var(--text-muted)]">–</span>}
            </td>
          </tr>
        ));
      }}
    </SortableTable>
  );
}
