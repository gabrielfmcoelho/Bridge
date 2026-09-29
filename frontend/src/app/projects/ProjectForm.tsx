"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { projectsAPI, enumsAPI, contactsAPI, integrationsAPI, glpiAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useSituacao } from "@/hooks/useSituacao";
import { useDefaultSituacao } from "@/hooks/useDefaultSituacao";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import FormField from "@/components/ui/FormField";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import GitLabLinksEditor from "./[id]/_components/GitLabLinksEditor";
import type { Project, EntityResponsavel, AssetGrants, AssetGrantsInput } from "@/lib/types";

const URL_RE = /^https?:\/\/\S+$/i;

interface ProjectFormProps {
  initial?: Project | null;
  initialGrants?: AssetGrants | null;
  /** The detail endpoint returns these beside the project, not on it. */
  initialTags?: string[];
  initialResponsaveis?: EntityResponsavel[];
  initialServiceIds?: number[];
  /** Linked to the project itself, not through one of its services. */
  initialHostIds?: number[];
  initialDnsIds?: number[];
  onSuccess: () => void;
  onClose?: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
  onFooterChange?: (footer: React.ReactNode) => void;
}

/**
 * Create/edit a project: the sectioned EntityFormShell form shared with hosts,
 * DNS and serviços. Only the form's own fields are sent — the API merges them
 * onto the stored project, so the legacy responsável text, gitlab_url and
 * anything else not shown here are kept.
 */
export default function ProjectForm({
  initial, initialGrants, initialTags, initialResponsaveis, initialServiceIds, initialHostIds, initialDnsIds, onSuccess, onClose, onSubHeaderChange, onFooterChange,
}: ProjectFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!initial;
  const { valueOf: situacaoValueOf } = useSituacao();
  const [grants, setGrants] = useState<AssetGrantsInput>(initialGrants ?? defaultGrants(user));
  const [form, setForm] = useState({
    name: initial?.name || "",
    description: initial?.description || "",
    situacao: initial?.situacao || situacaoValueOf("active"),
    setor_responsavel: initial?.setor_responsavel || "",
    tem_empresa_externa_responsavel: initial?.tem_empresa_externa_responsavel || false,
    contato_empresa_responsavel: initial?.contato_empresa_responsavel || "",
    is_directly_managed: initial?.is_directly_managed ?? true,
    is_responsible: initial?.is_responsible ?? true,
    documentation_url: initial?.documentation_url || "",
    outline_collection_id: initial?.outline_collection_id || "",
    glpi_token_id: initial?.glpi_token_id ?? null,
    glpi_entity_id: initial?.glpi_entity_id ?? 0,
    glpi_category_id: initial?.glpi_category_id ?? 0,
  });
  const [tags, setTags] = useState<string[]>(initialTags ?? initial?.tags ?? []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initialResponsaveis ?? []);
  const [serviceIds, setServiceIds] = useState<number[]>(initialServiceIds ?? []);
  const [hostIds, setHostIds] = useState<number[]>(initialHostIds ?? []);
  const [dnsIds, setDnsIds] = useState<number[]>(initialDnsIds ?? []);
  const relationOptions = useRelationOptions(["services", "hosts", "dns"]);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  // Integration settings decide which fields show; non-admins can't read them,
  // and then GitLab falls back to gitlab.com and Outline/GLPI stay hidden.
  const { data: integrations } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get, retry: false, staleTime: 60_000 });
  const gitlabBaseURL = integrations?.gitlab?.auth_gitlab_base_url || "https://gitlab.com";
  const outlineEnabled = integrations?.outline?.outline_enabled === "true";
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";
  const { data: glpiProfiles = [] } = useQuery({ queryKey: ["glpi-profiles"], queryFn: glpiAPI.listProfiles, enabled: glpiEnabled, retry: false });

  const set = (key: keyof typeof form, value: unknown) => setForm((f) => ({ ...f, [key]: value }));
  useDefaultSituacao(form.situacao, (v) => set("situacao", v), !isEdit);

  const mutation = useMutation({
    mutationFn: () => {
      const payload = {
        ...form,
        name: form.name.trim(),
        tags,
        responsaveis: responsaveis.map((r) => ({ contact_id: r.contact_id, is_main: r.is_main })),
        service_ids: serviceIds,
        host_ids: hostIds,
        dns_ids: dnsIds,
        ...grants,
      };
      return initial ? projectsAPI.update(initial.id, payload) : projectsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const errors: Record<string, string> = {};
  if (!form.name.trim()) errors.name = t("form.required");
  if (form.documentation_url && form.documentation_url !== (initial?.documentation_url ?? "") && !URL_RE.test(form.documentation_url.trim())) {
    errors.documentation_url = t("form.urlInvalid");
  }
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  const sections = [
    { id: "prj-identity", label: t("form.section.identity") },
    { id: "prj-operation", label: t("form.section.operation") },
    { id: "prj-integrations", label: t("form.section.integrations") },
    { id: "prj-owners", label: t("form.section.owners") },
    { id: "prj-links", label: t("form.section.links") },
    { id: "prj-notes", label: t("form.section.notes") },
  ];

  return (
    <EntityFormShell
      id="project-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("form.createProject")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="prj-identity" title={t("form.section.identity")}>
        <div className="sm:col-span-2">
          <Input label={t("project.name")} value={form.name} onChange={(e) => set("name", e.target.value)} required
            placeholder={t("project.namePlaceholder")} error={err("name")} aria-invalid={!!err("name")} />
        </div>
        <div className="sm:col-span-2">
          <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} placeholder={t("project.descriptionPlaceholder")} />
        </div>
        <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((o) => ({ value: o.value, label: o.value }))} />
        <Input label={t("project.setorResponsavel")} value={form.setor_responsavel} onChange={(e) => set("setor_responsavel", e.target.value)} placeholder={t("project.setorResponsavelPlaceholder")} />
      </FormSection>

      <FormSection id="prj-operation" title={t("form.section.operation")} description={t("form.section.originHint")}>
        <FormField label={t("form.management")}>
          <div className="flex flex-col gap-2 pt-1">
            <Checkbox label={t("project.isDirectlyManaged")} checked={form.is_directly_managed} onChange={(v) => set("is_directly_managed", v)} />
            <Checkbox label={t("project.isResponsible")} checked={form.is_responsible} onChange={(v) => set("is_responsible", v)} />
            <Checkbox label={t("project.temEmpresaExterna")} checked={form.tem_empresa_externa_responsavel} onChange={(v) => set("tem_empresa_externa_responsavel", v)} />
          </div>
        </FormField>
        {form.tem_empresa_externa_responsavel ? (
          <Input label={t("project.externalCompanyContactLabel")} value={form.contato_empresa_responsavel} onChange={(e) => set("contato_empresa_responsavel", e.target.value)} placeholder={t("project.contactInfoPlaceholder")} />
        ) : <div />}
      </FormSection>

      <FormSection id="prj-integrations" title={t("form.section.integrations")} description={t("form.section.integrationsHint")} stack>
        <FormField label="GitLab" hint={isEdit ? undefined : t("project.gitlabLinkHint")}>
          {isEdit && initial ? <GitLabLinksEditor projectId={initial.id} canEdit gitlabBaseURL={gitlabBaseURL} /> : null}
        </FormField>
        {outlineEnabled && (
          <Input label={t("project.outlineCollectionIdLabel")} value={form.outline_collection_id} onChange={(e) => set("outline_collection_id", e.target.value)}
            placeholder={t("project.outlineCollectionIdPlaceholder")} hint={t("project.outlineCollectionIdHint")} />
        )}
        {glpiEnabled && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="sm:col-span-2">
              <Select label={t("project.glpiTokenProfileLabel")} value={form.glpi_token_id == null ? "" : String(form.glpi_token_id)}
                onChange={(e) => set("glpi_token_id", e.target.value ? parseInt(e.target.value, 10) : null)}
                options={glpiProfiles.map((p) => ({ value: String(p.id), label: p.description ? `${p.name} — ${p.description}` : p.name }))} />
            </div>
            <Input label={t("project.glpiEntityIdLabel")} type="number" value={String(form.glpi_entity_id ?? 0)} onChange={(e) => set("glpi_entity_id", parseInt(e.target.value || "0", 10))} />
            <Input label={t("project.glpiCategoryIdLabel")} type="number" value={String(form.glpi_category_id ?? 0)} onChange={(e) => set("glpi_category_id", parseInt(e.target.value || "0", 10))}
              hint={t("project.glpiScopeHint")} />
          </div>
        )}
      </FormSection>

      <FormSection id="prj-owners" title={t("form.section.owners")} description={t("form.section.ownersHint")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
        <ResponsavelList value={responsaveis} onChange={setResponsaveis} contacts={contacts} t={t} />
      </FormSection>

      <FormSection id="prj-links" title={t("form.section.links")} description={t("project.linksHint")} stack>
        <RelationPicker label={t("nav.services")} options={relationOptions.services} selected={serviceIds} onChange={setServiceIds} />
        <RelationPicker label={t("project.directHosts")} options={relationOptions.hosts} selected={hostIds} onChange={setHostIds} />
        <RelationPicker label={t("project.directDns")} options={relationOptions.dns} selected={dnsIds} onChange={setDnsIds} />
      </FormSection>

      <FormSection id="prj-notes" title={t("form.section.notes")} stack>
        <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="project" />
        <Input label={t("project.documentationUrl")} value={form.documentation_url} onChange={(e) => set("documentation_url", e.target.value)} type="url"
          placeholder="https://docs..." error={err("documentation_url")} aria-invalid={!!err("documentation_url")} />
      </FormSection>
    </EntityFormShell>
  );
}
