"use client";

import { useMemo, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { secretsAPI, projectsAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import type { Secret } from "@/lib/types";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import EmptyState from "@/components/ui/EmptyState";
import ShareLinkModal from "@/app/secrets/_components/ShareLinkModal";
import HistoryDrawer from "@/app/secrets/_components/HistoryDrawer";
import NewSecretModal from "@/app/secrets/_components/NewSecretModal";
import LinkedHostsModal from "@/app/secrets/_components/LinkedHostsModal";
import VaultEntryEditor from "@/components/vault/VaultEntryEditor";

// The Vault page lives here (outside app/secrets/, which is write-protected)
// and is re-exported by app/secrets/page.tsx. Adds over the original: a
// search bar, grouping by project, per-row project context, an Edit action,
// and Share enabled for shared secrets (anyone who can reveal them).

type ScopeFilter = "all" | "service" | "host" | "tool" | "projeto" | "avulso";
type VisibilityFilter = "all" | "personal" | "shared";
type TypeFilter = "all" | "cred" | "sshkey" | "password" | "app_login" | "env_var";

const SCOPES: ScopeFilter[] = ["all", "service", "host", "tool", "projeto", "avulso"];
const VISIBILITIES: VisibilityFilter[] = ["all", "personal", "shared"];
const TYPES: TypeFilter[] = ["all", "cred", "sshkey", "password", "app_login", "env_var"];

export default function VaultPage() {
  return (
    <Suspense fallback={<PageShell><div className="text-sm text-[var(--text-muted)]">Loading...</div></PageShell>}>
      <VaultPageInner />
    </Suspense>
  );
}

function VaultPageInner() {
  const params = useSearchParams();
  const [scope, setScope] = useState<ScopeFilter>((params.get("scope") || "all") as ScopeFilter);
  const [visibility, setVisibility] = useState<VisibilityFilter>((params.get("visibility") || "all") as VisibilityFilter);
  const [typeF, setTypeF] = useState<TypeFilter>((params.get("type") || "all") as TypeFilter);
  const [search, setSearch] = useState("");
  const [shareTarget, setShareTarget] = useState<number | null>(null);
  const [historyTarget, setHistoryTarget] = useState<number | null>(null);
  const [editTarget, setEditTarget] = useState<Secret | null>(null);
  const [manageHostsTarget, setManageHostsTarget] = useState<number | null>(null);
  const [newOpen, setNewOpen] = useState(false);

  const { data: rawSecrets = [], isLoading, refetch } = useQuery({
    queryKey: ["secrets-all"],
    queryFn: () => secretsAPI.list({}),
  });
  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: () => projectsAPI.list() });

  const projectName = useMemo(() => {
    const m = new Map<number, string>();
    for (const p of projects) m.set(p.id, p.name);
    return m;
  }, [projects]);

  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return rawSecrets.filter((s) => {
      if (scope !== "all" && s.scope !== scope) return false;
      if (visibility !== "all" && s.visibility !== visibility) return false;
      if (typeF !== "all" && s.type !== typeF) return false;
      if (q) {
        const proj = s.scope === "projeto" && s.parent_id != null ? projectName.get(s.parent_id) ?? "" : "";
        const hay = `${s.name} ${s.description ?? ""} ${s.group_label ?? ""} ${s.type} ${proj}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  }, [rawSecrets, scope, visibility, typeF, search, projectName]);

  // Group by project: projeto-scoped secrets group under their project name;
  // everything else falls into a trailing "Other secrets" group.
  const groups = useMemo(() => {
    const byKey = new Map<string, { title: string; order: number; items: Secret[] }>();
    for (const s of visible) {
      let key: string, title: string, order: number;
      if (s.scope === "projeto" && s.parent_id != null) {
        key = `p:${s.parent_id}`;
        title = projectName.get(s.parent_id) ?? `Project #${s.parent_id}`;
        order = 0;
      } else {
        key = "other";
        title = "Other secrets";
        order = 1;
      }
      if (!byKey.has(key)) byKey.set(key, { title, order, items: [] });
      byKey.get(key)!.items.push(s);
    }
    return Array.from(byKey.values()).sort((a, b) => a.order - b.order || a.title.localeCompare(b.title));
  }, [visible, projectName]);

  return (
    <PageShell>
      <PageHeader title="Vault" />
      <div className="flex items-center justify-between mb-4 gap-3 flex-wrap">
        <p className="text-sm text-[var(--text-muted)] flex-1 min-w-[200px]">
          All secrets you can access. Personal secrets are owner-only; shared secrets follow role-based ACL.
        </p>
        <div className="flex items-center gap-2">
          <Link href="/secrets/trash" className="text-xs text-[var(--accent)] hover:underline">
            View trash →
          </Link>
          <Button size="sm" onClick={() => setNewOpen(true)}>
            + New secret
          </Button>
        </div>
      </div>

      <Card className="mb-4">
        <div className="px-1 pb-2">
          <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search secrets by name, description, project…" />
        </div>
        <FilterRow label="Scope">
          {SCOPES.map((s) => (
            <Chip key={s} active={scope === s} onClick={() => setScope(s)}>{s}</Chip>
          ))}
        </FilterRow>
        <FilterRow label="Visibility">
          {VISIBILITIES.map((v) => (
            <Chip key={v} active={visibility === v} onClick={() => setVisibility(v)}>{v}</Chip>
          ))}
        </FilterRow>
        <FilterRow label="Type">
          {TYPES.map((t) => (
            <Chip key={t} active={typeF === t} onClick={() => setTypeF(t)}>{t.replace("_", " ")}</Chip>
          ))}
        </FilterRow>
      </Card>

      {isLoading ? (
        <p className="text-sm text-[var(--text-muted)]">Loading...</p>
      ) : visible.length === 0 ? (
        <EmptyState icon="key" title="No secrets match these filters" description="Try widening the chips above, adjusting your search, or create a new secret." />
      ) : (
        <div className="space-y-6">
          {groups.map((g) => (
            <div key={g.title}>
              <h2 className="text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] mb-2 px-1">
                {g.title} <span className="text-[var(--text-faint)]">· {g.items.length}</span>
              </h2>
              <div className="space-y-2">
                {g.items.map((s) => (
                  <SecretRow
                    key={s.id}
                    secret={s}
                    projectName={s.scope === "projeto" && s.parent_id != null ? projectName.get(s.parent_id) : undefined}
                    onShare={() => setShareTarget(s.id)}
                    onHistory={() => setHistoryTarget(s.id)}
                    onEdit={() => setEditTarget(s)}
                    onManageHosts={() => setManageHostsTarget(s.id)}
                    onDeleted={refetch}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      <ShareLinkModal secretID={shareTarget} onClose={() => setShareTarget(null)} />
      <HistoryDrawer secretID={historyTarget} onClose={() => setHistoryTarget(null)} />
      <NewSecretModal open={newOpen} onClose={() => setNewOpen(false)} />
      <LinkedHostsModal secretID={manageHostsTarget} onClose={() => setManageHostsTarget(null)} />
      <VaultEntryEditor secret={editTarget} onClose={() => setEditTarget(null)} />
    </PageShell>
  );
}

function FilterRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 py-1.5">
      <span className="text-xs font-medium text-[var(--text-muted)] w-20 shrink-0">{label}</span>
      <div className="flex gap-1.5 flex-wrap">{children}</div>
    </div>
  );
}

function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-3 py-1 text-xs rounded-[var(--radius-md)] border transition-colors ${
        active
          ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/50"
          : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-subtle)] hover:text-[var(--text-secondary)]"
      }`}
    >
      {children}
    </button>
  );
}

function SecretRow({
  secret,
  projectName,
  onShare,
  onHistory,
  onEdit,
  onManageHosts,
  onDeleted,
}: {
  secret: Secret;
  projectName?: string;
  onShare: () => void;
  onHistory: () => void;
  onEdit: () => void;
  onManageHosts: () => void;
  onDeleted: () => void;
}) {
  const { user } = useAuth();
  const reveal = useSecretReveal();
  const qc = useQueryClient();

  const isOwner = user?.id === secret.owner_user_id;
  // Personal: owner only. Shared: anyone who can see it (the list already
  // trims to revealable rows) can now share it externally.
  const canShare = (secret.visibility === "personal" && isOwner) || secret.visibility === "shared";

  const del = useMutation({
    mutationFn: () => secretsAPI.delete(secret.id),
    onSuccess: () => {
      onDeleted();
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
    },
  });

  const handleToggleReveal = () => {
    if (reveal.revealed) reveal.hide();
    else reveal.reveal(secret.id);
  };

  return (
    <Card hover>
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          {projectName && (
            <p className="text-[10px] text-[var(--text-faint)] leading-tight">{projectName}</p>
          )}
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm text-[var(--text-primary)]">{secret.name}</span>
            <Badge>{secret.type}</Badge>
            <Badge color={secret.visibility === "personal" ? "purple" : "amber"}>{secret.visibility}</Badge>
            <Badge>{secret.scope}</Badge>
            {secret.group_label && (
              <span className="text-[10px] text-[var(--text-faint)]">env: {secret.group_label}</span>
            )}
          </div>
          {secret.description && <p className="text-xs text-[var(--text-muted)] mt-1">{secret.description}</p>}
          {reveal.revealed && (
            <pre className="mt-2 text-xs text-[var(--text-secondary)] bg-[var(--bg-surface)] rounded-[var(--radius-sm)] p-2 overflow-x-auto whitespace-pre-wrap break-all border border-[var(--border-subtle)]">
              {reveal.value}
            </pre>
          )}
          {reveal.error && <p className="text-xs text-red-400 mt-1">{reveal.error}</p>}
        </div>
        <div className="flex items-center gap-2 flex-wrap justify-end">
          <Button size="sm" variant="secondary" onClick={handleToggleReveal} disabled={reveal.loading}>
            {reveal.loading ? "..." : reveal.revealed ? `Hide (${Math.ceil(reveal.remainingMs / 1000)}s)` : "Reveal"}
          </Button>
          {reveal.revealed && (
            <Button size="sm" variant="secondary" onClick={() => reveal.copy()}>
              {reveal.copyState === "copied" ? "Copied" : reveal.copyState === "cleared" ? "Cleared" : "Copy"}
            </Button>
          )}
          <Button size="sm" variant="secondary" onClick={onEdit}>
            Edit
          </Button>
          {secret.type === "password" && secret.scope === "avulso" && (
            <Button size="sm" variant="secondary" onClick={onManageHosts}>
              Hosts
            </Button>
          )}
          {canShare && (
            <Button size="sm" variant="secondary" onClick={onShare}>
              Share
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={onHistory}>
            History
          </Button>
          <Button size="sm" variant="danger" onClick={() => del.mutate()} disabled={del.isPending}>
            Delete
          </Button>
        </div>
      </div>
    </Card>
  );
}
