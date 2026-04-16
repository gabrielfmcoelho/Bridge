"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { hostsAPI, enumsAPI, sshKeysAPI, contactsAPI, usersAPI, dnsAPI, servicesAPI, projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useMultiStepFormEffects } from "@/hooks/useMultiStepForm";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import FormError from "@/components/ui/FormError";
import DrawerSection from "@/components/ui/DrawerSection";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import CheckboxList from "@/components/ui/CheckboxList";
import ResponsavelList from "./_components/ResponsavelList";
import ChamadoList from "./_components/ChamadoList";
import type { Host, DNSRecord, Service, Project, HostResponsavel, HostChamado } from "@/lib/types";

interface HostFormProps {
  host?: Host;
  tags?: string[];
  responsaveis?: HostResponsavel[];
  chamados?: HostChamado[];
  dnsRecords?: DNSRecord[];
  services?: Service[];
  projects?: Project[];
  onSuccess: () => void;
  onClose?: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

const STEP_COUNT = 4;

export default function HostForm({
  host, tags, responsaveis, chamados, dnsRecords, services: linkedServices, projects: linkedProjects,
  onSuccess, onClose, onFooterChange, onSubHeaderChange,
}: HostFormProps) {
  const { t } = useLocale();
  const isEdit = !!host;

  const [form, setForm] = useState({
    nickname: host?.nickname ?? "",
    oficial_slug: host?.oficial_slug ?? "",
    hostname: host?.hostname ?? "",
    user: host?.user ?? "",
    port: host?.port || "22",
    hospedagem: host?.hospedagem ?? "",
    tipo_maquina: host?.tipo_maquina ?? "",
    description: host?.description ?? "",
    situacao: host?.situacao || "active",
    preferred_auth: host?.preferred_auth ?? "",
    password: "",
    proxy_jump: host?.proxy_jump ?? "",
    forward_agent: host?.forward_agent ?? "",
    observacoes: host?.observacoes ?? "",
  });
  const [formTags, setFormTags] = useState<string[]>(tags ?? []);
  const [formResponsaveis, setFormResponsaveis] = useState<HostResponsavel[]>(responsaveis ?? []);
  const [formChamados, setFormChamados] = useState<HostChamado[]>(chamados ?? []);
  const [selectedKeyId, setSelectedKeyId] = useState("");
  const [linkedDnsIds, setLinkedDnsIds] = useState<number[]>(dnsRecords?.map((d) => d.id) ?? []);
  const [linkedServiceIds, setLinkedServiceIds] = useState<number[]>(linkedServices?.map((s) => s.id) ?? []);
  const [linkedProjectIds, setLinkedProjectIds] = useState<number[]>(linkedProjects?.map((p) => p.id) ?? []);
  const [error, setError] = useState("");

  // Create mode: step-based | Edit mode: section-based
  const [step, setStep] = useState(1);
  const [openSection, setOpenSection] = useState<string>("basic");
  // Inner collapsible sections within steps 3 and 4
  const [openStepSection, setOpenStepSection] = useState<string | null>(null);
  const toggleStepSection = (key: string) =>
    setOpenStepSection((prev) => (prev === key ? null : key));

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
  const { data: rawEntidades } = useQuery({
    queryKey: ["enums", "entidade_responsavel"],
    queryFn: () => enumsAPI.list("entidade_responsavel"),
  });
  const entidades = Array.isArray(rawEntidades) ? rawEntidades : [];
  const { data: rawContacts } = useQuery({
    queryKey: ["contacts"],
    queryFn: contactsAPI.list,
  });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: sshKeys = [] } = useQuery({
    queryKey: ["ssh-keys"],
    queryFn: sshKeysAPI.list,
  });
  const { data: rawUsers = [] } = useQuery({
    queryKey: ["users"],
    queryFn: usersAPI.list,
  });
  const users = Array.isArray(rawUsers)
    ? rawUsers.map((u) => ({ id: u.id, display_name: u.display_name }))
    : [];

  // Lists for linking step
  const { data: allDns = [] } = useQuery({
    queryKey: ["dns"],
    queryFn: dnsAPI.list,
  });
  const { data: allServices = [] } = useQuery({
    queryKey: ["services"],
    queryFn: servicesAPI.list,
  });
  const { data: allProjects = [] } = useQuery({
    queryKey: ["projects"],
    queryFn: projectsAPI.list,
  });

  /* ── Helpers ─────────────────────────────────────────────────────── */

  const set = (key: string, value: string | boolean) =>
    setForm((f) => ({ ...f, [key]: value }));

  const toggleSection = (key: string) =>
    setOpenSection((prev) => (prev === key ? "" : key));

  /* ── Mutation ────────────────────────────────────────────────────── */

  const mutation = useMutation({
    mutationFn: async () => {
      const effectiveHasPassword = Boolean(form.password) || (isEdit && Boolean(host?.has_password));
      const effectiveHasKey = Boolean(selectedKeyId);
      const preferredAuth = resolvePreferredAuth(effectiveHasPassword, effectiveHasKey, String(form.preferred_auth || ""));
      if (!preferredAuth.valid) {
        setError(preferredAuth.error);
        throw new Error(preferredAuth.error);
      }
      const payload: Record<string, unknown> = {
        ...form,
        preferred_auth: preferredAuth.value,
        tags: formTags,
        password: form.password || undefined,
        responsaveis: formResponsaveis,
        chamados: formChamados,
        dns_ids: linkedDnsIds,
        service_ids: linkedServiceIds,
        project_ids: linkedProjectIds,
      };
      if (selectedKeyId) {
        payload.ssh_key_id = parseInt(selectedKeyId);
      } else if (isEdit && host?.has_key) {
        payload.clear_key = true;
      }
      if (isEdit) {
        return hostsAPI.update(host.oficial_slug, payload);
      }
      return hostsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  /* ── SSH key selection is driven entirely by the picker; the backend
     stores the encrypted key bytes on the host row itself, so there is no
     stable filesystem path to auto-match against on edit. ─────────────── */

  /* ── Step validation ─────────────────────────────────────────────── */

  const canProceedStep1 = form.nickname.trim().length > 0 && form.oficial_slug.trim().length > 0;

  /* ── Footer / SubHeader ──────────────────────────────────────────── */

  useMultiStepFormEffects({
    step,
    setStep,
    totalSteps: STEP_COUNT,
    stepLabels: [t("host.basicInfo"), t("host.sshConnection"), t("host.responsibility"), t("host.links")],
    onSubmit: () => mutation.mutate(),
    canProceed: step === 1 ? canProceedStep1 : true,
    isPending: mutation.isPending,
    isEditMode: isEdit,
    onClose,
    t,
    onFooterChange,
    onSubHeaderChange,
  });

  // Reset open section when step changes in create mode
  useEffect(() => {
    if (!isEdit) setOpenStepSection(null);
  }, [step, isEdit]);

  /* ── Shared field groups ─────────────────────────────────────────── */

  const basicInfoFields = (
    <>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Input label={t("host.nickname")} value={form.nickname} onChange={(e) => set("nickname", e.target.value)} required />
        <Input label={t("host.oficialSlug")} value={form.oficial_slug} onChange={(e) => set("oficial_slug", e.target.value)} required />
        <Input label={t("host.hostname")} value={form.hostname} onChange={(e) => set("hostname", e.target.value)} />
        <Select label={t("host.hospedagem")} value={form.hospedagem} onChange={(e) => set("hospedagem", e.target.value)} options={hospedagens.map((e) => ({ value: e.value, label: e.value }))} />
        <Select label={t("host.tipoMaquina")} value={form.tipo_maquina} onChange={(e) => set("tipo_maquina", e.target.value)} options={tipoMaquinas.map((e) => ({ value: e.value, label: e.value }))} />
        <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((e) => ({ value: e.value, label: e.value }))} />
      </div>
      <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} />
      <TagInput label={t("common.tags")} tags={formTags} onChange={setFormTags} entityType="host" />
    </>
  );

  const sshFields = (
    <>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Input label={t("host.user")} value={form.user} onChange={(e) => set("user", e.target.value)} />
        <Input label={t("host.port")} value={form.port} onChange={(e) => set("port", e.target.value)} />
        <Input label={t("host.proxyJump")} value={form.proxy_jump} onChange={(e) => set("proxy_jump", e.target.value)} placeholder="bastion-host" />
        <Select
          label={t("host.forwardAgent")}
          value={form.forward_agent}
          onChange={(e) => set("forward_agent", e.target.value)}
          options={[
            { value: "yes", label: t("common.yes") },
            { value: "no", label: t("common.no") },
          ]}
        />
      </div>
      {sshKeys.length > 0 ? (
        <Select
          label={t("host.sshKey")}
          value={selectedKeyId}
          onChange={(e) => setSelectedKeyId(e.target.value)}
          options={sshKeys.map((k) => ({
            value: k.id.toString(),
            label: `${k.name}${k.fingerprint ? ` (${k.fingerprint})` : ""}`,
          }))}
        />
      ) : isEdit && host?.has_key ? (
        <div className="space-y-1.5">
          <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">{t("host.sshKey")}</label>
          <p className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2">
            {t("host.sshKeyStored")}
          </p>
        </div>
      ) : null}
      <Input
        label={t("auth.password")}
        type="password"
        value={form.password}
        onChange={(e) => set("password", e.target.value)}
        placeholder={isEdit ? "Leave empty to keep current" : undefined}
      />
      <Select
        label={t("host.defaultAuth")}
        value={form.preferred_auth as string}
        onChange={(e) => set("preferred_auth", e.target.value)}
        options={[
          { value: "", label: "Auto" },
          { value: "password", label: t("auth.password") },
          { value: "key", label: t("host.sshKey") },
        ]}
      />
    </>
  );

  const responsaveisFields = (
    <div className="space-y-0">
      <DrawerSection title={t("project.responsaveis")} open={openStepSection === "responsaveis"} onToggle={() => toggleStepSection("responsaveis")} active={formResponsaveis.length > 0}>
        <ResponsavelList
          value={formResponsaveis}
          onChange={setFormResponsaveis}
          contacts={contacts}
          entidades={entidades}
          t={t}
        />
      </DrawerSection>
      <DrawerSection title={t("host.chamados")} open={openStepSection === "chamados"} onToggle={() => toggleStepSection("chamados")} active={formChamados.length > 0}>
        <ChamadoList
          value={formChamados}
          onChange={setFormChamados}
          users={users}
          t={t}
        />
      </DrawerSection>
      <DrawerSection title={t("common.observacoes")} open={openStepSection === "observacoes"} onToggle={() => toggleStepSection("observacoes")} active={!!form.observacoes}>
        <MarkdownEditor
          value={form.observacoes as string}
          onChange={(v) => set("observacoes", v)}
          rows={4}
          placeholder="Markdown..."
        />
      </DrawerSection>
    </div>
  );

  const linksFields = (
    <div className="space-y-0">
      <DrawerSection title={t("host.linkedDns")} open={openStepSection === "dns"} onToggle={() => toggleStepSection("dns")} active={linkedDnsIds.length > 0}>
        <CheckboxList
          label=""
          items={allDns.map((d) => ({ id: d.id, name: d.domain }))}
          selected={linkedDnsIds}
          onChange={setLinkedDnsIds}
        />
      </DrawerSection>
      <DrawerSection title={t("host.linkedServices")} open={openStepSection === "services"} onToggle={() => toggleStepSection("services")} active={linkedServiceIds.length > 0}>
        <CheckboxList
          label=""
          items={allServices.map((s) => ({ id: s.id, name: s.nickname }))}
          selected={linkedServiceIds}
          onChange={setLinkedServiceIds}
        />
      </DrawerSection>
      <DrawerSection title={t("host.linkedProjects")} open={openStepSection === "projects"} onToggle={() => toggleStepSection("projects")} active={linkedProjectIds.length > 0}>
        <CheckboxList
          label=""
          items={allProjects.map((p) => ({ id: p.id, name: p.name }))}
          selected={linkedProjectIds}
          onChange={setLinkedProjectIds}
        />
      </DrawerSection>
    </div>
  );

  /* ── Render ──────────────────────────────────────────────────────── */

  if (isEdit) {
    // Edit mode: collapsible sections for quick random access
    return (
      <div className="space-y-0">
        <FormError message={error} />

        <DrawerSection title={t("host.basicInfo")} open={openSection === "basic"} onToggle={() => toggleSection("basic")}>
          {basicInfoFields}
        </DrawerSection>

        <DrawerSection title={t("host.sshConnection")} open={openSection === "ssh"} onToggle={() => toggleSection("ssh")}>
          {sshFields}
        </DrawerSection>

        <DrawerSection title={t("host.responsibility")} open={openSection === "responsaveis"} onToggle={() => toggleSection("responsaveis")}>
          {responsaveisFields}
        </DrawerSection>

        <DrawerSection title={t("host.links")} open={openSection === "links"} onToggle={() => toggleSection("links")}>
          {linksFields}
        </DrawerSection>
      </div>
    );
  }

  // Create mode: stepped wizard
  return (
    <div className="space-y-4">
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-3 animate-fade-in">
          {basicInfoFields}
        </div>
      )}

      {step === 2 && (
        <div className="space-y-3 animate-fade-in">
          {sshFields}
        </div>
      )}

      {step === 3 && (
        <div className="space-y-3 animate-fade-in">
          {responsaveisFields}
        </div>
      )}

      {step === 4 && (
        <div className="space-y-3 animate-fade-in">
          {linksFields}
        </div>
      )}
    </div>
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
      error: "Select default auth method (password or key) when both are configured.",
    };
  }
  if (hasPassword) return { valid: true as const, value: "password", error: "" };
  if (hasKey) return { valid: true as const, value: "key", error: "" };
  return { valid: true as const, value: "", error: "" };
}
