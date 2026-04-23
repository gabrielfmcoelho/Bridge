"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Field from "@/components/ui/Field";
import Badge from "@/components/ui/Badge";
import { glpiAPI, integrationsAPI } from "@/lib/api";
import type { HostChamado } from "@/lib/types";

function applyDateMask(raw: string): string {
  const digits = raw.replace(/\D/g, "").slice(0, 8);
  if (digits.length <= 2) return digits;
  if (digits.length <= 4) return `${digits.slice(0, 2)}/${digits.slice(2)}`;
  return `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
}

function isValidDate(value: string): boolean {
  if (!/^\d{2}\/\d{2}\/\d{4}$/.test(value)) return false;
  const [dd, mm, yyyy] = value.split("/").map(Number);
  const d = new Date(yyyy, mm - 1, dd);
  return d.getFullYear() === yyyy && d.getMonth() === mm - 1 && d.getDate() === dd;
}

const STATUS_LABELS: Record<string, string> = {
  in_execution: "Em Execução",
  solved: "Resolvido",
};

const STATUS_COLORS: Record<string, string> = {
  in_execution: "bg-amber-500/15 text-amber-400 border-amber-500/30",
  solved: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
};

interface ChamadoDrawerProps {
  open: boolean;
  onClose: () => void;
  chamado: HostChamado | null;
  users: { id: number; display_name: string }[];
  onCreate: (data: { chamado_id: string; title: string; status: string; user_id: number; date: string }) => void;
  onUpdate: (id: number, data: { chamado_id: string; title: string; status: string; user_id: number; date: string }) => void;
  onDelete: (id: number) => void;
  loading: boolean;
  t: (k: string) => string;
  slug?: string; // needed for the GLPI refresh button
}

export default function ChamadoDrawer({ open, onClose, chamado, users, onCreate, onUpdate, onDelete, loading, t, slug }: ChamadoDrawerProps) {
  const isEdit = !!chamado;
  const [mode, setMode] = useState<"read" | "edit" | "create">("create");
  const [chamadoId, setChamadoId] = useState("");
  const [title, setTitle] = useState("");
  const [status, setStatus] = useState("in_execution");
  const [userId, setUserId] = useState(0);
  const [date, setDate] = useState("");

  useEffect(() => {
    if (open) {
      if (chamado) {
        setChamadoId(chamado.chamado_id);
        setTitle(chamado.title || "");
        setStatus(chamado.status || "in_execution");
        setUserId(chamado.user_id);
        setDate(chamado.date || "");
        setMode("read");
      } else {
        setChamadoId("");
        setTitle("");
        setStatus("in_execution");
        setUserId(users.length > 0 ? users[0].id : 0);
        setDate("");
        setMode("create");
      }
    }
  }, [open, chamado, users]);

  const userOptions = users.map((u) => ({ value: String(u.id), label: u.display_name }));
  const userName = users.find((u) => u.id === userId)?.display_name || chamado?.user_display_name || "--";

  const handleSubmit = () => {
    const data = { chamado_id: chamadoId, title, status, user_id: userId, date };
    if (isEdit && chamado?.id) {
      onUpdate(chamado.id, data);
    } else {
      onCreate(data);
    }
  };

  /* ─── READ MODE ─── */
  if (mode === "read" && chamado) {
    return (
      <Drawer
        open={open}
        onClose={onClose}
        title={t("host.chamados")}
        footer={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>{t("common.close")}</Button>
            <Button size="sm" className="flex-1" onClick={() => setMode("edit")}>{t("common.edit")}</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <div>
            <p className="text-base font-semibold text-[var(--text-primary)]">{chamado.title || chamado.chamado_id || "--"}</p>
            <p className="text-xs text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>{chamado.chamado_id || "--"}</p>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <span className="text-[var(--text-muted)] text-xs font-medium block mb-1">{t("common.status")}</span>
              <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${STATUS_COLORS[chamado.status] || STATUS_COLORS.in_execution}`}>
                {chamado.status === "in_execution" ? t("chamado.inExecution") : chamado.status === "solved" ? t("chamado.solved") : chamado.status}
              </span>
            </div>
            <Field label={t("host.chamadoUser") || "User"} value={chamado.user_display_name || "--"} />
          </div>

          <Field label={t("host.chamadoDate") || "Date"} value={chamado.date || "--"} />

          {slug && <GlpiRefreshBlock slug={slug} chamado={chamado} />}
        </div>
      </Drawer>
    );
  }

  /* ─── EDIT MODE ─── */
  if (mode === "edit" && chamado) {
    return (
      <Drawer
        open={open}
        onClose={onClose}
        title={t("common.edit") + " " + t("host.chamados")}
        footer={
          <div className="flex gap-2">
            <Button variant="danger" size="sm" onClick={() => { if (confirm(t("chamado.deleteConfirm")) && chamado.id) onDelete(chamado.id); }} className="mr-auto">
              {t("common.delete")}
            </Button>
            <Button variant="secondary" size="sm" className="flex-1" onClick={() => setMode("read")}>{t("common.cancel")}</Button>
            <Button size="sm" className="flex-1" disabled={!chamadoId.trim()} loading={loading} onClick={handleSubmit}>{t("common.save")}</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <Input label={t("host.chamadoId") || "Chamado ID"} value={chamadoId} onChange={(e) => setChamadoId(e.target.value)} placeholder="GLPI #..." required />
          <Input label={t("common.title")} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("chamado.titlePlaceholder")} />
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("common.status")} value={status} onChange={(e) => setStatus(e.target.value)} options={[
              { value: "in_execution", label: t("chamado.inExecution") || "In Execution" },
              { value: "solved", label: t("chamado.solved") || "Solved" },
            ]} />
            <Select label={t("host.chamadoUser") || "User"} value={String(userId)} options={userOptions} onChange={(e) => setUserId(Number(e.target.value))} />
          </div>
          <Input
            label={t("host.chamadoDate") || "Date"}
            value={date}
            onChange={(e) => setDate(applyDateMask(e.target.value))}
            placeholder="DD/MM/YYYY"
            maxLength={10}
            error={date.length === 10 && !isValidDate(date) ? "Data inválida" : undefined}
          />
        </div>
      </Drawer>
    );
  }

  /* ─── CREATE MODE ─── */
  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("host.addChamado") || "+ Add Chamado"}
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={onClose}>{t("common.cancel")}</Button>
          <Button size="sm" className="flex-1" disabled={!chamadoId.trim()} loading={loading} onClick={handleSubmit}>{t("common.create")}</Button>
        </div>
      }
    >
      <div className="space-y-4">
        <Input label={t("host.chamadoId") || "Chamado ID"} value={chamadoId} onChange={(e) => setChamadoId(e.target.value)} placeholder="GLPI #..." required />
        <Input label={t("common.title")} value={title} onChange={(e) => setTitle(e.target.value)} placeholder={t("chamado.titlePlaceholder")} />
        <div className="grid grid-cols-2 gap-3">
          <Select label={t("common.status")} value={status} onChange={(e) => setStatus(e.target.value)} options={[
            { value: "in_execution", label: t("chamado.inExecution") || "In Execution" },
            { value: "solved", label: t("chamado.solved") || "Solved" },
          ]} />
          <Select label={t("host.chamadoUser") || "User"} value={String(userId)} options={userOptions} onChange={(e) => setUserId(Number(e.target.value))} />
        </div>
        <Input
          label={t("host.chamadoDate") || "Date"}
          value={date}
          onChange={(e) => setDate(applyDateMask(e.target.value))}
          placeholder="DD/MM/YYYY"
          maxLength={10}
          error={date.length === 10 && !isValidDate(date) ? "Data inválida" : undefined}
        />
      </div>
    </Drawer>
  );
}

// GlpiRefreshBlock renders a small card inside the chamado read view when the
// chamado_id looks numeric and the GLPI integration is enabled. It shows the
// cached title/status from the last refresh plus a button to re-query GLPI.
// A profile picker appears inline when more than one profile is configured.
function GlpiRefreshBlock({ slug, chamado }: { slug: string; chamado: HostChamado }) {
  const queryClient = useQueryClient();
  const [profileID, setProfileID] = useState<number | null>(null);
  const [liveStatus, setLiveStatus] = useState<{ label: string; slug: string } | null>(null);

  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";

  const { data: profiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: glpiEnabled,
    retry: false,
  });

  // Auto-pick the first profile if only one is configured.
  useEffect(() => {
    if (profiles && profiles.length > 0 && profileID == null) {
      setProfileID(profiles[0].id);
    }
  }, [profiles, profileID]);

  const ticketID = parseInt(chamado.chamado_id.trim(), 10);
  const isNumericTicket = !Number.isNaN(ticketID) && ticketID > 0;

  const mutation = useMutation({
    mutationFn: () => glpiAPI.refreshChamadoCache(slug, chamado.id!, profileID!),
    onSuccess: (res) => {
      setLiveStatus({ label: res.status_label, slug: res.status_slug });
      queryClient.invalidateQueries({ queryKey: ["chamados", slug] });
    },
  });

  if (!glpiEnabled || !isNumericTicket || !chamado.id) return null;

  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] p-3 space-y-2">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-[var(--text-primary)]">GLPI</p>
          {chamado.cached_title ? (
            <p className="text-[11px] text-[var(--text-muted)] truncate">{chamado.cached_title}</p>
          ) : (
            <p className="text-[11px] text-[var(--text-faint)] italic">No live data yet — click Refresh.</p>
          )}
          {(chamado.cached_status || liveStatus) && (
            <div className="flex items-center gap-2 mt-1">
              <Badge className="text-[10px] uppercase tracking-wide">
                {liveStatus?.label ?? chamado.cached_status}
              </Badge>
              {chamado.external_url && (
                <Link
                  href={chamado.external_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-[11px] text-[var(--accent)] hover:underline"
                >
                  Open in GLPI ↗
                </Link>
              )}
            </div>
          )}
        </div>
        <Button
          size="sm"
          variant="secondary"
          onClick={() => mutation.mutate()}
          loading={mutation.isPending}
          disabled={!profileID}
        >
          Refresh
        </Button>
      </div>

      {(profiles?.length ?? 0) > 1 && (
        <div>
          <label className="block text-[10px] text-[var(--text-muted)] mb-1">Using profile</label>
          <select
            value={profileID ?? ""}
            onChange={(e) => setProfileID(e.target.value ? parseInt(e.target.value, 10) : null)}
            className="w-full bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-2 py-1 text-xs"
          >
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
        </div>
      )}

      {mutation.isError && (
        <p className="text-[11px] text-red-400">
          {mutation.error instanceof Error ? mutation.error.message : "Refresh failed"}
        </p>
      )}
    </div>
  );
}
