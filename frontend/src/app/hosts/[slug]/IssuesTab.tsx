"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { globalIssuesAPI, usersAPI, hostAlertsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import { AlertsSection, IssuesKanban, IssuesTableView } from "./_components/IssueViews";
import { AlertDrawer, AlertDetailDrawer, IssueDrawer } from "./_components/IssueDrawers";
import ChamadoSection from "./_components/ChamadoSection";
import type { Issue, HostAlert, HostChamado } from "@/lib/types";

function alertToPriority(level: string): string {
  return level === "critical" ? "critical" : level === "warning" ? "high" : "medium";
}

export default function IssuesTab({ hostAlerts, chamados, hostId, slug, canEdit, openAlertCreate, onAlertCreateDone, openIssueCreate, onIssueCreateDone, openChamadoCreate, onChamadoCreateDone }: {
  hostAlerts: HostAlert[];
  chamados: HostChamado[];
  hostId: number;
  slug: string;
  canEdit: boolean;
  openAlertCreate?: boolean;
  onAlertCreateDone?: () => void;
  openIssueCreate?: boolean;
  onIssueCreateDone?: () => void;
  openChamadoCreate?: boolean;
  onChamadoCreateDone?: () => void;
}) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [issueView, setIssueView] = useState<"kanban" | "table">("kanban");
  const [showResolvedAlerts, setShowResolvedAlerts] = useState(false);
  const [showArchivedIssues, setShowArchivedIssues] = useState(false);
  const [showAlertDrawer, setShowAlertDrawer] = useState(false);
  const [selectedAlert, setSelectedAlert] = useState<HostAlert | null>(null);
  const [showAlertDetailDrawer, setShowAlertDetailDrawer] = useState(false);
  const [showIssueDrawer, setShowIssueDrawer] = useState(false);
  const [editingIssue, setEditingIssue] = useState<Issue | null>(null);

  // FAB triggers
  useEffect(() => { if (openAlertCreate) { setShowAlertDrawer(true); onAlertCreateDone?.(); } }, [openAlertCreate]);
  useEffect(() => { if (openIssueCreate) { setEditingIssue(null); setShowIssueDrawer(true); onIssueCreateDone?.(); } }, [openIssueCreate]);

  // Queries
  const { data: hostIssues = [] } = useQuery({
    queryKey: ["issues", "host", hostId],
    queryFn: () => globalIssuesAPI.list({ entity_type: "host", entity_id: String(hostId) }),
  });
  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: usersAPI.list });
  // hostAlerts already contains both auto + manual alerts (merged by backend)
  const allAlerts = hostAlerts;

  const invalidateAlerts = () => {
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
  };

  // Alert mutations
  const createAlertMutation = useMutation({
    mutationFn: (data: { type: string; level: string; message: string; description: string }) =>
      hostAlertsAPI.create(slug, data),
    onSuccess: () => { invalidateAlerts(); setShowAlertDrawer(false); },
  });

  const updateAlertMutation = useMutation({
    mutationFn: ({ id, ...data }: { id: number; type: string; level: string; message: string; description: string }) =>
      hostAlertsAPI.update(slug, id, data),
    onSuccess: () => { invalidateAlerts(); setShowAlertDetailDrawer(false); },
  });

  const deleteAlertMutation = useMutation({
    mutationFn: (id: number) => hostAlertsAPI.delete(slug, id),
    onSuccess: () => { invalidateAlerts(); setShowAlertDetailDrawer(false); },
  });

  const concludeAlertMutation = useMutation({
    mutationFn: (id: number) => hostAlertsAPI.conclude(slug, id),
    onSuccess: () => { invalidateAlerts(); setShowAlertDetailDrawer(false); },
  });

  // Issue mutations
  const createIssueMutation = useMutation({
    mutationFn: (data: Partial<Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) =>
      globalIssuesAPI.create({ entity_type: "host", entity_id: hostId, status: "backlog", ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["issues", "host", hostId] });
      invalidateAlerts();
    },
  });

  const updateIssueMutation = useMutation({
    mutationFn: ({ id, ...data }: Partial<Issue> & { id: number; assignee_ids?: number[]; alert_ids?: number[] }) =>
      globalIssuesAPI.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["issues", "host", hostId] });
      invalidateAlerts();
    },
  });

  const deleteIssueMutation = useMutation({
    mutationFn: (id: number) => globalIssuesAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["issues", "host", hostId] }),
  });

  const moveIssueMutation = useMutation({
    mutationFn: ({ id, status, position }: { id: number; status: string; position: number }) =>
      globalIssuesAPI.move(id, status, position),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["issues", "host", hostId] });
      invalidateAlerts();
    },
  });

  const archiveIssueMutation = useMutation({
    mutationFn: (id: number) => globalIssuesAPI.archive(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["issues", "host", hostId] }),
  });

  const openEditIssue = (issue: Issue) => {
    setEditingIssue(issue);
    setShowIssueDrawer(true);
  };

  const closeIssueDrawer = () => {
    setShowIssueDrawer(false);
    setEditingIssue(null);
  };

  const openAlertDetail = (alert: HostAlert) => {
    setSelectedAlert(alert);
    setShowAlertDetailDrawer(true);
  };

  const createIssueFromAlert = async (alert: HostAlert) => {
    let alertId = alert.id;

    // Auto alerts have no DB ID — persist first so vinculos can be created
    if (!alertId) {
      try {
        const persisted = await hostAlertsAPI.create(slug, {
          type: alert.type,
          level: alert.level,
          message: alert.message,
          description: alert.description || "",
          source: "auto",
        });
        alertId = persisted.id;
      } catch {
        // Fall through — create issue without vinculos
      }
    }

    createIssueMutation.mutate({
      title: alert.message,
      priority: alertToPriority(alert.level),
      source: "alert",
      source_ref: alert.type,
      alert_ids: alertId ? [alertId] : undefined,
    });
  };

  return (
    <div className="space-y-8 animate-fade-in">
      {/* ══════ CHAMADOS SECTION ══════ */}
      <ChamadoSection
        chamados={chamados}
        hostId={hostId}
        slug={slug}
        canEdit={canEdit}
        t={t}
        openCreate={openChamadoCreate}
        onCreateDone={onChamadoCreateDone}
      />

      {/* ══════ ALERTS SECTION ══════ */}
      <AlertsSection
        alerts={showResolvedAlerts ? allAlerts : allAlerts.filter(a => a.status !== "resolved")}
        onAlertClick={openAlertDetail}
        showResolved={showResolvedAlerts}
        onToggleResolved={() => setShowResolvedAlerts(v => !v)}
        hasResolved={allAlerts.some(a => a.status === "resolved")}
        addButton={
          canEdit ? (
            <span className="hidden md:contents">
              <Button size="sm" onClick={() => setShowAlertDrawer(true)}>
                <span className="mr-1">+</span> {t("host.addAlert")}
              </Button>
            </span>
          ) : undefined
        }
      />

      {/* ══════ ISSUES SECTION ══════ */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">Issues</h3>
          <div className="flex items-center gap-1.5">
            {hostIssues.some(i => i.archived) && (
              <button
                onClick={() => setShowArchivedIssues(v => !v)}
                className={`inline-flex items-center gap-1 h-[30px] px-2.5 text-xs font-medium rounded-[var(--radius-md)] border transition-all ${
                  showArchivedIssues
                    ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                    : "bg-[var(--bg-elevated)] text-[var(--text-faint)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
                }`}
                title={showArchivedIssues ? t("issue.hideArchived") : t("issue.showArchived")}
              >
                <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                </svg>
              </button>
            )}
            <ViewToggle
              value={issueView}
              onChange={(v) => setIssueView(v as "kanban" | "table")}
              options={[
                { key: "kanban", label: "Kanban", icon: VIEW_ICONS.kanban },
                { key: "table", label: t("common.table"), icon: VIEW_ICONS.table },
              ]}
            />
            {canEdit && (
              <span className="hidden md:contents">
                <Button size="sm" onClick={() => { setEditingIssue(null); setShowIssueDrawer(true); }}>
                  <span className="mr-1">+</span> {t("host.addIssue")}
                </Button>
              </span>
            )}
          </div>
        </div>

        {issueView === "kanban" ? (
          <IssuesKanban issues={showArchivedIssues ? hostIssues : hostIssues.filter(i => !i.archived)} users={users} onEdit={openEditIssue} onMove={(id, status, position) => moveIssueMutation.mutate({ id, status, position })} />
        ) : (
          <IssuesTableView issues={showArchivedIssues ? hostIssues : hostIssues.filter(i => !i.archived)} users={users} onEdit={openEditIssue} />
        )}
      </div>

      {/* ══════ DRAWERS ══════ */}
      <AlertDrawer
        open={showAlertDrawer}
        onClose={() => setShowAlertDrawer(false)}
        onSave={(data) => createAlertMutation.mutate(data)}
        loading={createAlertMutation.isPending}
        t={t}
        knownTypes={allAlerts.map((a) => a.type)}
      />

      <AlertDetailDrawer
        open={showAlertDetailDrawer}
        onClose={() => setShowAlertDetailDrawer(false)}
        alert={selectedAlert}
        slug={slug}
        canEdit={canEdit}
        onCreateIssue={createIssueFromAlert}
        onConclude={(alert) => {
          if (alert.id) concludeAlertMutation.mutate(alert.id);
        }}
        onUpdate={(alert, data) => {
          if (alert.id) updateAlertMutation.mutate({ id: alert.id, ...data });
        }}
        onDelete={(alert) => {
          if (alert.id) deleteAlertMutation.mutate(alert.id);
        }}
        createLoading={createIssueMutation.isPending}
        concludeLoading={concludeAlertMutation.isPending}
        updateLoading={updateAlertMutation.isPending}
        t={t}
      />

      <IssueDrawer
        open={showIssueDrawer}
        onClose={closeIssueDrawer}
        issue={editingIssue}
        users={users}
        hostId={hostId}
        alerts={allAlerts}
        onCreate={(data) => { createIssueMutation.mutate(data, { onSuccess: closeIssueDrawer }); }}
        onUpdate={(id, data) => { updateIssueMutation.mutate({ id, ...data }, { onSuccess: closeIssueDrawer }); }}
        onDelete={(id) => { if (confirm(t("issue.deleteConfirm"))) deleteIssueMutation.mutate(id, { onSuccess: closeIssueDrawer }); }}
        onArchive={(id) => { archiveIssueMutation.mutate(id, { onSuccess: closeIssueDrawer }); }}
        loading={createIssueMutation.isPending || updateIssueMutation.isPending}
        t={t}
      />
    </div>
  );
}
