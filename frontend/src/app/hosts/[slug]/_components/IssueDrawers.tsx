"use client";

import { useState, useMemo, useEffect } from "react";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useQuery } from "@tanstack/react-query";
import { integrationsAPI } from "@/lib/api";
import Button from "@/components/ui/Button";
import CreateTicketModal from "@/components/glpi/CreateTicketModal";
import Drawer from "@/components/ui/Drawer";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Field from "@/components/ui/Field";
import MarkdownEditor, { MarkdownContent } from "@/components/ui/MarkdownEditor";
import type { HostAlert } from "@/lib/types";
import { ALERT_DOT_COLOR, ALERT_TEXT_COLOR } from "@/lib/alert-colors";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
  t: (k: string, vars?: Record<string, string>) => string;
}) {
  const confirm = useConfirm();
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
          {t("common.conclude")}
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
        <Button variant="danger" size="sm" onClick={async () => { if (await confirm({ title: t("alert.deleteConfirm"), danger: true, confirmLabel: t("common.delete") })) { onDelete(alert); onClose(); } }} className="mr-auto">
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
            <span className="text-sm text-[var(--text-muted)]">–</span>
          )}
        </div>

        {isResolved && (
          <div className="flex items-center gap-2 text-xs text-[var(--success)] bg-[var(--success)]/10 rounded-[var(--radius-md)] p-2.5 border border-[var(--success)]/20">
            <Icon path={ICON_PATHS.checkCircle} className="w-3.5 h-3.5 shrink-0" />
            {t("common.resolved")}
          </div>
        )}

        {hasLinkedIssue && (
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-2.5 border border-[var(--border-subtle)]">
            <Icon path={ICON_PATHS.clipboard} className="w-3.5 h-3.5 text-[var(--accent)] shrink-0" />
            {isResolved
              ? t("issue.alertLinkResolved", { id: String(alert.linked_issue_id) })
              : t("issue.alertLinkPending", { id: String(alert.linked_issue_id) })
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
