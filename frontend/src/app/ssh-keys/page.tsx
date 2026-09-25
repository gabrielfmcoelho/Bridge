"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { sshKeysAPI, coolifyAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Field from "@/components/ui/Field";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import StepIndicator from "@/components/ui/StepIndicator";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import type { SSHKeyRecord, AssetGrantsInput } from "@/lib/types";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function HostCredentialsPage() {
  const confirm = useConfirm();
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [viewingKey, setViewingKey] = useState<number | null>(null);
  const [search, setSearch] = useState("");
  const canEdit = user?.role === "admin" || user?.role === "editor";

  const { data: allKeys = [], isLoading } = useQuery({
    queryKey: ["ssh-keys"],
    queryFn: sshKeysAPI.list,
  });

  const keys = search
    ? allKeys.filter(k => k.name.toLowerCase().includes(search.toLowerCase()) || k.fingerprint?.toLowerCase().includes(search.toLowerCase()) || k.username?.toLowerCase().includes(search.toLowerCase()))
    : allKeys;

  const deleteMutation = useMutation({
    mutationFn: (id: number) => sshKeysAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["ssh-keys"] }),
  });

  return (
    <PageShell>
      <PageHeader
        title={t("nav.hostCredentials")}
        addLabel={canEdit ? t("common.add") : undefined}
        onAdd={canEdit ? () => setShowForm(true) : undefined}
      />

      {/* Search */}
      <div className="flex items-center gap-2 mb-5">
        <div className="relative flex-1 max-w-sm min-w-[200px]">
          <Icon path={ICON_PATHS.search} className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none" />
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-1.5 text-sm transition duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : keys.length === 0 ? (
        <EmptyState
          icon="key"
          title={t("sshKey.emptyTitle")}
          description={t("sshKey.emptyDescription")}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {keys.map((cred, i) => (
            <div key={cred.id} className="stagger-in" style={{ "--i": i } as React.CSSProperties}>
              <CredentialCard cred={cred} onClick={() => setViewingKey(cred.id)} onDelete={canEdit ? async () => { if (await confirm({ title: t("confirm.deleteTitle", { name: cred.name }), danger: true, confirmLabel: t("common.delete") })) deleteMutation.mutate(cred.id); } : undefined} />
            </div>
          ))}
        </div>
      )}

      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={t("common.add")}>
        <CredentialForm onSuccess={() => {
          setShowForm(false);
          queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
        }} />
      </ResponsiveModal>

      <ResponsiveModal open={viewingKey !== null} onClose={() => setViewingKey(null)} title={t("sshKey.detailsTitle")}>
        {viewingKey !== null && <KeyView id={viewingKey} />}
      </ResponsiveModal>
    </PageShell>
  );
}

function CredentialCard({ cred, onClick, onDelete }: { cred: SSHKeyRecord; onClick: () => void; onDelete?: () => void }) {
  const { t } = useLocale();
  const isKey = cred.credential_type === "key";

  return (
    <div onClick={onClick} className="cursor-pointer h-full">
      <Card accent={isKey ? "cyan" : "accent"} className="h-full flex flex-col">
        <div className="flex items-start justify-between mb-2">
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-[var(--text-primary)] text-sm truncate font-mono">
              {cred.name}
            </h3>
            {cred.username && (
              <p className="text-2xs text-[var(--text-faint)] truncate font-mono">
                {cred.username}
              </p>
            )}
          </div>
          <Badge color={isKey ? "cyan" : "accent"}>
            {isKey ? t("sshKey.sshKeyLabel") : t("sshKey.password")}
          </Badge>
        </div>

        {cred.description && (
          <p className="text-xs text-[var(--text-muted)] line-clamp-1 mb-2">{cred.description}</p>
        )}

        <div className="flex gap-1 mb-2">
          {isKey && (
            <>
              {cred.has_public_key && <Badge color="emerald">{t("sshKey.pubBadge")}</Badge>}
              {cred.has_private_key && <Badge color="cyan">{t("sshKey.privBadge")}</Badge>}
            </>
          )}
          {!isKey && cred.has_password && <Badge color="accent">{t("sshKey.encryptedBadge")}</Badge>}
        </div>

        {/* Consistent bottom section */}
        <div className="mt-auto pt-2 border-t border-[var(--border-subtle)]">
          {cred.fingerprint ? (
            <p className="text-2xs text-[var(--text-faint)] truncate font-mono">
              {cred.fingerprint}
            </p>
          ) : (
            <p className="text-2xs text-[var(--text-faint)]">&nbsp;</p>
          )}
          <div className="flex items-center justify-between mt-1">
            <p className="text-2xs text-[var(--text-faint)]">
              {new Date(cred.created_at).toLocaleDateString()}
            </p>
            {onDelete && (
              <button
                onClick={(e) => { e.stopPropagation(); onDelete(); }}
                className="text-2xs text-[var(--text-faint)] hover:text-[var(--danger)] transition-colors"
              >
                {t("common.delete")}
              </button>
            )}
          </div>
        </div>
      </Card>
    </div>
  );
}

function CredentialForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const [grants, setGrants] = useState<AssetGrantsInput>(defaultGrants(user));
  const [step, setStep] = useState(1);
  const [credType, setCredType] = useState<"key" | "password">("key");
  const [form, setForm] = useState({
    name: "",
    username: "",
    description: "",
    public_key: "",
    private_key: "",
    password: "",
  });
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () => sshKeysAPI.create({
      name: form.name,
      credential_type: credType,
      username: form.username || undefined,
      description: form.description || undefined,
      public_key: credType === "key" ? form.public_key || undefined : undefined,
      private_key: credType === "key" ? form.private_key || undefined : undefined,
      password: credType === "password" ? form.password || undefined : undefined,
      ...grants,
    }),
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : t("sshKey.genericFailed")),
  });

  return (
    <div className="space-y-4">
      <StepIndicator steps={[t("common.type"), credType === "key" ? t("sshKey.stepKeys") : t("sshKey.password")]} current={step} />
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          {/* Credential type selector */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-2">{t("common.type")}</label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setCredType("key")}
                className={`p-3 rounded-[var(--radius-md)] border text-left transition ${
                  credType === "key"
                    ? "border-[var(--accent)]/30 bg-[var(--accent-muted)]"
                    : "border-[var(--border-default)] bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
                }`}
              >
                <div className="flex items-center gap-2 mb-1">
                  <Icon path={ICON_PATHS.key} className={`w-4 h-4 ${credType === "key" ? "text-[var(--accent)]" : "text-[var(--text-faint)]"}`} />
                  <span className={`text-sm font-medium ${credType === "key" ? "text-[var(--accent)]" : "text-[var(--text-secondary)]"}`}>{t("sshKey.sshKeyLabel")}</span>
                </div>
                <p className="text-2xs text-[var(--text-faint)]">{t("sshKey.keyPairHint")}</p>
              </button>
              <button
                type="button"
                onClick={() => setCredType("password")}
                className={`p-3 rounded-[var(--radius-md)] border text-left transition ${
                  credType === "password"
                    ? "border-[var(--accent)]/30 bg-[var(--accent)]/10"
                    : "border-[var(--border-default)] bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
                }`}
              >
                <div className="flex items-center gap-2 mb-1">
                  <Icon path={ICON_PATHS.lock} className={`w-4 h-4 ${credType === "password" ? "text-[var(--accent)]" : "text-[var(--text-faint)]"}`} />
                  <span className={`text-sm font-medium ${credType === "password" ? "text-[var(--accent)]" : "text-[var(--text-secondary)]"}`}>{t("sshKey.password")}</span>
                </div>
                <p className="text-2xs text-[var(--text-faint)]">{t("sshKey.passwordStorageHint")}</p>
              </button>
            </div>
          </div>
          <Input label={t("common.name")} value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} required placeholder={t("sshKey.namePlaceholder")} />
          <Input label={t("sshKey.usernameLabel")} value={form.username} onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} placeholder={t("sshKey.usernamePlaceholder")} />
          <Input label={t("common.description")} value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} placeholder={t("sshKey.descriptionPlaceholder")} />
          <p className="text-2xs text-[var(--text-faint)]">
            {t("sshKey.encryptionNotice")}
          </p>
          <Button type="button" className="w-full" disabled={!form.name.trim()} onClick={() => setStep(2)}>
            {t("host.nextStep")}
          </Button>
        </div>
      )}

      {step === 2 && credType === "key" && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Textarea label={t("sshKey.publicKeyLabel")} value={form.public_key} onChange={(e) => setForm((f) => ({ ...f, public_key: e.target.value }))} rows={3} placeholder="ssh-ed25519 AAAA..." className="font-mono" />
          <Textarea label={t("sshKey.privateKeyLabel")} value={form.private_key} onChange={(e) => setForm((f) => ({ ...f, private_key: e.target.value }))} rows={4} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" className="font-mono" />
          <EntidadeScopeFields value={grants} onChange={setGrants} compact />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{t("common.create")}</Button>
          </div>
        </form>
      )}

      {step === 2 && credType === "password" && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Input label={t("sshKey.password")} type="password" value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} required placeholder={t("sshKey.passwordPlaceholder")} />
          <EntidadeScopeFields value={grants} onChange={setGrants} compact />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{t("common.create")}</Button>
          </div>
        </form>
      )}
    </div>
  );
}

function KeyView({ id, onUpdated }: { id: number; onUpdated?: () => void }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [isEditing, setIsEditing] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", username: "", description: "", public_key: "", private_key: "", password: "" });
  const [editGrants, setEditGrants] = useState<AssetGrantsInput>({});
  const [editError, setEditError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["ssh-key", id],
    queryFn: () => sshKeysAPI.get(id),
  });

  // Coolify key status
  const { data: coolifyStatus } = useQuery({
    queryKey: ["coolify-status"],
    queryFn: coolifyAPI.status,
    staleTime: 60_000,
  });
  const { data: coolifyCheck, isLoading: coolifyChecking } = useQuery({
    queryKey: ["coolify-key-check", id],
    queryFn: () => coolifyAPI.checkKey(id),
    enabled: !!coolifyStatus?.enabled && !!data?.fingerprint,
    staleTime: 30_000,
  });
  const syncMutation = useMutation({
    mutationFn: () => coolifyAPI.syncKey(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["coolify-key-check", id] }),
  });

  const updateMutation = useMutation({
    mutationFn: () => sshKeysAPI.update(id, {
      name: editForm.name || undefined,
      username: editForm.username,
      description: editForm.description,
      public_key: editForm.public_key || undefined,
      private_key: editForm.private_key || undefined,
      password: editForm.password || undefined,
      ...editGrants,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ssh-key", id] });
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
      setIsEditing(false);
      setEditError("");
      onUpdated?.();
    },
    onError: (err) => setEditError(err instanceof Error ? err.message : t("sshKey.genericFailed")),
  });

  const startEditing = () => {
    if (!data) return;
    setEditForm({
      name: data.name,
      username: data.username || "",
      description: data.description || "",
      public_key: data.public_key || "",
      private_key: data.private_key || "",
      password: data.password || "",
    });
    setEditError("");
    setIsEditing(true);
  };

  if (isLoading) return <div className="text-sm text-[var(--text-muted)]">{t("common.loading")}</div>;
  if (!data) return <div className="text-sm text-[var(--danger)]">{t("sshKey.notFound")}</div>;

  if (isEditing) {
    return (
      <form onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }} className="space-y-4">
        {editError && <div className="bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)] text-sm rounded-[var(--radius-md)] p-3">{editError}</div>}
        <Input label={t("common.name")} value={editForm.name} onChange={(e) => setEditForm((f) => ({ ...f, name: e.target.value }))} required />
        <Input label={t("sshKey.usernameLabel")} value={editForm.username} onChange={(e) => setEditForm((f) => ({ ...f, username: e.target.value }))} />
        <Input label={t("common.description")} value={editForm.description} onChange={(e) => setEditForm((f) => ({ ...f, description: e.target.value }))} />
        {data.credential_type === "key" && (
          <>
            <Textarea label={t("sshKey.publicKeyLabel")} value={editForm.public_key} onChange={(e) => setEditForm((f) => ({ ...f, public_key: e.target.value }))} rows={3} placeholder={t("sshKey.keepCurrentPlaceholder")} className="font-mono" />
            <Textarea label={t("sshKey.privateKeyLabel")} value={editForm.private_key} onChange={(e) => setEditForm((f) => ({ ...f, private_key: e.target.value }))} rows={4} placeholder={t("sshKey.keepCurrentPlaceholder")} className="font-mono" />
          </>
        )}
        {data.credential_type === "password" && (
          <Input label={t("sshKey.password")} type="password" value={editForm.password} onChange={(e) => setEditForm((f) => ({ ...f, password: e.target.value }))} placeholder={t("sshKey.keepCurrentPlaceholder")} />
        )}
        <EntidadeScopeFields value={editGrants} onChange={setEditGrants} compact loadFrom={{ type: "ssh_key", id }} />
        <div className="flex justify-end gap-2">
          <Button type="button" variant="secondary" onClick={() => setIsEditing(false)}>{t("common.cancel")}</Button>
          <Button type="submit" loading={updateMutation.isPending}>{t("common.save")}</Button>
        </div>
      </form>
    );
  }

  const isKey = data.credential_type === "key";

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <Badge color={isKey ? "cyan" : "accent"}>{isKey ? t("sshKey.sshKeyLabel") : t("sshKey.password")}</Badge>
          </div>
          <p className="text-sm font-semibold text-[var(--text-primary)] font-mono">{data.name}</p>
        </div>
        <Button size="sm" variant="secondary" onClick={startEditing}>{t("common.edit")}</Button>
      </div>
      {data.username && <Field label={t("sshKey.usernameLabel")} value={data.username} />}
      {data.description && <Field label={t("common.description")} value={data.description} />}
      {data.fingerprint && <Field label={t("sshKey.fingerprintLabel")} value={data.fingerprint} mono />}
      {isKey && data.public_key && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">{t("sshKey.publicKeyLabel")}</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto whitespace-pre-wrap break-all font-mono">
            {data.public_key}
          </pre>
        </div>
      )}
      {isKey && data.private_key && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">{t("sshKey.privateKeyLabel")}</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto whitespace-pre-wrap break-all font-mono">
            {data.private_key}
          </pre>
        </div>
      )}
      {!isKey && data.password && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">{t("sshKey.password")}</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] font-mono">
            {"•".repeat(12)}
          </pre>
        </div>
      )}

      {/* Coolify integration */}
      {coolifyStatus?.enabled && isKey && data.fingerprint && (
        <div className="pt-3 mt-3 border-t border-[var(--border-subtle)]">
          <div className="flex items-center gap-2">
            <Icon path={ICON_PATHS.serverStack} className="w-3.5 h-3.5 text-[var(--accent)] shrink-0" />
            <span className="text-xs font-medium text-[var(--text-primary)]">Coolify</span>
            {coolifyChecking ? (
              <span className="text-2xs text-[var(--text-faint)] ml-auto">{t("common.loading")}</span>
            ) : coolifyCheck?.found ? (
              <>
                <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-[var(--success)]/15 text-2xs text-[var(--success)] border border-[var(--success)]/20">
                  {coolifyCheck.coolify_name}
                </span>
                <span className="text-2xs text-[var(--text-faint)] ml-auto font-mono">
                  {coolifyCheck.coolify_uuid}
                </span>
              </>
            ) : (
              <>
                <span className="text-2xs text-[var(--text-faint)]">{t("operation.coolifyNotFound")}</span>
                <Button size="sm" variant="secondary" className="ml-auto" onClick={() => syncMutation.mutate()} loading={syncMutation.isPending}>
                  {t("operation.coolifySync")}
                </Button>
              </>
            )}
          </div>
          {syncMutation.isSuccess && (
            <p className="text-2xs text-[var(--success)] mt-1">
              {syncMutation.data?.already_existed ? t("sshKey.coolifyAlreadyExists") : t("sshKey.coolifySynced")}
            </p>
          )}
          {syncMutation.isError && (
            <p className="text-2xs text-[var(--danger)] mt-1">{syncMutation.error instanceof Error ? syncMutation.error.message : t("sshKey.genericFailed")}</p>
          )}
        </div>
      )}
    </div>
  );
}

