"use client";

import { useSituacao } from "@/hooks/useSituacao";
import { useDefaultSituacao } from "@/hooks/useDefaultSituacao";
import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { hostsAPI, enumsAPI, secretsAPI, contactsAPI, usersAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import FormField from "@/components/ui/FormField";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import GrafanaUidField from "@/components/forms/GrafanaUidField";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import ChamadoList from "./_components/ChamadoList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import { useAuth } from "@/contexts/AuthContext";
import type { Host, DNSRecord, Service, Project, HostResponsavel, HostChamado, AssetGrants, AssetGrantsInput } from "@/lib/types";

interface HostFormProps {
  host?: Host;
  tags?: string[];
  responsaveis?: HostResponsavel[];
  chamados?: HostChamado[];
  entidades?: AssetGrants;
  dnsRecords?: DNSRecord[];
  services?: Service[];
  projects?: Project[];
  onSuccess: () => void;
  onClose?: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}


export default function HostForm({
  host, tags, responsaveis, chamados, entidades, dnsRecords, services: linkedServices, projects: linkedProjects,
  onSuccess, onClose, onFooterChange, onSubHeaderChange,
}: HostFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!host;

  // New records start in whichever situação carries the "active" role.
  const { valueOf: situacaoValueOf } = useSituacao();
  const [form, setForm] = useState({
    nickname: host?.nickname ?? "",
    oficial_slug: host?.oficial_slug ?? "",
    hostname: host?.hostname ?? "",
    user: host?.user ?? "",
    port: host?.port || "22",
    hospedagem: host?.hospedagem ?? "",
    tipo_maquina: host?.tipo_maquina ?? "",
    description: host?.description ?? "",
    situacao: host?.situacao || situacaoValueOf("active"),
    preferred_auth: host?.preferred_auth ?? "",
    password: "",
    proxy_jump: host?.proxy_jump ?? "",
    forward_agent: host?.forward_agent ?? "",
    observacoes: host?.observacoes ?? "",
    grafana_dashboard_uid: host?.grafana_dashboard_uid ?? "",
  });
  const [formTags, setFormTags] = useState<string[]>(tags ?? []);
  const [formResponsaveis, setFormResponsaveis] = useState<HostResponsavel[]>(responsaveis ?? []);
  const [formChamados, setFormChamados] = useState<HostChamado[]>(chamados ?? []);
  const [grants, setGrants] = useState<AssetGrantsInput>(entidades ?? defaultGrants(user));
  // selectedKeyId is three-valued:
  //   - null           → untouched; don't send key_secret_id OR clear_key
  //   - "__clear__"    → user explicitly chose to unlink the current key
  //   - any digit id   → link that shared vault key
  // The plain empty-string state was ambiguous and caused edit-without-key-
  // touch saves to silently wipe the stored key in production.
  const [selectedKeyId, setSelectedKeyId] = useState<string | null>(null);
  const [linkedDnsIds, setLinkedDnsIds] = useState<number[]>(dnsRecords?.map((d) => d.id) ?? []);
  const [linkedServiceIds, setLinkedServiceIds] = useState<number[]>(linkedServices?.map((s) => s.id) ?? []);
  const [linkedProjectIds, setLinkedProjectIds] = useState<number[]>(linkedProjects?.map((p) => p.id) ?? []);
  const [error, setError] = useState("");

  // Field errors show after the first save attempt.
  const [attempted, setAttempted] = useState(false);

  /* ── Queries ─────────────────────────────────────────────────────── */

  const { data: hospedagens = [] } = useQuery({
    queryKey: ["enums", "hospedagem"],
    queryFn: () => enumsAPI.list("hospedagem"),
  });
  const { data: tipoMaquinas = [] } = useQuery({
    queryKey: ["enums", "tipo_maquina"],
    queryFn: () => enumsAPI.list("tipo_maquina"),
  });
  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const { data: rawContacts } = useQuery({
    queryKey: ["contacts"],
    queryFn: contactsAPI.list,
  });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  // Shared SSH keys in the vault; the host links one (no copy is made).
  const { data: sshKeys = [] } = useQuery({
    queryKey: ["secrets-all", "shared-keys"],
    queryFn: secretsAPI.sharedKeys,
  });
  const { data: rawUsers = [] } = useQuery({
    queryKey: ["users"],
    queryFn: usersAPI.list,
  });
  const users = Array.isArray(rawUsers)
    ? rawUsers.map((u) => ({ id: u.id, display_name: u.display_name }))
    : [];

  const relationOptions = useRelationOptions(["dns", "services", "projects"]);

  /* ── Helpers ─────────────────────────────────────────────────────── */

  const set = (key: string, value: string | boolean) =>
    setForm((f) => ({ ...f, [key]: value }));
  useDefaultSituacao(form.situacao, (v) => set("situacao", v), !isEdit);

  /* ── Mutation ────────────────────────────────────────────────────── */

  const mutation = useMutation({
    mutationFn: async () => {
      const effectiveHasPassword = Boolean(form.password) || (isEdit && Boolean(host?.has_password));
      const willLinkNewKey = selectedKeyId !== null && selectedKeyId !== "" && selectedKeyId !== "__clear__";
      const willClearKey = selectedKeyId === "__clear__";
      const keptExistingKey = isEdit && Boolean(host?.has_key) && !willClearKey && !willLinkNewKey;
      const effectiveHasKey = willLinkNewKey || keptExistingKey;
      const preferredAuth = resolvePreferredAuth(effectiveHasPassword, effectiveHasKey, String(form.preferred_auth || ""));
      if (!preferredAuth.valid) throw new Error(t(preferredAuth.error));
      const payload: Record<string, unknown> = {
        ...form,
        preferred_auth: preferredAuth.value,
        tags: formTags,
        password: form.password || undefined,
        responsaveis: formResponsaveis,
        chamados: formChamados,
        ...grants,
        dns_ids: linkedDnsIds,
        service_ids: linkedServiceIds,
        project_ids: linkedProjectIds,
      };
      if (willLinkNewKey) {
        payload.key_secret_id = parseInt(selectedKeyId as string);
      } else if (willClearKey) {
        payload.clear_key = true;
      }
      // When selectedKeyId is null (untouched), we send neither field, so
      // the backend preserves whatever key the host currently has.
      if (isEdit) {
        return hostsAPI.update(host.oficial_slug, payload);
      }
      return hostsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  /* ── Validation ──────────────────────────────────────────────────── */

  const portNum = Number(form.port);
  const errors: Record<string, string> = {};
  if (!form.nickname.trim()) errors.nickname = t("form.required");
  if (!form.oficial_slug.trim()) errors.oficial_slug = t("form.required");
  // Formats only for what was typed here, so older records stay editable.
  else if (form.oficial_slug !== (host?.oficial_slug ?? "") && !/^[A-Za-z0-9._-]+$/.test(form.oficial_slug.trim())) errors.oficial_slug = t("form.slugInvalid");
  if (form.port && form.port !== (host?.port ?? "") && !(Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535)) errors.port = t("form.portInvalid");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  /* ── Render ──────────────────────────────────────────────────────── */

  const sections = [
    { id: "host-identity", label: t("form.section.identity") },
    { id: "host-operation", label: t("form.section.operation") },
    { id: "host-access", label: t("form.section.access") },
    { id: "host-owners", label: t("form.section.owners") },
    { id: "host-links", label: t("form.section.links") },
    { id: "host-notes", label: t("form.section.notes") },
  ];

  return (
    <EntityFormShell
      id="host-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("form.createHost")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="host-identity" title={t("form.section.identity")} description={t("form.section.identityHostHint")}>
        <Input label={t("host.nickname")} value={form.nickname} onChange={(e) => set("nickname", e.target.value)} required error={err("nickname")} aria-invalid={!!err("nickname")} />
        <Input label={t("host.oficialSlug")} value={form.oficial_slug} onChange={(e) => set("oficial_slug", e.target.value)} required error={err("oficial_slug")} aria-invalid={!!err("oficial_slug")} className="font-mono" hint={t("form.slugHint")} />
        <Input label={t("host.hostname")} value={form.hostname} onChange={(e) => set("hostname", e.target.value)} className="font-mono" placeholder="10.0.0.12" />
        <div className="sm:col-span-2">
          <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} />
        </div>
      </FormSection>

      <FormSection id="host-operation" title={t("form.section.operation")}>
        <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((e) => ({ value: e.value, label: e.value }))} />
        <Select label={t("host.hospedagem")} value={form.hospedagem} onChange={(e) => set("hospedagem", e.target.value)} options={hospedagens.map((e) => ({ value: e.value, label: e.value }))} />
        <Select label={t("host.tipoMaquina")} value={form.tipo_maquina} onChange={(e) => set("tipo_maquina", e.target.value)} options={tipoMaquinas.map((e) => ({ value: e.value, label: e.value }))} />
        <div className="sm:col-span-2">
          <GrafanaUidField kind="host" target={isEdit ? host?.oficial_slug : undefined} value={form.grafana_dashboard_uid} onChange={(v) => set("grafana_dashboard_uid", v)} />
        </div>
      </FormSection>

      <FormSection id="host-access" title={t("form.section.access")} description={t("form.section.accessHint")}>
        <Input label={t("host.user")} value={form.user} onChange={(e) => set("user", e.target.value)} className="font-mono" />
        <Input label={t("host.port")} value={form.port} onChange={(e) => set("port", e.target.value)} inputMode="numeric" className="font-mono" error={err("port")} aria-invalid={!!err("port")} />
        <Input label={t("host.proxyJump")} value={form.proxy_jump} onChange={(e) => set("proxy_jump", e.target.value)} placeholder="bastion-host" className="font-mono" />
        <Select
          label={t("host.forwardAgent")}
          value={form.forward_agent}
          onChange={(e) => set("forward_agent", e.target.value)}
          options={[
            { value: "yes", label: t("common.yes") },
            { value: "no", label: t("common.no") },
          ]}
        />
        {sshKeys.length > 0 ? (
          <div className="sm:col-span-2">
            <Select
              label={t("host.sshKey")}
              value={selectedKeyId ?? ""}
              onChange={(e) => setSelectedKeyId(e.target.value === "" ? null : e.target.value)}
              hint={selectedKeyId === "__clear__" ? undefined : isEdit && host?.has_key && selectedKeyId === null ? t("host.sshKeyKeepCurrentHint") : undefined}
              options={[
                { value: "", label: isEdit && host?.has_key ? t("host.sshKeyKeepCurrent") : t("host.sshKeyNone") },
                ...(isEdit && host?.has_key ? [{ value: "__clear__", label: t("host.sshKeyClear") }] : []),
                ...sshKeys.map((k) => ({ value: k.id.toString(), label: [k.name, k.username, k.ssh_fingerprint].filter(Boolean).join(" · ") })),
              ]}
            />
            {selectedKeyId === "__clear__" && <p className="mt-1.5 text-xs text-[var(--warning)]">{t("host.sshKeyClearHint")}</p>}
          </div>
        ) : isEdit && host?.has_key ? (
          <div className="sm:col-span-2">
            <FormField label={t("host.sshKey")}>
              <p className="text-sm text-[var(--text-secondary)]">{t("host.sshKeyStored")}</p>
            </FormField>
          </div>
        ) : null}
        <Input
          label={t("auth.password")}
          type="password"
          autoComplete="new-password"
          value={form.password}
          onChange={(e) => set("password", e.target.value)}
          placeholder={isEdit ? t("host.passwordKeepCurrentPlaceholder") : undefined}
        />
        <Select
          label={t("host.defaultAuth")}
          value={form.preferred_auth as string}
          onChange={(e) => set("preferred_auth", e.target.value)}
          options={[
            { value: "", label: t("host.scanAuthMethodAuto") },
            { value: "password", label: t("auth.password") },
            { value: "key", label: t("host.sshKey") },
          ]}
        />
      </FormSection>

      <FormSection id="host-owners" title={t("form.section.owners")} description={t("form.section.ownersHint")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
        <ResponsavelList value={formResponsaveis} onChange={setFormResponsaveis} contacts={contacts} t={t} />
        <FormField label={t("host.chamados")}>
          <ChamadoList value={formChamados} onChange={setFormChamados} users={users} t={t} />
        </FormField>
      </FormSection>

      <FormSection id="host-links" title={t("form.section.links")} description={t("form.section.linksHint")} stack>
        <RelationPicker label={t("topology.services")} options={relationOptions.services} selected={linkedServiceIds} onChange={setLinkedServiceIds} />
        <RelationPicker label={t("topology.dnsRecords")} options={relationOptions.dns} selected={linkedDnsIds} onChange={setLinkedDnsIds} />
        <RelationPicker label={t("topology.projects")} options={relationOptions.projects} selected={linkedProjectIds} onChange={setLinkedProjectIds} />
      </FormSection>

      <FormSection id="host-notes" title={t("form.section.notes")} stack>
        <TagInput label={t("common.tags")} tags={formTags} onChange={setFormTags} entityType="host" />
        <FormField label={t("common.observacoes")}>
          <MarkdownEditor value={form.observacoes as string} onChange={(v) => set("observacoes", v)} rows={4} placeholder={t("form.markdownPlaceholder")} />
        </FormField>
      </FormSection>
    </EntityFormShell>
  );
}

/* ─── Validation ──────────────────────────────────────────────────── */

function resolvePreferredAuth(hasPassword: boolean, hasKey: boolean, preferredAuth: string) {
  if (hasPassword && hasKey) {
    if (preferredAuth === "password" || preferredAuth === "key") {
      return { valid: true as const, value: preferredAuth, error: "" };
    }
    return {
      valid: false as const,
      value: "",
      error: "host.authPickDefault",
    };
  }
  if (hasPassword) return { valid: true as const, value: "password", error: "" };
  if (hasKey) return { valid: true as const, value: "key", error: "" };
  return { valid: true as const, value: "", error: "" };
}
