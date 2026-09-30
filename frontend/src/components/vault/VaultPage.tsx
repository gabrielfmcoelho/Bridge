"use client";

import { Suspense, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useDebounce } from "@/hooks/useDebounce";
import type { Secret } from "@/lib/types";
import { groupVault } from "@/lib/vaultGroups";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import ListToolbar from "@/components/ui/ListToolbar";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import PillButton from "@/components/ui/PillButton";
import Drawer from "@/components/ui/Drawer";
import SectionHeading from "@/components/ui/SectionHeading";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import type { RowAction } from "@/components/ui/RowActions";
import { ICON_PATHS } from "@/lib/icon-paths";
import { NewSecretDrawer } from "./SecretForm";
import VaultEntryEditor from "./VaultEntryEditor";
import SecretDetailDrawer from "./SecretDetailDrawer";
import VaultTable from "./VaultTable";
import VaultCards from "./VaultCards";
import ShareLinkDrawer from "./ShareLinkDrawer";
import ConsolidateDrawer, { consolidationKey } from "./ConsolidateDrawer";

// The Cofre lives here (outside app/secrets/, which is write-protected) and
// app/secrets/page.tsx re-exports it. One page for every secret, host
// credentials included (the old /ssh-keys library): a grouped table — env
// vars by bundle, the same credential repeated across places as one line —
// cards on phones, filters in the URL, a detail drawer per secret.

const KINDS = ["", "host_cred", "password", "sshkey", "cred", "app_login", "env_var"] as const;
const SCOPES = ["", "avulso", "projeto", "service", "host", "tool"] as const;
const VISIBILITIES = ["", "shared", "personal"] as const;
type Filters = { kind: string; scope: string; visibility: string };

export default function VaultPage() {
  return (
    <Suspense fallback={<PageShell><SkeletonCard /></PageShell>}>
      <VaultPageInner />
    </Suspense>
  );
}

function VaultPageInner() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();
  const isAdmin = user?.role === "admin";
  const isEditor = isAdmin || user?.role === "editor";

  // Filters live in the URL, so /secrets?kind=host_cred is a link.
  const pick = (k: string, allowed: readonly string[]) => (allowed.includes(params.get(k) ?? "") ? params.get(k)! : "");
  const filters: Filters = {
    kind: pick("kind", KINDS) || pick("type", KINDS), scope: pick("scope", SCOPES), visibility: pick("visibility", VISIBILITIES),
  };
  const setFilters = (f: Partial<Filters>) => {
    const next = { ...filters, ...f };
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(next)) if (v) qs.set(k, v);
    router.replace(`/secrets${qs.size ? `?${qs}` : ""}`, { scroll: false });
  };
  const activeFilterCount = [filters.kind, filters.scope, filters.visibility].filter(Boolean).length;

  const [search, setSearch] = useState("");
  const q = useDebounce(search.trim(), 250);
  const [viewMode, setViewMode] = useLocalStorage<"table" | "cards">("vault.view", "table");
  const [showFilters, setShowFilters] = useState(false);
  const [newOpen, setNewOpen] = useState(false);
  const [consolidateOpen, setConsolidateOpen] = useState(false);
  const [detail, setDetail] = useState<Secret | null>(null);
  const [editing, setEditing] = useState<Secret | null>(null);
  const [sharing, setSharing] = useState<number | null>(null);

  const { data: secrets = [], isLoading } = useQuery({
    queryKey: ["secrets-all", "vault", filters.kind, filters.scope, filters.visibility, q],
    queryFn: () => secretsAPI.list({
      kind: filters.kind === "host_cred" ? "host_cred" : undefined,
      type: filters.kind && filters.kind !== "host_cred" ? filters.kind : undefined,
      scope: filters.scope || undefined,
      visibility: filters.visibility || undefined,
      q: q || undefined,
    }),
  });
  const { data: plan = [] } = useQuery({ queryKey: consolidationKey, queryFn: secretsAPI.consolidationPlan, enabled: isAdmin });
  const rows = useMemo(() => groupVault(secrets), [secrets]);

  const canWrite = (s: Secret) => (s.visibility === "personal" ? s.owner_user_id === user?.id : isEditor);
  const canDelete = (s: Secret) => (s.visibility === "personal" ? s.owner_user_id === user?.id : isAdmin);
  const del = useMutation({
    mutationFn: (id: number) => secretsAPI.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["secrets-all"] }),
    onError: (e) => flag({ appearance: "error", title: t("common.delete"), description: e instanceof Error ? e.message : t("form.saveFailed") }),
  });
  const actionsFor = (s: Secret): RowAction[] => [
    { label: t("vault.open"), icon: ICON_PATHS.eye, onClick: () => setDetail(s) },
    { label: t("common.edit"), icon: ICON_PATHS.editPencil, onClick: () => setEditing(s), hidden: !canWrite(s) },
    { label: t("common.share"), icon: ICON_PATHS.share, onClick: () => setSharing(s.id) },
    {
      label: t("common.delete"), icon: ICON_PATHS.trashOutline, danger: true, hidden: !canDelete(s),
      onClick: async () => {
        if (await confirm({ title: t("confirm.deleteTitle", { name: `"${s.name}"` }), message: t("vault.deleteHint"), danger: true, confirmLabel: t("common.delete") })) del.mutate(s.id);
      },
    },
  ];

  const filtered = !!(q || activeFilterCount);
  const pills = (label: string, values: readonly string[], value: string, key: keyof Filters, labelOf: (v: string) => string) => (
    <section className="space-y-2">
      <SectionHeading as="h3" className="!mb-0">{label}</SectionHeading>
      <div className="flex flex-wrap gap-1.5">
        {values.map((v) => (
          <PillButton key={v || "all"} size="sm" active={value === v} onClick={() => setFilters({ [key]: v })}>{v ? labelOf(v) : t("common.all")}</PillButton>
        ))}
      </div>
    </section>
  );

  return (
    <PageShell>
      <PageHeader
        title={t("nav.vault")}
        description={t("vault.pageDescription")}
        addLabel={t("vault.newSecretButton")}
        onAdd={() => setNewOpen(true)}
        controlsKey="vault"
        controlsBadge={activeFilterCount}
        controls={
          <ListToolbar
            search={search}
            onSearchChange={setSearch}
            onFilterClick={() => setShowFilters(true)}
            activeFilterCount={activeFilterCount}
            searchPlaceholder={t("vault.searchPlaceholder")}
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            actions={<>
              <ToolbarActionButton icon={ICON_PATHS.server} label={t("vault.hostCredentials")} hideLabel="md"
                active={filters.kind === "host_cred"} onClick={() => setFilters({ kind: filters.kind === "host_cred" ? "" : "host_cred" })} />
              {isAdmin && plan.length > 0 && (
                <ToolbarActionButton icon={ICON_PATHS.link} label={t("vault.consolidateN", { count: String(plan.length) })} hideLabel="md"
                  onClick={() => setConsolidateOpen(true)} />
              )}
              <ToolbarActionButton icon={ICON_PATHS.trashOutline} label={t("vault.viewTrash")} hideLabel="md" onClick={() => router.push("/secrets/trash")} />
            </>}
          />
        }
      />

      {isLoading ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <SkeletonCard key={i} />)}</div>
      ) : rows.length === 0 ? (
        filtered ? (
          <EmptyState icon="search" title={t("common.noResults")} description={t("vault.emptyFiltered")}
            action={<Button size="sm" variant="secondary" onClick={() => { setSearch(""); setFilters({ kind: "", scope: "", visibility: "" }); }}>{t("vault.clearFilters")}</Button>} />
        ) : (
          <EmptyState icon="key" title={t("vault.emptyTitle")} description={t("vault.emptyDescription")}
            action={<Button size="sm" onClick={() => setNewOpen(true)}>{t("vault.newSecretButton")}</Button>} />
        )
      ) : (
        <>
          <div className={viewMode === "table" ? "hidden md:block" : "hidden"}>
            <VaultTable rows={rows} onOpen={setDetail} actionsFor={actionsFor} t={t} locale={locale} />
          </div>
          <div className={viewMode === "table" ? "md:hidden" : ""}>
            <VaultCards rows={rows} onOpen={setDetail} actionsFor={actionsFor} t={t} />
          </div>
        </>
      )}

      <Drawer open={showFilters} onClose={() => setShowFilters(false)} title={t("common.filter")}
        footer={activeFilterCount > 0 ? <Button variant="secondary" className="w-full" onClick={() => setFilters({ kind: "", scope: "", visibility: "" })}>{t("vault.clearFilters")}</Button> : undefined}>
        <div className="space-y-6">
          {pills(t("secretForm.type"), KINDS, filters.kind, "kind", (v) => (v === "host_cred" ? t("vault.hostCredentials") : t(`vault.type.${v}`)))}
          {pills(t("secretForm.scope"), SCOPES, filters.scope, "scope", (v) => t(`vault.scope.${v}`))}
          {pills(t("secretForm.visibility"), VISIBILITIES, filters.visibility, "visibility", (v) => t(`vault.visibility.${v}`))}
        </div>
      </Drawer>

      <SecretDetailDrawer secret={detail} onClose={() => setDetail(null)}
        onEdit={(s) => { setDetail(null); setEditing(s); }} onShare={(s) => setSharing(s.id)}
        canWrite={!!detail && canWrite(detail)} canDelete={!!detail && canDelete(detail)} />
      <NewSecretDrawer open={newOpen} onClose={() => setNewOpen(false)} />
      <VaultEntryEditor secret={editing} onClose={() => setEditing(null)} />
      <ShareLinkDrawer secretID={sharing} onClose={() => setSharing(null)} />
      {isAdmin && <ConsolidateDrawer open={consolidateOpen} onClose={() => setConsolidateOpen(false)} />}
    </PageShell>
  );
}
