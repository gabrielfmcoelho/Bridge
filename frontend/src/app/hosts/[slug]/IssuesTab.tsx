"use client";

import { useState, useEffect } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { globalIssuesAPI, hostAlertsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import IconButton from "@/components/ui/IconButton";
import { AlertsSection } from "./_components/IssueViews";
import IssuesBoard, { issuesKey } from "@/components/issues/IssuesBoard";
import { AlertDrawer, AlertDetailDrawer } from "./_components/IssueDrawers";
import ChamadoSection from "./_components/ChamadoSection";
import type { Issue, HostAlert, HostChamado } from "@/lib/types";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
  const [showResolvedAlerts, setShowResolvedAlerts] = useState(false);
  const [showAlertDrawer, setShowAlertDrawer] = useState(false);
  const [selectedAlert, setSelectedAlert] = useState<HostAlert | null>(null);
  const [showAlertDetailDrawer, setShowAlertDetailDrawer] = useState(false);

  // FAB triggers
  useEffect(() => { if (openAlertCreate) { setShowAlertDrawer(true); onAlertCreateDone?.(); } }, [openAlertCreate]);

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

  // Turning an alert into an issue; the board below owns every other issue write.
  const createIssueMutation = useMutation({
    mutationFn: (data: Partial<Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) =>
      globalIssuesAPI.create({ entity_type: "host", entity_id: hostId, status: "backlog", ...data }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: issuesKey("host", hostId) });
      invalidateAlerts();
    },
  });

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
    <div className="space-y-5 animate-fade-in">
      {/* Signals (alerts from scans, tickets from outside) side by side; the work they turn into spans below. */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5 items-start">
        <AlertsSection
          alerts={showResolvedAlerts ? allAlerts : allAlerts.filter(a => a.status !== "resolved")}
          onAlertClick={openAlertDetail}
          showResolved={showResolvedAlerts}
          onToggleResolved={() => setShowResolvedAlerts(v => !v)}
          hasResolved={allAlerts.some(a => a.status === "resolved")}
          addButton={
            canEdit ? (
              <span className="hidden md:contents">
                <IconButton onClick={() => setShowAlertDrawer(true)} label={t("host.addAlert")}><Icon path={ICON_PATHS.plus} /></IconButton>
              </span>
            ) : undefined
          }
        />
        <div className="space-y-5 min-w-0">
          <ChamadoSection
            chamados={chamados}
            hostId={hostId}
            slug={slug}
            canEdit={canEdit}
            t={t}
            openCreate={openChamadoCreate}
            onCreateDone={onChamadoCreateDone}
          />
        </div>
      </div>

      {/* ══════ ISSUES SECTION ══════ */}
      <IssuesBoard entityType="host" entityId={hostId} canEdit={canEdit} alerts={allAlerts} openCreate={openIssueCreate} onCreateDone={onIssueCreateDone} />

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

    </div>
  );
}
