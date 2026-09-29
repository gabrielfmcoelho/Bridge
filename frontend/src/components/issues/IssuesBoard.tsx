"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { globalIssuesAPI, usersAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { IssuesKanban, IssuesTableView } from "./IssueViews";
import { IssueDrawer } from "./IssueDrawer";
import type { HostAlert, Issue } from "@/lib/types";

/** Query key of an asset's issues; anything that creates one (e.g. an alert
 *  turned into an issue) invalidates it. */
export const issuesKey = (entityType: string, entityId: number) => ["issues", entityType, entityId];

/**
 * The issues of one asset (host, dns, service, project): a SectionCard with
 * the kanban/table, an archived toggle and "+", plus the create/edit drawer.
 * Detail pages drop it into their Acompanhamento tab.
 */
export default function IssuesBoard({ entityType, entityId, canEdit, alerts, openCreate, onCreateDone }: {
  entityType: string;
  entityId: number;
  canEdit: boolean;
  /** Host alerts the drawer can link (hosts only). */
  alerts?: HostAlert[];
  /** Phone FAB trigger: opens the create drawer once. */
  openCreate?: boolean;
  onCreateDone?: () => void;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const queryClient = useQueryClient();
  const [view, setView] = useState<"kanban" | "table">("kanban");
  const [showArchived, setShowArchived] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editing, setEditing] = useState<Issue | null>(null);

  const key = issuesKey(entityType, entityId);
  const { data: issues = [] } = useQuery({
    queryKey: key,
    queryFn: () => globalIssuesAPI.list({ entity_type: entityType, entity_id: String(entityId) }),
  });
  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: usersAPI.list });

  // eslint-disable-next-line react-hooks/set-state-in-effect -- one-shot FAB trigger, same as the host tab's
  useEffect(() => { if (openCreate) { setEditing(null); setDrawerOpen(true); onCreateDone?.(); } }, [openCreate]); // eslint-disable-line react-hooks/exhaustive-deps

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: key });
    // Counts on cards and headers.
    queryClient.invalidateQueries({ queryKey: [entityType === "host" ? "hosts" : entityType === "service" ? "services" : entityType] });
  };
  const close = () => { setDrawerOpen(false); setEditing(null); };

  const create = useMutation({
    mutationFn: (data: Partial<Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) =>
      globalIssuesAPI.create({ entity_type: entityType, entity_id: entityId, status: "backlog", ...data }),
    onSuccess: () => { invalidate(); close(); },
  });
  const update = useMutation({
    mutationFn: ({ id, ...data }: Partial<Issue> & { id: number; assignee_ids?: number[]; alert_ids?: number[] }) => globalIssuesAPI.update(id, data),
    onSuccess: () => { invalidate(); close(); },
  });
  const remove = useMutation({ mutationFn: (id: number) => globalIssuesAPI.delete(id), onSuccess: () => { invalidate(); close(); } });
  const move = useMutation({
    mutationFn: ({ id, status, position }: { id: number; status: string; position: number }) => globalIssuesAPI.move(id, status, position),
    onSuccess: invalidate,
  });
  const archive = useMutation({ mutationFn: (id: number) => globalIssuesAPI.archive(id), onSuccess: () => { invalidate(); close(); } });

  const visible = showArchived ? issues : issues.filter((i) => !i.archived);
  const openEdit = (issue: Issue) => { setEditing(issue); setDrawerOpen(true); };

  return (
    <>
      <SectionCard as="h3" title={t("issue.title")} count={visible.length} empty={visible.length === 0 ? t("issue.boardEmpty") : undefined} controls={
        <>
          {issues.some((i) => i.archived) && (
            <IconButton
              variant={showArchived ? "active" : "default"}
              onClick={() => setShowArchived((v) => !v)}
              label={showArchived ? t("issue.hideArchived") : t("issue.showArchived")}
            >
              <Icon path={ICON_PATHS.archive} />
            </IconButton>
          )}
          <ViewToggle
            value={view}
            onChange={(v) => setView(v as "kanban" | "table")}
            options={[
              { key: "kanban", label: "Kanban", icon: VIEW_ICONS.kanban },
              { key: "table", label: t("common.table"), icon: VIEW_ICONS.table },
            ]}
          />
          {canEdit && (
            <span className="hidden md:contents">
              <IconButton onClick={() => { setEditing(null); setDrawerOpen(true); }} label={t("host.addIssue")}><Icon path={ICON_PATHS.plus} /></IconButton>
            </span>
          )}
        </>
      }>
        {view === "kanban" ? (
          <IssuesKanban issues={visible} users={users} onEdit={openEdit} onMove={(id, status, position) => move.mutate({ id, status, position })} />
        ) : (
          <IssuesTableView issues={visible} users={users} onEdit={openEdit} />
        )}
      </SectionCard>

      <IssueDrawer
        open={drawerOpen}
        onClose={close}
        issue={editing}
        users={users}
        entityType={entityType}
        entityId={entityId}
        alerts={alerts}
        onCreate={(data) => create.mutate(data)}
        onUpdate={(id, data) => update.mutate({ id, ...data })}
        onDelete={async (id) => { if (await confirm({ title: t("issue.deleteConfirm"), danger: true, confirmLabel: t("common.delete") })) remove.mutate(id); }}
        onArchive={(id) => archive.mutate(id)}
        loading={create.isPending || update.isPending}
        t={t}
      />
    </>
  );
}
