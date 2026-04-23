"use client";

import { useState } from "react";
import { DragDropContext, Droppable, Draggable, type DropResult } from "@hello-pangea/dnd";
import Badge from "@/components/ui/Badge";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import SortDropdown from "@/components/ui/SortDropdown";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import type { Issue, HostAlert } from "@/lib/types";
import { ALERT_DOT_COLOR, ALERT_TEXT_COLOR, LEVEL_ORDER, PRIORITY_DOT_COLOR } from "../../_components/alert-colors";
import { useLocale } from "@/contexts/LocaleContext";

/* ─── Constants ─── */

const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;
function getStatusLabels(t: (k: string) => string): Record<string, string> {
  return { backlog: t("issue.backlog"), todo: t("issue.todo"), in_progress: t("issue.inProgress"), review: t("issue.review"), done: t("issue.done") };
}

/* ─── Alerts Section ─── */

export function AlertsSection({ alerts, onAlertClick, addButton, showResolved, onToggleResolved, hasResolved }: {
  alerts: HostAlert[];
  onAlertClick: (alert: HostAlert) => void;
  addButton?: React.ReactNode;
  showResolved?: boolean;
  onToggleResolved?: () => void;
  hasResolved?: boolean;
}) {
  const { t } = useLocale();
  const [alertView, setAlertView] = useState<"cards" | "table">("cards");
  const [alertSort, setAlertSort] = useState<"level" | "type">("level");
  const [alertSortDir, setAlertSortDir] = useState<"asc" | "desc">("asc");

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">{t("alert.title")}</h3>
        <div className="flex items-center gap-2">
          {hasResolved && onToggleResolved && (
            <button
              onClick={onToggleResolved}
              className={`inline-flex items-center gap-1 h-[30px] px-2.5 text-xs font-medium rounded-[var(--radius-md)] border transition-all ${
                showResolved
                  ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                  : "bg-[var(--bg-elevated)] text-[var(--text-faint)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
              }`}
              title={showResolved ? t("alert.hideResolved") : t("alert.showResolved")}
            >
              <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                {showResolved ? (
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.878 9.878L3 3m6.878 6.878L21 21" />
                ) : (
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                )}
              </svg>
            </button>
          )}
          {alertView === "cards" && (
            <SortDropdown
              options={[{ key: "level" as const, label: t("alert.level") }, { key: "type" as const, label: t("alert.type") }]}
              value={alertSort}
              direction={alertSortDir}
              onChange={(v, d) => { setAlertSort(v); setAlertSortDir(d); }}
            />
          )}
          <ViewToggle value={alertView} onChange={(v) => setAlertView(v as "cards" | "table")} options={[{ key: "cards", label: t("common.cards"), icon: VIEW_ICONS.cards }, { key: "table", label: t("common.table"), icon: VIEW_ICONS.table }]} />
          {addButton}
        </div>
      </div>

      {alerts.length === 0 ? (
        <EmptyState icon="search" title={t("alert.noAlerts")} description={t("alert.noAlertsDesc")} compact />
      ) : alertView === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4 gap-2">
          {[...alerts].sort((a, b) => { const cmp = alertSort === "type" ? a.type.localeCompare(b.type) : LEVEL_ORDER[a.level] - LEVEL_ORDER[b.level]; return alertSortDir === "desc" ? -cmp : cmp; }).map((alert, i) => (
            <Card key={i} onClick={() => onAlertClick(alert)} clickIndicator="drawer" className="!p-3">
              <div className="flex items-center gap-2 mb-2">
                <span className={`text-sm font-medium truncate flex-1 ${alert.status === "resolved" ? "text-[var(--text-muted)] line-through" : "text-[var(--text-primary)]"}`}>{alert.message}</span>
                {alert.status === "resolved" && (
                  <span title={t("common.resolved")}>
                    <svg className="w-3.5 h-3.5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  </span>
                )}
                <span title={alert.linked_issue_id ? `Issue #${alert.linked_issue_id}` : t("alert.noIssueLinked")}>
                  <svg className={`w-3.5 h-3.5 shrink-0 ${alert.linked_issue_id ? "text-purple-400" : "text-[var(--text-faint)]/30"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                </span>
              </div>
              <div className="grid grid-cols-3 gap-3 text-xs">
                <div>
                  <span className="text-[var(--text-faint)] block mb-0.5">{t("alert.type")}</span>
                  <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{alert.type}</span>
                </div>
                <div>
                  <span className="text-[var(--text-faint)] block mb-0.5">{t("alert.level")}</span>
                  <span className={`capitalize ${ALERT_TEXT_COLOR[alert.level] || "text-[var(--text-secondary)]"}`}>{alert.level}</span>
                </div>
                <div>
                  <span className="text-[var(--text-faint)] block mb-0.5">{t("alert.source")}</span>
                  {alert.source === "grafana" ? (
                    <span className="inline-flex items-center gap-1 text-purple-400">
                      <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
                      </svg>
                      Grafana
                    </span>
                  ) : (
                    <span className="text-[var(--text-muted)]">{alert.source === "manual" ? t("alert.manual") : t("alert.auto")}</span>
                  )}
                </div>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <SortableTable
          columns={[
            { key: "level" as const, label: t("alert.level") },
            { key: "type" as const, label: t("alert.type") },
            { key: "message" as const, label: t("alert.message") },
          ]}
          defaultSort="level"
        >
          {(sk, sd) => {
            const sorted = sortRows(alerts, sk, sd, {
              level: (a, b) => (LEVEL_ORDER[a.level] ?? 2) - (LEVEL_ORDER[b.level] ?? 2),
              type: (a, b) => a.type.localeCompare(b.type),
              message: (a, b) => a.message.localeCompare(b.message),
            });
            return sorted.map((alert, i) => (
              <tr key={i} className={`border-t border-[var(--border-subtle)] cursor-pointer hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`} onClick={() => onAlertClick(alert)}>
                <td className="px-4 py-2.5"><span className="inline-flex items-center gap-1.5"><span className={`w-2 h-2 rounded-full ${ALERT_DOT_COLOR[alert.level]}`} /><span className="capitalize text-[var(--text-secondary)]">{alert.level}</span></span></td>
                <td className="px-4 py-2.5"><span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{alert.type}</span></td>
                <td className="px-4 py-2.5">
                  <span className={`inline-flex items-center gap-1.5 ${alert.status === "resolved" ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>
                    {alert.status === "resolved" && (
                      <svg className="w-3.5 h-3.5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                    )}
                    {alert.message}
                  </span>
                </td>
              </tr>
            ));
          }}
        </SortableTable>
      )}
    </div>
  );
}

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

  return (
    <DragDropContext onDragEnd={handleDragEnd}>
      <div className="overflow-x-auto -mx-1 px-1 pb-2">
        <div className="flex gap-3 min-w-[1200px]">
          {STATUSES.map((status) => {
            const items = issues.filter((i) => i.status === status);
            return (
              <div key={status} className="flex-1 min-w-[220px]">
                <div className="flex items-center gap-1.5 mb-2 px-1">
                  <span className="text-[10px] font-semibold text-[var(--text-faint)] uppercase tracking-wider">{STATUS_LABELS[status]}</span>
                  {items.length > 0 && <span className="text-[10px] text-[var(--text-faint)] bg-[var(--bg-elevated)] rounded-full px-1.5">{items.length}</span>}
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
                                <Card
                                  onClick={() => onEdit(issue)}
                                  clickIndicator="drawer"
                                  className={`!p-3 ${
                                    dragSnapshot.isDragging
                                      ? "shadow-lg !border-[var(--accent)]/40 ring-1 ring-[var(--accent)]/20"
                                      : ""
                                  }`}
                                >
                                  <div className="flex items-start gap-1.5 mb-4 min-h-[1.25rem]">
                                    {issue.status === "done" && (
                                      <svg className="w-4 h-4 shrink-0 mt-0.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                      </svg>
                                    )}
                                    {issue.archived && (
                                      <svg className="w-4 h-4 shrink-0 mt-0.5 text-[var(--text-faint)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                                      </svg>
                                    )}
                                    <p className={`text-sm font-medium leading-snug line-clamp-3 ${issue.archived ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{issue.title}</p>
                                  </div>
                                  <div className="grid grid-cols-2 gap-3 text-xs">
                                    <div>
                                      <span className="text-[var(--text-faint)] block mb-0.5">{t("common.priority")}</span>
                                      <span className="text-[var(--text-muted)] capitalize">{issue.priority}</span>
                                    </div>
                                    <div>
                                      <span className="text-[var(--text-faint)] block mb-0.5">{t("common.assignees")}</span>
                                      {assignees.length > 0 ? (
                                        <div className="flex -space-x-1">
                                          {assignees.map((user) => (
                                            <span key={user!.id} className="w-5 h-5 rounded-full bg-[var(--accent-muted)] text-[var(--accent)] text-[8px] font-bold flex items-center justify-center border border-[var(--bg-surface)]" title={user!.display_name}>
                                              {getInitials(user!.display_name)}
                                            </span>
                                          ))}
                                          {(issue.assignee_ids?.length || 0) > 3 && <span className="w-5 h-5 rounded-full bg-[var(--bg-overlay)] text-[var(--text-faint)] text-[8px] font-bold flex items-center justify-center border border-[var(--bg-surface)]">+{issue.assignee_ids!.length - 3}</span>}
                                        </div>
                                      ) : (
                                        <span className="text-[var(--text-faint)]">--</span>
                                      )}
                                    </div>
                                    <div>
                                      <span className="text-[var(--text-faint)] block mb-0.5">{t("issue.due")}</span>
                                      <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>
                                        {issue.expected_end_date || "--"}
                                      </span>
                                    </div>
                                    <div>
                                      <span className="text-[var(--text-faint)] block mb-0.5">{t("issue.source")}</span>
                                      <span className="text-[var(--text-muted)] capitalize">{issue.source || "manual"}</span>
                                    </div>
                                  </div>
                                  {/* Entity link icons */}
                                  <div className="flex items-center gap-2 mt-3 pt-2 border-t border-[var(--border-subtle)]">
                                    <span className={`${issue.entity_type === "host" ? "text-emerald-400" : "text-[var(--text-faint)]/30"}`} title="Host">
                                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
                                      </svg>
                                    </span>
                                    <span className={`${issue.entity_type === "dns" ? "text-cyan-400" : "text-[var(--text-faint)]/30"}`} title="DNS">
                                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9" />
                                      </svg>
                                    </span>
                                    <span className={`${issue.entity_type === "service" ? "text-amber-400" : "text-[var(--text-faint)]/30"}`} title="Service">
                                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                                      </svg>
                                    </span>
                                    <span className={`${issue.entity_type === "project" ? "text-violet-400" : "text-[var(--text-faint)]/30"}`} title="Project">
                                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
                                      </svg>
                                    </span>
                                    <span className={`${(issue.alert_ids?.length || 0) > 0 || issue.source === "alert" ? "text-amber-400" : "text-[var(--text-faint)]/30"}`} title="Alert">
                                      <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                      </svg>
                                    </span>
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

  if (issues.length === 0) return <EmptyState icon="search" title={t("issue.noIssues")} description={t("issue.noIssuesDesc")} compact />;

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
              {issue.assignee_ids?.map((uid) => users.find((u) => u.id === uid)?.display_name).filter(Boolean).join(", ") || <span className="text-[var(--text-faint)]">-</span>}
            </td>
          </tr>
        ));
      }}
    </SortableTable>
  );
}
