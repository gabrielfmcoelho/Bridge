"use client";

import { useState, useMemo, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { hostsAPI, dnsAPI, servicesAPI, projectsAPI, integrationsAPI } from "@/lib/api";
import Button from "@/components/ui/Button";
import CreateTicketModal from "@/components/glpi/CreateTicketModal";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Field from "@/components/ui/Field";
import Badge from "@/components/ui/Badge";
import MarkdownEditor, { MarkdownContent } from "@/components/ui/MarkdownEditor";
import CheckboxList from "@/components/ui/CheckboxList";
import StepIndicator from "@/components/ui/StepIndicator";
import type { Issue, HostAlert } from "@/lib/types";
import { ALERT_DOT_COLOR, ALERT_TEXT_COLOR, PRIORITY_DOT_COLOR } from "../../_components/alert-colors";

/* ─── Constants ─── */

const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;
function getStatusLabels(t: (k: string) => string): Record<string, string> {
  return { backlog: t("issue.backlog"), todo: t("issue.todo"), in_progress: t("issue.inProgress"), review: t("issue.review"), done: t("issue.done") };
}

function LinkReadRow({ label, items }: { label: string; items: string[] }) {
  return (
    <div className="flex items-start gap-2">
      <span className="text-[10px] text-[var(--text-faint)] uppercase tracking-wider w-16 shrink-0 pt-0.5">{label}</span>
      {items.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {items.map((name) => (
            <span key={name} className="text-xs px-2 py-0.5 rounded-full bg-[var(--bg-elevated)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">{name}</span>
          ))}
        </div>
      ) : (
        <span className="text-xs text-[var(--text-faint)]">--</span>
      )}
    </div>
  );
}

/* ═══════════════════════════════════════════════════════════════════
   Alert Drawer — Create manual alert
   ═══════════════════════════════════════════════════════════════════ */

export function AlertDrawer({ open, onClose, onSave, loading, t, knownTypes }: {
  open: boolean;
  onClose: () => void;
  onSave: (data: { type: string; level: string; message: string; description: string }) => void;
  loading?: boolean;
  t: (k: string) => string;
  knownTypes: string[];
}) {
  const [type, setType] = useState("");
  const [customType, setCustomType] = useState("");
  const [level, setLevel] = useState("warning");
  const [message, setMessage] = useState("");
  const [description, setDescription] = useState("");

  useEffect(() => {
    if (open) { setType(""); setCustomType(""); setLevel("warning"); setMessage(""); setDescription(""); }
  }, [open]);

  const typeOptions = useMemo(() => {
    const unique = [...new Set(knownTypes)];
    return [...unique.map((v) => ({ value: v, label: v })), { value: "__custom__", label: t("alert.custom") }];
  }, [knownTypes, t]);

  const effectiveType = type === "__custom__" ? customType : type;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("alert.addAlert")}
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>{t("common.cancel")}</Button>
          <Button size="sm" className="flex-1" disabled={!effectiveType.trim() || !message.trim()} loading={loading}
            onClick={() => onSave({ type: effectiveType, level, message, description })}>
            {t("common.create")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-xs text-[var(--text-faint)]">{t("alert.manualDesc")}</p>
        <Select label={t("alert.type")} value={type} onChange={(e) => setType(e.target.value)} options={typeOptions} />
        {type === "__custom__" && (
          <Input label={t("alert.customType")} value={customType} onChange={(e) => setCustomType(e.target.value)} placeholder={t("alert.customTypePlaceholder")} />
        )}
        <Select label={t("alert.level")} value={level} onChange={(e) => setLevel(e.target.value)} options={[{ value: "critical", label: t("alert.critical") }, { value: "warning", label: t("alert.warning") }, { value: "info", label: t("alert.info") }]} />
        <Input label={t("alert.message")} value={message} onChange={(e) => setMessage(e.target.value)} />
        <MarkdownEditor label={t("common.description")} value={description} onChange={setDescription} rows={4} placeholder={t("alert.optionalDetails")} />
      </div>
    </Drawer>
  );
}

/* ═══════════════════════════════════════════════════════════════════
   Alert Detail Drawer — Read / Edit / Delete
   ═══════════════════════════════════════════════════════════════════ */

export function AlertDetailDrawer({ open, onClose, alert, slug, canEdit, onCreateIssue, onConclude, onUpdate, onDelete, createLoading, concludeLoading, updateLoading, t }: {
  open: boolean;
  onClose: () => void;
  alert: HostAlert | null;
  slug?: string;
  canEdit: boolean;
  onCreateIssue: (alert: HostAlert) => void;
  onConclude?: (alert: HostAlert) => void;
  onUpdate?: (alert: HostAlert, data: { type: string; level: string; message: string; description: string }) => void;
  onDelete?: (alert: HostAlert) => void;
  createLoading?: boolean;
  concludeLoading?: boolean;
  updateLoading?: boolean;
  t: (k: string) => string;
}) {
  const [editing, setEditing] = useState(false);
  const [escalateOpen, setEscalateOpen] = useState(false);

  // Integration status controls visibility of the Escalate-to-GLPI affordance.
  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";
  const [editType, setEditType] = useState("");
  const [editLevel, setEditLevel] = useState("");
  const [editMessage, setEditMessage] = useState("");
  const [editDescription, setEditDescription] = useState("");

  useEffect(() => {
    if (open && alert) {
      setEditing(false);
      setEditType(alert.type);
      setEditLevel(alert.level);
      setEditMessage(alert.message);
      setEditDescription(alert.description || "");
    }
  }, [open, alert]);

  if (!alert) return null;

  const isManual = alert.source === "manual";
  const hasLinkedIssue = !!alert.linked_issue_id;
  const isResolved = alert.status === "resolved";
  const canConclude = canEdit && isManual && !hasLinkedIssue && !isResolved;

  const readFooter = (
    <div className="flex gap-2 flex-wrap">
      <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>{t("common.close")}</Button>
      {canEdit && isManual && !isResolved && (
        <Button size="sm" className="flex-1" onClick={() => setEditing(true)}>{t("common.edit")}</Button>
      )}
      {canConclude && onConclude && (
        <Button variant="secondary" size="sm" className="flex-1" onClick={() => onConclude(alert)} loading={concludeLoading}>
          {t("common.conclude") || "Conclude"}
        </Button>
      )}
      {canEdit && !hasLinkedIssue && !isResolved && (
        <Button size="sm" className="flex-1" onClick={() => { onCreateIssue(alert); onClose(); }} loading={createLoading}>
          + {t("common.createIssue")}
        </Button>
      )}
      {canEdit && glpiEnabled && slug && !isResolved && (
        <Button variant="secondary" size="sm" className="flex-1" onClick={() => setEscalateOpen(true)}>
          + GLPI
        </Button>
      )}
    </div>
  );

  const editFooter = (
    <div className="flex gap-2">
      {isManual && onDelete && (
        <Button variant="danger" size="sm" onClick={() => { if (confirm(t("alert.deleteConfirm"))) { onDelete(alert); onClose(); } }} className="mr-auto">
          {t("common.delete")}
        </Button>
      )}
      <Button variant="secondary" size="sm" className="flex-1" onClick={() => setEditing(false)}>{t("common.cancel")}</Button>
      <Button size="sm" className="flex-1" loading={updateLoading} disabled={!editType.trim() || !editMessage.trim()}
        onClick={() => { onUpdate?.(alert, { type: editType, level: editLevel, message: editMessage, description: editDescription }); setEditing(false); }}>
        {t("common.save")}
      </Button>
    </div>
  );

  if (editing) {
    return (
      <Drawer open={open} onClose={onClose} title={t("alert.editAlert")} footer={editFooter}>
        <div className="space-y-4">
          <Input label={t("alert.type")} value={editType} onChange={(e) => setEditType(e.target.value)} />
          <Select label={t("alert.level")} value={editLevel} onChange={(e) => setEditLevel(e.target.value)} options={[{ value: "critical", label: t("alert.critical") }, { value: "warning", label: t("alert.warning") }, { value: "info", label: t("alert.info") }]} />
          <Input label={t("alert.message")} value={editMessage} onChange={(e) => setEditMessage(e.target.value)} />
          <MarkdownEditor label={t("common.description")} value={editDescription} onChange={setEditDescription} rows={4} />
        </div>
      </Drawer>
    );
  }

  return (
    <>
    <Drawer open={open} onClose={onClose} title={t("alert.detail")} footer={readFooter}>
      <div className="space-y-4">
        <p className="text-base font-semibold text-[var(--text-primary)]">{alert.message}</p>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("alert.level")}</span>
            <div className="flex items-center gap-1.5">
              <span className={`w-2.5 h-2.5 rounded-full shrink-0 ${ALERT_DOT_COLOR[alert.level] || "bg-[var(--text-faint)]"}`} />
              <span className={`text-sm capitalize ${ALERT_TEXT_COLOR[alert.level] || "text-[var(--text-secondary)]"}`}>{alert.level}</span>
            </div>
          </div>
          <Field label={t("alert.source")} value={alert.source === "grafana" ? "Grafana (external)" : isManual ? t("alert.manual") : t("alert.auto")} />
        </div>

        <Field label={t("alert.type")} value={alert.type} mono />

        <div>
          <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("common.description")}</span>
          {alert.description ? (
            <MarkdownContent content={alert.description} />
          ) : (
            <span className="text-sm text-[var(--text-primary)]">--</span>
          )}
        </div>

        {isResolved && (
          <div className="flex items-center gap-2 text-xs text-emerald-400 bg-emerald-500/10 rounded-[var(--radius-md)] p-2.5 border border-emerald-500/20">
            <svg className="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            {t("common.resolved") || "Resolved"}
          </div>
        )}

        {hasLinkedIssue && (
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-2.5 border border-[var(--border-subtle)]">
            <svg className="w-3.5 h-3.5 text-purple-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            {isResolved
              ? `Issue #${alert.linked_issue_id} — resolved`
              : `Issue #${alert.linked_issue_id} linked — resolve the issue to conclude this alert`
            }
          </div>
        )}
      </div>
    </Drawer>
    {slug && (
      <CreateTicketModal
        open={escalateOpen}
        onClose={() => setEscalateOpen(false)}
        defaultTitle={`[alerta] ${alert.message}`}
        defaultDescription={
          `**Alerta:** ${alert.message}\n\n` +
          `- **Tipo:** ${alert.type}\n` +
          `- **Nível:** ${alert.level}\n` +
          (alert.description ? `\n${alert.description}\n` : "")
        }
        hostSlug={slug}
        alertID={alert.id}
        onCreated={onClose}
      />
    )}
    </>
  );
}

/* ═══════════════════════════════════════════════════════════════════
   Issue Drawer — Read / Edit / Create (stepped)
   ═══════════════════════════════════════════════════════════════════ */

export function IssueDrawer({ open, onClose, issue, users, hostId, alerts, onCreate, onUpdate, onDelete, onArchive, loading, t }: {
  open: boolean;
  onClose: () => void;
  issue: Issue | null;
  users: { id: number; display_name: string; username: string }[];
  hostId: number;
  alerts?: HostAlert[];
  onCreate: (data: Partial<Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) => void;
  onUpdate: (id: number, data: Partial<Issue> & { assignee_ids?: number[]; alert_ids?: number[] }) => void;
  onDelete: (id: number) => void;
  onArchive?: (id: number) => void;
  loading: boolean;
  t: (k: string) => string;
}) {
  const isEdit = !!issue;
  const STATUS_LABELS = getStatusLabels(t);
  const [mode, setMode] = useState<"read" | "edit" | "create">("create");
  const [step, setStep] = useState(1);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState("medium");
  const [status, setStatus] = useState("backlog");
  const [expectedEndDate, setExpectedEndDate] = useState("");
  const [assigneeIds, setAssigneeIds] = useState<number[]>([]);
  const [openSection, setOpenSection] = useState<string | null>(null);
  const toggleSection = (key: string) => setOpenSection((prev) => (prev === key ? null : key));

  // Entity data for linking
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allDns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const [linkedHostIds, setLinkedHostIds] = useState<number[]>([]);
  const [linkedDnsIds, setLinkedDnsIds] = useState<number[]>([]);
  const [linkedServiceIds, setLinkedServiceIds] = useState<number[]>([]);
  const [linkedProjectIds, setLinkedProjectIds] = useState<number[]>([]);
  const [linkedAlertIds, setLinkedAlertIds] = useState<number[]>([]);

  // Alert items for CheckboxList (only manual alerts with IDs)
  const alertItems = (alerts || []).filter((a) => a.id).map((a) => ({ id: a.id!, name: `[${a.level}] ${a.message}` }));

  useEffect(() => {
    if (open) {
      if (issue) {
        setTitle(issue.title);
        setDescription(issue.description || "");
        setPriority(issue.priority || "medium");
        setStatus(issue.status || "backlog");
        setExpectedEndDate(issue.expected_end_date || "");
        setAssigneeIds(issue.assignee_ids || []);
        // Derive linked entity IDs from the issue's entity_type/entity_id
        setLinkedHostIds(issue.entity_type === "host" && issue.entity_id ? [issue.entity_id] : []);
        setLinkedDnsIds(issue.entity_type === "dns" && issue.entity_id ? [issue.entity_id] : []);
        setLinkedServiceIds(issue.entity_type === "service" && issue.entity_id ? [issue.entity_id] : []);
        setLinkedProjectIds(issue.entity_type === "project" && issue.entity_id ? [issue.entity_id] : []);
        // Use alert_ids from join table, fall back to legacy alert_id
        const ids = issue.alert_ids?.length ? issue.alert_ids : (issue.alert_id ? [issue.alert_id] : []);
        setLinkedAlertIds(ids);
        setMode("read");
        setOpenSection(null);
      } else {
        setTitle(""); setDescription(""); setPriority("medium"); setStatus("backlog");
        setExpectedEndDate(""); setAssigneeIds([]); setStep(1); setMode("create");
        setLinkedHostIds([hostId]); setLinkedDnsIds([]); setLinkedServiceIds([]); setLinkedProjectIds([]);
        setLinkedAlertIds([]);
        setOpenSection(null);
      }
    }
  }, [open, issue, hostId]);

  const canProceedStep1 = title.trim().length > 0;
  const stepLabels = [t("common.basicInfo"), t("common.links") || "Links"];

  const handleSubmit = () => {
    const data = {
      title, description, priority, status, expected_end_date: expectedEndDate,
      assignee_ids: assigneeIds,
      source: issue?.source || "manual",
      entity_type: "host",
      entity_id: hostId,
      alert_ids: linkedAlertIds,
    };
    if (isEdit) {
      onUpdate(issue.id, data);
    } else {
      onCreate(data);
    }
  };

  const getInitials = (name: string) => {
    const parts = name.trim().split(/\s+/);
    return parts.length === 1 ? parts[0].slice(0, 2).toUpperCase() : (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  };

  /* ─── READ MODE ─── */
  if (mode === "read" && issue) {
    return (
      <Drawer
        open={open}
        onClose={onClose}
        title={t("issue.detail")}
        footer={
          <div className="flex gap-2">
            {issue.status === "done" && onArchive && (
              <Button variant="secondary" size="sm" onClick={() => onArchive(issue.id)} className="mr-auto">
                <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
                </svg>
                {issue.archived ? t("issue.unarchive") : t("issue.archive")}
              </Button>
            )}
            <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>{t("common.close")}</Button>
            <Button size="sm" className="flex-1" onClick={() => setMode("edit")}>{t("common.edit")}</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <div className="flex items-center gap-2">
            <p className="text-base font-semibold text-[var(--text-primary)] flex-1">{issue.title}</p>
            {issue.status === "done" && (
              <svg className="w-5 h-5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            )}
            {issue.archived && (
              <Badge>{t("issue.archived")}</Badge>
            )}
          </div>

          {/* Priority + Status */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("common.priority")}</span>
              <div className="flex items-center gap-1.5">
                <span className={`w-2.5 h-2.5 rounded-full ${PRIORITY_DOT_COLOR[issue.priority] || "bg-[var(--text-faint)]"}`} />
                <span className="text-sm text-[var(--text-primary)] capitalize">{issue.priority}</span>
              </div>
            </div>
            <div>
              <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("common.status")}</span>
              <Badge>{STATUS_LABELS[issue.status] || issue.status}</Badge>
            </div>
          </div>

          {/* Assignees */}
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-1.5">{t("common.assignees")}</span>
            {issue.assignee_ids && issue.assignee_ids.length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {issue.assignee_ids.map((uid) => {
                  const user = users.find((u) => u.id === uid);
                  return user ? (
                    <span key={uid} className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20">
                      <span className="w-4 h-4 rounded-full bg-[var(--accent)] text-white text-[8px] font-bold flex items-center justify-center">{getInitials(user.display_name)}</span>
                      {user.display_name}
                    </span>
                  ) : null;
                })}
              </div>
            ) : (
              <span className="text-sm text-[var(--text-primary)]">--</span>
            )}
          </div>

          {/* Dates */}
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("issue.expectedEndDate")} value={issue.expected_end_date || ""} />
            <Field label={t("issue.startDate")} value={issue.start_date || ""} />
            <Field label={t("issue.endDate")} value={issue.end_date || ""} />
          </div>

          {/* Links */}
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-1.5">{t("common.links")}</span>
            <div className="space-y-2">
              <LinkReadRow label={t("host.title")} items={allHosts.filter((h) => linkedHostIds.includes(h.id)).map((h) => h.nickname)} />
              <LinkReadRow label={t("dns.title")} items={allDns.filter((d) => linkedDnsIds.includes(d.id)).map((d) => d.domain)} />
              <LinkReadRow label={t("service.title")} items={allServices.filter((s) => linkedServiceIds.includes(s.id)).map((s) => s.nickname)} />
              <LinkReadRow label={t("project.title")} items={allProjects.filter((p) => linkedProjectIds.includes(p.id)).map((p) => p.name)} />
              <LinkReadRow label={t("alert.title")} items={alertItems.filter((a) => linkedAlertIds.includes(a.id)).map((a) => a.name)} />
            </div>
          </div>

          {/* Source info */}
          {issue.source === "alert" && (
            <div className="flex items-center gap-2 text-xs text-[var(--text-faint)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-2.5 border border-[var(--border-subtle)]">
              <svg className="w-3.5 h-3.5 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              {t("alert.createdFromAlert")} {issue.source_ref}
            </div>
          )}

          {/* Description */}
          <div>
            <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("common.description")}</span>
            {issue.description ? (
              <MarkdownContent content={issue.description} />
            ) : (
              <span className="text-sm text-[var(--text-primary)]">--</span>
            )}
          </div>
        </div>
      </Drawer>
    );
  }

  /* ─── EDIT MODE ─── */
  if (mode === "edit" && issue) {
    return (
      <Drawer
        open={open}
        onClose={onClose}
        title={t("issue.editIssue")}
        footer={
          <div className="flex gap-2">
            <Button variant="danger" size="sm" onClick={() => { if (confirm(t("issue.deleteConfirm"))) onDelete(issue.id); }} className="mr-auto">
              {t("common.delete")}
            </Button>
            <Button variant="secondary" size="sm" className="flex-1" onClick={() => setMode("read")}>{t("common.cancel")}</Button>
            <Button size="sm" className="flex-1" disabled={!title.trim()} loading={loading} onClick={handleSubmit}>{t("common.save")}</Button>
          </div>
        }
      >
        <div className="space-y-0">
          <DrawerSection title={t("common.basicInfo")} open={openSection === "basic"} onToggle={() => toggleSection("basic")} active={!!title}>
            <Input label={t("common.title")} value={title} onChange={(e) => setTitle(e.target.value)} required />
            <div className="grid grid-cols-2 gap-3">
              <Select label={t("common.priority")} value={priority} onChange={(e) => setPriority(e.target.value)} options={[{ value: "critical", label: t("issue.critical") }, { value: "high", label: t("issue.high") }, { value: "medium", label: t("issue.medium") }, { value: "low", label: t("issue.low") }]} />
              <Select label={t("common.status")} value={status} onChange={(e) => setStatus(e.target.value)} options={STATUSES.map((s) => ({ value: s, label: STATUS_LABELS[s] }))} />
            </div>
          </DrawerSection>
          <DrawerSection title={t("common.description")} open={openSection === "description"} onToggle={() => toggleSection("description")} active={!!description}>
            <MarkdownEditor value={description} onChange={setDescription} rows={4} />
          </DrawerSection>
          <DrawerSection title={t("common.assignees")} open={openSection === "assignees"} onToggle={() => toggleSection("assignees")} active={assigneeIds.length > 0}>
            <CheckboxList label="" items={users.map((u) => ({ id: u.id, name: u.display_name || u.username }))} selected={assigneeIds} onChange={setAssigneeIds} />
          </DrawerSection>
          <DrawerSection title={t("issue.dates")} open={openSection === "dates"} onToggle={() => toggleSection("dates")} active={!!expectedEndDate}>
            <Input label={t("issue.expectedEndDate")} type="date" value={expectedEndDate} onChange={(e) => setExpectedEndDate(e.target.value)} />
            {issue.start_date && <Field label={t("issue.startDate")} value={issue.start_date} />}
            {issue.end_date && <Field label={t("issue.endDate")} value={issue.end_date} />}
          </DrawerSection>
          <DrawerSection title={t("common.links")} open={openSection === "links"} onToggle={() => toggleSection("links")} active={linkedHostIds.length > 0 || linkedDnsIds.length > 0 || linkedServiceIds.length > 0 || linkedProjectIds.length > 0 || linkedAlertIds.length > 0}>
            <CheckboxList label={t("host.title")} items={allHosts.map((h) => ({ id: h.id, name: h.nickname }))} selected={linkedHostIds} onChange={setLinkedHostIds} />
            <CheckboxList label={t("dns.title")} items={allDns.map((d) => ({ id: d.id, name: d.domain }))} selected={linkedDnsIds} onChange={setLinkedDnsIds} />
            <CheckboxList label={t("service.title")} items={allServices.map((s) => ({ id: s.id, name: s.nickname }))} selected={linkedServiceIds} onChange={setLinkedServiceIds} />
            <CheckboxList label={t("project.title")} items={allProjects.map((p) => ({ id: p.id, name: p.name }))} selected={linkedProjectIds} onChange={setLinkedProjectIds} />
            {alertItems.length > 0 && (
              <CheckboxList label={t("alert.title")} items={alertItems} selected={linkedAlertIds} onChange={setLinkedAlertIds} />
            )}
          </DrawerSection>
        </div>
      </Drawer>
    );
  }

  /* ─── CREATE MODE (stepped wizard) ─── */
  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("host.addIssue")}
      subHeader={<StepIndicator steps={stepLabels} current={step} />}
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={step === 1 ? onClose : () => setStep(1)}>
            {step === 1 ? t("common.cancel") : t("common.back")}
          </Button>
          {step < 2 ? (
            <Button size="sm" className="flex-1" disabled={!canProceedStep1} onClick={() => setStep(2)}>
              {t("host.nextStep")}
            </Button>
          ) : (
            <Button size="sm" className="flex-1" loading={loading} onClick={handleSubmit}>
              {t("common.create")}
            </Button>
          )}
        </div>
      }
    >
      {step === 1 ? (
        <div className="space-y-4 animate-fade-in">
          <Input label={t("common.title")} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("issue.titlePlaceholder")} required />
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("common.priority")} value={priority} onChange={(e) => setPriority(e.target.value)} options={[{ value: "critical", label: t("issue.critical") }, { value: "high", label: t("issue.high") }, { value: "medium", label: t("issue.medium") }, { value: "low", label: t("issue.low") }]} />
            <Select label={t("common.status")} value={status} onChange={(e) => setStatus(e.target.value)} options={STATUSES.map((s) => ({ value: s, label: STATUS_LABELS[s] }))} />
          </div>
          <Input label={t("issue.expectedEndDate")} type="date" value={expectedEndDate} onChange={(e) => setExpectedEndDate(e.target.value)} />
          <MarkdownEditor label={t("common.description")} value={description} onChange={setDescription} placeholder={t("issue.descriptionPlaceholder")} rows={4} />
        </div>
      ) : (
        <div className="space-y-0 animate-fade-in">
          <DrawerSection title={t("common.assignees")} open={openSection === "assignees"} onToggle={() => toggleSection("assignees")} active={assigneeIds.length > 0}>
            <CheckboxList label="" items={users.map((u) => ({ id: u.id, name: u.display_name || u.username }))} selected={assigneeIds} onChange={setAssigneeIds} />
          </DrawerSection>
          <DrawerSection title={t("host.title")} open={openSection === "hosts"} onToggle={() => toggleSection("hosts")} active={linkedHostIds.length > 0}>
            <CheckboxList label="" items={allHosts.map((h) => ({ id: h.id, name: h.nickname }))} selected={linkedHostIds} onChange={setLinkedHostIds} />
          </DrawerSection>
          <DrawerSection title={t("dns.title")} open={openSection === "dns"} onToggle={() => toggleSection("dns")} active={linkedDnsIds.length > 0}>
            <CheckboxList label="" items={allDns.map((d) => ({ id: d.id, name: d.domain }))} selected={linkedDnsIds} onChange={setLinkedDnsIds} />
          </DrawerSection>
          <DrawerSection title={t("service.title")} open={openSection === "services"} onToggle={() => toggleSection("services")} active={linkedServiceIds.length > 0}>
            <CheckboxList label="" items={allServices.map((s) => ({ id: s.id, name: s.nickname }))} selected={linkedServiceIds} onChange={setLinkedServiceIds} />
          </DrawerSection>
          <DrawerSection title={t("project.title")} open={openSection === "projects"} onToggle={() => toggleSection("projects")} active={linkedProjectIds.length > 0}>
            <CheckboxList label="" items={allProjects.map((p) => ({ id: p.id, name: p.name }))} selected={linkedProjectIds} onChange={setLinkedProjectIds} />
          </DrawerSection>
          {alertItems.length > 0 && (
            <DrawerSection title={t("alert.title")} open={openSection === "alert"} onToggle={() => toggleSection("alert")} active={linkedAlertIds.length > 0}>
              <CheckboxList label="" items={alertItems} selected={linkedAlertIds} onChange={setLinkedAlertIds} />
            </DrawerSection>
          )}
        </div>
      )}
    </Drawer>
  );
}
