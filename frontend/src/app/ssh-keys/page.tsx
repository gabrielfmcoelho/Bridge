"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { sshKeysAPI, coolifyAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import StepIndicator from "@/components/ui/StepIndicator";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import type { SSHKeyRecord } from "@/lib/types";

export default function HostCredentialsPage() {
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
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("nav.hostCredentials")}</h1>
        <div className="flex items-center gap-1.5">
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={() => setShowForm(true)}><span className="mr-1">+</span> {t("common.add")}</Button>
            </div>
          )}
          {canEdit && (
            <IconButton variant="accent" size="md" onClick={() => setShowForm(true)} title={t("common.add")} className="sm:hidden">
              <svg className="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" /></svg>
            </IconButton>
          )}
        </div>
      </div>

      {/* Search */}
      <div className="flex items-center gap-2 mb-5">
        <div className="relative flex-1 max-w-sm min-w-[200px]">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-1.5 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
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
          title="No credentials stored"
          description="Add SSH keys or passwords to manage them centrally and associate with hosts."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {keys.map((cred, i) => (
            <div key={cred.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <CredentialCard cred={cred} onClick={() => setViewingKey(cred.id)} onDelete={canEdit ? () => { if (confirm(`Delete ${cred.name}?`)) deleteMutation.mutate(cred.id); } : undefined} />
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

      <ResponsiveModal open={viewingKey !== null} onClose={() => setViewingKey(null)} title="Details">
        {viewingKey !== null && <KeyView id={viewingKey} />}
      </ResponsiveModal>
    </PageShell>
  );
}

function CredentialCard({ cred, onClick, onDelete }: { cred: SSHKeyRecord; onClick: () => void; onDelete?: () => void }) {
  const isKey = cred.credential_type === "key";
  const borderColor = isKey ? "#06b6d4" : "#a855f7";

  return (
    <div onClick={onClick} className="cursor-pointer h-full">
      <Card className="h-full border-l-[3px] flex flex-col" style={{ borderLeftColor: borderColor }}>
        <div className="flex items-start justify-between mb-2">
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold text-[var(--text-primary)] text-sm truncate" style={{ fontFamily: "var(--font-mono)" }}>
              {cred.name}
            </h3>
            {cred.username && (
              <p className="text-[10px] text-[var(--text-faint)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
                {cred.username}
              </p>
            )}
          </div>
          <Badge color={isKey ? "cyan" : "purple"}>
            {isKey ? "SSH Key" : "Password"}
          </Badge>
        </div>

        {cred.description && (
          <p className="text-xs text-[var(--text-muted)] line-clamp-1 mb-2">{cred.description}</p>
        )}

        <div className="flex gap-1 mb-2">
          {isKey && (
            <>
              {cred.has_public_key && <Badge color="emerald">pub</Badge>}
              {cred.has_private_key && <Badge color="cyan">priv</Badge>}
            </>
          )}
          {!isKey && cred.has_password && <Badge color="purple">encrypted</Badge>}
        </div>

        {/* Consistent bottom section */}
        <div className="mt-auto pt-2 border-t border-[var(--border-subtle)]">
          {cred.fingerprint ? (
            <p className="text-[10px] text-[var(--text-faint)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
              {cred.fingerprint}
            </p>
          ) : (
            <p className="text-[10px] text-[var(--text-faint)]">&nbsp;</p>
          )}
          <div className="flex items-center justify-between mt-1">
            <p className="text-[10px] text-[var(--text-faint)]">
              {new Date(cred.created_at).toLocaleDateString()}
            </p>
            {onDelete && (
              <button
                onClick={(e) => { e.stopPropagation(); onDelete(); }}
                className="text-[10px] text-[var(--text-faint)] hover:text-red-400 transition-colors"
              >
                Delete
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
    }),
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  return (
    <div className="space-y-4">
      <StepIndicator steps={["Type", credType === "key" ? "Keys" : "Password"]} current={step} />
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          {/* Credential type selector */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-2">Type</label>
            <div className="grid grid-cols-2 gap-2">
              <button
                type="button"
                onClick={() => setCredType("key")}
                className={`p-3 rounded-[var(--radius-md)] border text-left transition-all ${
                  credType === "key"
                    ? "border-[var(--accent)]/30 bg-[var(--accent-muted)]"
                    : "border-[var(--border-default)] bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
                }`}
              >
                <div className="flex items-center gap-2 mb-1">
                  <svg className={`w-4 h-4 ${credType === "key" ? "text-[var(--accent)]" : "text-[var(--text-faint)]"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                  </svg>
                  <span className={`text-sm font-medium ${credType === "key" ? "text-[var(--accent)]" : "text-[var(--text-secondary)]"}`}>SSH Key</span>
                </div>
                <p className="text-[10px] text-[var(--text-faint)]">Public/private key pair</p>
              </button>
              <button
                type="button"
                onClick={() => setCredType("password")}
                className={`p-3 rounded-[var(--radius-md)] border text-left transition-all ${
                  credType === "password"
                    ? "border-purple-500/30 bg-purple-500/10"
                    : "border-[var(--border-default)] bg-[var(--bg-elevated)] hover:border-[var(--border-default)]"
                }`}
              >
                <div className="flex items-center gap-2 mb-1">
                  <svg className={`w-4 h-4 ${credType === "password" ? "text-purple-400" : "text-[var(--text-faint)]"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  <span className={`text-sm font-medium ${credType === "password" ? "text-purple-400" : "text-[var(--text-secondary)]"}`}>Password</span>
                </div>
                <p className="text-[10px] text-[var(--text-faint)]">Encrypted password storage</p>
              </button>
            </div>
          </div>
          <Input label="Name" value={form.name} onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))} required placeholder="e.g. prod-server-key" />
          <Input label="Username" value={form.username} onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))} placeholder="e.g. root, admin" />
          <Input label="Description" value={form.description} onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))} placeholder="Optional description" />
          <p className="text-[10px] text-[var(--text-faint)]">
            All credentials are encrypted with AES-256-GCM before storage.
          </p>
          <Button type="button" className="w-full" disabled={!form.name.trim()} onClick={() => setStep(2)}>
            {t("host.nextStep")}
          </Button>
        </div>
      )}

      {step === 2 && credType === "key" && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Textarea label="Public Key" value={form.public_key} onChange={(e) => setForm((f) => ({ ...f, public_key: e.target.value }))} rows={3} placeholder="ssh-ed25519 AAAA..." className="font-mono" />
          <Textarea label="Private Key" value={form.private_key} onChange={(e) => setForm((f) => ({ ...f, private_key: e.target.value }))} rows={4} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" className="font-mono" />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{t("common.create")}</Button>
          </div>
        </form>
      )}

      {step === 2 && credType === "password" && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Input label="Password" type="password" value={form.password} onChange={(e) => setForm((f) => ({ ...f, password: e.target.value }))} required placeholder="Enter password" />
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
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["ssh-key", id] });
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
      setIsEditing(false);
      setEditError("");
      onUpdated?.();
    },
    onError: (err) => setEditError(err instanceof Error ? err.message : "Failed"),
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

  if (isLoading) return <div className="text-sm text-[var(--text-muted)]">Loading...</div>;
  if (!data) return <div className="text-sm text-red-400">Credential not found</div>;

  if (isEditing) {
    return (
      <form onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }} className="space-y-4">
        {editError && <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3">{editError}</div>}
        <Input label="Name" value={editForm.name} onChange={(e) => setEditForm((f) => ({ ...f, name: e.target.value }))} required />
        <Input label="Username" value={editForm.username} onChange={(e) => setEditForm((f) => ({ ...f, username: e.target.value }))} />
        <Input label="Description" value={editForm.description} onChange={(e) => setEditForm((f) => ({ ...f, description: e.target.value }))} />
        {data.credential_type === "key" && (
          <>
            <Textarea label="Public Key" value={editForm.public_key} onChange={(e) => setEditForm((f) => ({ ...f, public_key: e.target.value }))} rows={3} placeholder="Leave empty to keep current" className="font-mono" />
            <Textarea label="Private Key" value={editForm.private_key} onChange={(e) => setEditForm((f) => ({ ...f, private_key: e.target.value }))} rows={4} placeholder="Leave empty to keep current" className="font-mono" />
          </>
        )}
        {data.credential_type === "password" && (
          <Input label="Password" type="password" value={editForm.password} onChange={(e) => setEditForm((f) => ({ ...f, password: e.target.value }))} placeholder="Leave empty to keep current" />
        )}
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
            <Badge color={isKey ? "cyan" : "purple"}>{isKey ? "SSH Key" : "Password"}</Badge>
          </div>
          <p className="text-sm font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{data.name}</p>
        </div>
        <Button size="sm" variant="secondary" onClick={startEditing}>{t("common.edit")}</Button>
      </div>
      {data.username && <Field label="Username" value={data.username} />}
      {data.description && <Field label="Description" value={data.description} />}
      {data.fingerprint && <Field label="Fingerprint" value={data.fingerprint} mono />}
      {isKey && data.public_key && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">Public Key</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto whitespace-pre-wrap break-all" style={{ fontFamily: "var(--font-mono)" }}>
            {data.public_key}
          </pre>
        </div>
      )}
      {isKey && data.private_key && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">Private Key</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto whitespace-pre-wrap break-all" style={{ fontFamily: "var(--font-mono)" }}>
            {data.private_key}
          </pre>
        </div>
      )}
      {!isKey && data.password && (
        <div>
          <span className="text-xs text-[var(--text-muted)]">Password</span>
          <pre className="mt-1 p-3 bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
            {"•".repeat(12)}
          </pre>
        </div>
      )}

      {/* Coolify integration */}
      {coolifyStatus?.enabled && isKey && data.fingerprint && (
        <div className="pt-3 mt-3 border-t border-[var(--border-subtle)]">
          <div className="flex items-center gap-2">
            <svg className="w-3.5 h-3.5 text-violet-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
            </svg>
            <span className="text-xs font-medium text-[var(--text-primary)]">Coolify</span>
            {coolifyChecking ? (
              <span className="text-[10px] text-[var(--text-faint)] ml-auto">{t("common.loading")}</span>
            ) : coolifyCheck?.found ? (
              <>
                <span className="shrink-0 px-1.5 py-0.5 rounded-full bg-emerald-500/15 text-[10px] text-emerald-400 border border-emerald-500/20">
                  {coolifyCheck.coolify_name}
                </span>
                <span className="text-[10px] text-[var(--text-faint)] ml-auto" style={{ fontFamily: "var(--font-mono)" }}>
                  {coolifyCheck.coolify_uuid}
                </span>
              </>
            ) : (
              <>
                <span className="text-[10px] text-[var(--text-faint)]">{t("operation.coolifyNotFound")}</span>
                <Button size="sm" variant="secondary" className="ml-auto" onClick={() => syncMutation.mutate()} loading={syncMutation.isPending}>
                  {t("operation.coolifySync")}
                </Button>
              </>
            )}
          </div>
          {syncMutation.isSuccess && (
            <p className="text-[10px] text-emerald-400 mt-1">
              {syncMutation.data?.already_existed ? t("sshKey.coolifyAlreadyExists") : t("sshKey.coolifySynced")}
            </p>
          )}
          {syncMutation.isError && (
            <p className="text-[10px] text-red-400 mt-1">{syncMutation.error instanceof Error ? syncMutation.error.message : "Failed"}</p>
          )}
        </div>
      )}
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <span className="text-xs text-[var(--text-muted)]">{label}</span>
      <p className={`text-sm text-[var(--text-secondary)] ${mono ? "" : ""}`} style={mono ? { fontFamily: "var(--font-mono)" } : undefined}>
        {value}
      </p>
    </div>
  );
}
