"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { glpiAPI, type GlpiTokenProfile } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import MarkdownEditor from "@/components/ui/MarkdownEditor";

interface Props {
  open: boolean;
  onClose: () => void;
  // Context-aware defaults; the modal composes them into the create request.
  defaultProfileID?: number | null;
  defaultTitle?: string;
  defaultDescription?: string;
  defaultEntityID?: number;
  hostSlug?: string;
  alertID?: number;
  onCreated?: (ticketID: number, ticketURL: string) => void;
}

// Shared "open GLPI ticket" modal. Used by project, host, and alert contexts —
// each passes whatever defaults it has. Relies on the admin having at least one
// token profile configured (queried on mount) — otherwise we show a setup hint.
export default function CreateTicketModal({
  open,
  onClose,
  defaultProfileID,
  defaultTitle,
  defaultDescription,
  defaultEntityID,
  hostSlug,
  alertID,
  onCreated,
}: Props) {
  const [profileID, setProfileID] = useState<number | null>(defaultProfileID ?? null);
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [description, setDescription] = useState(defaultDescription ?? "");
  const [entityID, setEntityID] = useState<number>(defaultEntityID ?? 0);
  const [linkComputer, setLinkComputer] = useState<boolean>(!!hostSlug);
  const [error, setError] = useState<string | null>(null);

  // Reset state when the modal is re-opened with fresh defaults.
  useEffect(() => {
    if (open) {
      setProfileID(defaultProfileID ?? null);
      setTitle(defaultTitle ?? "");
      setDescription(defaultDescription ?? "");
      setEntityID(defaultEntityID ?? 0);
      setLinkComputer(!!hostSlug);
      setError(null);
    }
  }, [open, defaultProfileID, defaultTitle, defaultDescription, defaultEntityID, hostSlug]);

  const { data: profiles } = useQuery<GlpiTokenProfile[]>({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: open,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: () => glpiAPI.createTicket({
      profile_id: profileID!,
      title: title.trim(),
      description: description.trim() || undefined,
      entity_id: entityID || undefined,
      host_slug: hostSlug,
      alert_id: alertID,
      link_computer: linkComputer,
    }),
    onSuccess: (res) => {
      if (res.warning) setError(res.warning); else setError(null);
      onCreated?.(res.ticket_id, res.ticket_url);
      if (!res.warning) {
        onClose();
      }
    },
    onError: (err: Error) => setError(err.message),
  });

  const hasProfiles = (profiles?.length ?? 0) > 0;

  return (
    <ResponsiveModal open={open} onClose={onClose} title="Abrir chamado no GLPI">
      <div className="space-y-4">
        {!hasProfiles && (
          <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-xs px-3 py-2">
            No GLPI profiles configured. Ask an admin to add one in Settings → Integrations → GLPI.
          </div>
        )}
        <div>
          <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">Profile</label>
          <select
            value={profileID ?? ""}
            onChange={(e) => setProfileID(e.target.value ? parseInt(e.target.value, 10) : null)}
            disabled={!hasProfiles}
            className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          >
            <option value="">Select a profile…</option>
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>{p.name}{p.description ? ` — ${p.description}` : ""}</option>
            ))}
          </select>
        </div>
        <Input
          label="Título"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Short summary"
        />
        <div>
          <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">Descrição</label>
          <MarkdownEditor
            value={description}
            onChange={setDescription}
            rows={6}
            placeholder="Descreva o problema ou a solicitação (markdown)..."
          />
        </div>
        <Input
          label="Entity ID (0 = use profile default)"
          type="number"
          value={String(entityID)}
          onChange={(e) => setEntityID(parseInt(e.target.value || "0", 10))}
        />
        {hostSlug && (
          <label className="flex items-center gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
            <input
              type="checkbox"
              checked={linkComputer}
              onChange={(e) => setLinkComputer(e.target.checked)}
              className="w-4 h-4 accent-[var(--accent)]"
            />
            Try to link to the matching GLPI Computer asset ({hostSlug})
          </label>
        )}
        {error && (
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-xs px-3 py-2">
            {error}
          </div>
        )}
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose} disabled={mutation.isPending}>Cancelar</Button>
          <Button
            type="button"
            onClick={() => mutation.mutate()}
            loading={mutation.isPending}
            disabled={!hasProfiles || !profileID || !title.trim()}
          >
            Abrir chamado
          </Button>
        </div>
      </div>
    </ResponsiveModal>
  );
}
