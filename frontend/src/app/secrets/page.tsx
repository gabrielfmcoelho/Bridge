"use client";

import { useMemo, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import type { Secret } from "@/lib/types";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import ShareLinkModal from "./_components/ShareLinkModal";
import HistoryDrawer from "./_components/HistoryDrawer";
import NewSecretModal from "./_components/NewSecretModal";

// Page contract (Plans.md Task 4.1):
//   - Single API call (/api/secrets) hydrates the list; filtering is
//     client-side over those rows.
//   - Three filter rows: scope chips / visibility chips / type filter.
//   - Default view: visibility=all, scope=all, filtered to permitted rows.
//   - The "personal" chip pre-selects when the URL carries
//     ?visibility=personal (e.g. via the /mine deep link) or
//     ?scope=service (the /service-credentials redirect target).
//   - Reveal uses the useSecretReveal hook (Task 4.4) — 30s auto-hide.
//
// The page itself reads ?scope/?visibility from URL so deep links work.

type ScopeFilter = "all" | "service" | "host" | "tool" | "avulso";
type VisibilityFilter = "all" | "personal" | "shared";
type TypeFilter = "all" | "cred" | "sshkey" | "password" | "app_login" | "env_var";

const SCOPES: ScopeFilter[] = ["all", "service", "host", "tool", "avulso"];
const VISIBILITIES: VisibilityFilter[] = ["all", "personal", "shared"];
const TYPES: TypeFilter[] = ["all", "cred", "sshkey", "password", "app_login", "env_var"];

export default function SecretsPage() {
  return (
    // useSearchParams() requires a Suspense boundary in App Router so the
    // page can prerender even when the search-params bailout fires.
    <Suspense fallback={<PageShell><div className="text-sm text-[var(--text-muted)]">Loading...</div></PageShell>}>
      <SecretsPageInner />
    </Suspense>
  );
}

function SecretsPageInner() {
  const params = useSearchParams();
  const initialScope = (params.get("scope") || "all") as ScopeFilter;
  const initialVisibility = (params.get("visibility") || "all") as VisibilityFilter;
  const initialType = (params.get("type") || "all") as TypeFilter;

  const [scope, setScope] = useState<ScopeFilter>(initialScope);
  const [visibility, setVisibility] = useState<VisibilityFilter>(initialVisibility);
  const [typeF, setTypeF] = useState<TypeFilter>(initialType);
  const [shareTarget, setShareTarget] = useState<number | null>(null);
  const [historyTarget, setHistoryTarget] = useState<number | null>(null);
  const [newOpen, setNewOpen] = useState(false);

  const { data: rawSecrets = [], isLoading, refetch } = useQuery({
    queryKey: ["secrets-all"],
    queryFn: () => secretsAPI.list({}),
  });

  // Client-side filtering — the spec wants a single API call and chip-
  // based narrowing. The repo's per-row ACL has already trimmed what we
  // can see; chips just narrow that further.
  const visible = useMemo(() => {
    return rawSecrets.filter((s) => {
      if (scope !== "all" && s.scope !== scope) return false;
      if (visibility !== "all" && s.visibility !== visibility) return false;
      if (typeF !== "all" && s.type !== typeF) return false;
      return true;
    });
  }, [rawSecrets, scope, visibility, typeF]);

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
        <EmptyState
          icon="key"
          title="No secrets match these filters"
          description="Try widening the chips above, or create a new secret."
        />
      ) : (
        <div className="space-y-2">
          {visible.map((s) => (
            <SecretRow
              key={s.id}
              secret={s}
              onShare={() => setShareTarget(s.id)}
              onHistory={() => setHistoryTarget(s.id)}
              onDeleted={refetch}
            />
          ))}
        </div>
      )}

      <ShareLinkModal secretID={shareTarget} onClose={() => setShareTarget(null)} />
      <HistoryDrawer secretID={historyTarget} onClose={() => setHistoryTarget(null)} />
      <NewSecretModal open={newOpen} onClose={() => setNewOpen(false)} />
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

function Chip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
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
  onShare,
  onHistory,
  onDeleted,
}: {
  secret: Secret;
  onShare: () => void;
  onHistory: () => void;
  onDeleted: () => void;
}) {
  const { user } = useAuth();
  const reveal = useSecretReveal();
  const qc = useQueryClient();

  const isOwner = user?.id === secret.owner_user_id;
  const canShare = secret.visibility === "personal" && isOwner;

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
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-medium text-sm text-[var(--text-primary)]">{secret.name}</span>
            <Badge>{secret.type}</Badge>
            <Badge color={secret.visibility === "personal" ? "purple" : "amber"}>
              {secret.visibility}
            </Badge>
            <Badge>{secret.scope}</Badge>
            {secret.group_label && (
              <span className="text-[10px] text-[var(--text-faint)]">
                env: {secret.group_label}
              </span>
            )}
          </div>
          {secret.description && (
            <p className="text-xs text-[var(--text-muted)] mt-1">{secret.description}</p>
          )}
          {reveal.revealed && (
            <pre className="mt-2 text-xs text-[var(--text-secondary)] bg-[var(--bg-surface)] rounded-[var(--radius-sm)] p-2 overflow-x-auto whitespace-pre-wrap break-all border border-[var(--border-subtle)]">
              {reveal.value}
            </pre>
          )}
          {reveal.error && (
            <p className="text-xs text-red-400 mt-1">{reveal.error}</p>
          )}
        </div>
        <div className="flex items-center gap-2 flex-wrap justify-end">
          <Button size="sm" variant="secondary" onClick={handleToggleReveal} disabled={reveal.loading}>
            {reveal.loading
              ? "..."
              : reveal.revealed
              ? `Hide (${Math.ceil(reveal.remainingMs / 1000)}s)`
              : "Reveal"}
          </Button>
          {reveal.revealed && (
            <Button size="sm" variant="secondary" onClick={() => reveal.copy()}>
              {reveal.copyState === "copied"
                ? "Copied"
                : reveal.copyState === "cleared"
                ? "Cleared"
                : "Copy"}
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
