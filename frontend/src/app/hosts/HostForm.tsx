"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { hostsAPI, enumsAPI, sshKeysAPI, contactsAPI, usersAPI, dnsAPI, servicesAPI, projectsAPI, grafanaAPI, integrationsAPI } from "@/lib/api";
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
import EntidadeList from "./_components/EntidadeList";
import type { Host, DNSRecord, Service, Project, HostResponsavel, HostChamado, HostEntidade } from "@/lib/types";

interface HostFormProps {
  host?: Host;
  tags?: string[];
  responsaveis?: HostResponsavel[];
  chamados?: HostChamado[];
  entidades?: HostEntidade[];
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
  host, tags, responsaveis, chamados, entidades, dnsRecords, services: linkedServices, projects: linkedProjects,
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
    grafana_dashboard_uid: host?.grafana_dashboard_uid ?? "",
  });
  const [formTags, setFormTags] = useState<string[]>(tags ?? []);
  const [formResponsaveis, setFormResponsaveis] = useState<HostResponsavel[]>(responsaveis ?? []);
  const [formChamados, setFormChamados] = useState<HostChamado[]>(chamados ?? []);
  const [formEntidades, setFormEntidades] = useState<HostEntidade[]>(entidades ?? []);
  // selectedKeyId is three-valued:
  //   - null           → untouched; don't send ssh_key_id OR clear_key
  //   - "__clear__"    → user explicitly chose to unlink the current key
  //   - any digit id   → link that ssh_keys row
  // The plain empty-string state was ambiguous and caused edit-without-key-
  // touch saves to silently wipe the stored key in production.
  const [selectedKeyId, setSelectedKeyId] = useState<string | null>(null);
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
  const { data: rawEntidadeOptions } = useQuery({
    queryKey: ["enums", "entidade_responsavel"],
    queryFn: () => enumsAPI.list("entidade_responsavel"),
  });
  const entidadeOptions = Array.isArray(rawEntidadeOptions) ? rawEntidadeOptions : [];
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
      const willLinkNewKey = selectedKeyId !== null && selectedKeyId !== "" && selectedKeyId !== "__clear__";
      const willClearKey = selectedKeyId === "__clear__";
      const keptExistingKey = isEdit && Boolean(host?.has_key) && !willClearKey && !willLinkNewKey;
      const effectiveHasKey = willLinkNewKey || keptExistingKey;
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
        entidades: formEntidades.map((e) => ({ entidade: e.entidade, is_main: e.is_main })),
        dns_ids: linkedDnsIds,
        service_ids: linkedServiceIds,
        project_ids: linkedProjectIds,
      };
      if (willLinkNewKey) {
        payload.ssh_key_id = parseInt(selectedKeyId as string);
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
        <div className="space-y-1">
          <Select
            label={t("host.sshKey")}
            value={selectedKeyId ?? ""}
            onChange={(e) => setSelectedKeyId(e.target.value === "" ? null : e.target.value)}
            options={[
              {
                value: "",
                label: isEdit && host?.has_key
                  ? t("host.sshKeyKeepCurrent")
                  : t("host.sshKeyNone"),
              },
              ...(isEdit && host?.has_key
                ? [{ value: "__clear__", label: t("host.sshKeyClear") }]
                : []),
              ...sshKeys.map((k) => ({
                value: k.id.toString(),
                label: `${k.name}${k.fingerprint ? ` (${k.fingerprint})` : ""}`,
              })),
            ]}
          />
          {isEdit && host?.has_key && selectedKeyId === null && (
            <p className="text-[10px] text-[var(--text-faint)]">{t("host.sshKeyKeepCurrentHint")}</p>
          )}
          {selectedKeyId === "__clear__" && (
            <p className="text-[10px] text-amber-400">{t("host.sshKeyClearHint")}</p>
          )}
        </div>
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
      <DrawerSection title={t("entidade.label")} open={openStepSection === "entidades"} onToggle={() => toggleStepSection("entidades")} active={formEntidades.length > 0}>
        <EntidadeList
          value={formEntidades}
          onChange={setFormEntidades}
          options={entidadeOptions}
          t={t}
        />
      </DrawerSection>
      <DrawerSection title={t("project.responsaveis")} open={openStepSection === "responsaveis"} onToggle={() => toggleStepSection("responsaveis")} active={formResponsaveis.length > 0}>
        <ResponsavelList
          value={formResponsaveis}
          onChange={setFormResponsaveis}
          contacts={contacts}
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
      <DrawerSection title="Grafana" open={openStepSection === "grafana"} onToggle={() => toggleStepSection("grafana")} active={!!form.grafana_dashboard_uid}>
        <Input
          label="Grafana dashboard UID"
          value={form.grafana_dashboard_uid as string}
          onChange={(e) => set("grafana_dashboard_uid", e.target.value)}
          placeholder="leave blank to use the default from Settings"
        />
        <p className="text-[11px] text-[var(--text-muted)] mt-1">
          Shown in the Metrics tab. The host&apos;s <code className="text-[var(--text-secondary)]">oficial_slug</code> is passed as dashboard variable <code className="text-[var(--text-secondary)]">var-host</code>.
        </p>
        {isEdit && host?.oficial_slug && (
          <HostDashboardProvisionButton
            slug={host.oficial_slug}
            onProvisioned={(uid) => set("grafana_dashboard_uid", uid)}
          />
        )}
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

// HostDashboardProvisionButton calls POST /api/hosts/{slug}/grafana/provision
// and populates the UID field with the returned deterministic UID on success.
// Only rendered for existing hosts (slug must be persisted).
function HostDashboardProvisionButton({ slug, onProvisioned }: { slug: string; onProvisioned: (uid: string) => void }) {
  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const grafanaEnabled = integrations?.grafana?.grafana_enabled === "true";
  const datasourceSet = !!integrations?.grafana?.grafana_datasource_uid;

  const mutation = useMutation({
    mutationFn: () => grafanaAPI.provisionHostDashboard(slug),
    onSuccess: (res) => {
      onProvisioned(res.uid);
    },
  });

  if (!grafanaEnabled) return null;

  return (
    <div className="mt-3 space-y-1">
      <button
        type="button"
        onClick={() => mutation.mutate()}
        disabled={mutation.isPending || !datasourceSet}
        className="text-[11px] text-[var(--accent)] hover:underline disabled:opacity-40 disabled:cursor-not-allowed disabled:no-underline"
      >
        {mutation.isPending ? "Provisioning…" : "Provision default dashboard in Grafana"}
      </button>
      {!datasourceSet && (
        <p className="text-[10px] text-amber-400">
          Set the Prometheus datasource UID in Settings → Integrations → Grafana first.
        </p>
      )}
      {mutation.isSuccess && !mutation.isPending && (
        <p className="text-[10px] text-emerald-400">{mutation.data?.message}</p>
      )}
      {mutation.isError && (
        <p className="text-[10px] text-red-400">
          {mutation.error instanceof Error ? mutation.error.message : "Provision failed"}
        </p>
      )}
    </div>
  );
}
