"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { servicesAPI, projectsAPI, enumsAPI, contactsAPI } from "@/lib/api";
import { SERVICE_KINDS } from "@/lib/serviceDisplay";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
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
import GrafanaUidField from "@/components/forms/GrafanaUidField";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import type { Service, AssetGrants, AssetGrantsInput, EntityResponsavel } from "@/lib/types";

const URL_RE = /^https?:\/\/\S+$/i;

interface ServiceFormProps {
  initial?: Service | null;
  initialGrants?: AssetGrants | null;
  /** The detail endpoint returns these beside the service, not on it. */
  initialTags?: string[];
  initialHostIds?: number[];
  initialDnsIds?: number[];
  initialDependsOnIds?: number[];
  initialResponsaveis?: EntityResponsavel[];
  onSuccess: () => void;
  onClose?: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
  onFooterChange?: (footer: React.ReactNode) => void;
}

export default function ServiceForm({
  initial, initialGrants, initialTags, initialHostIds, initialDnsIds, initialDependsOnIds, initialResponsaveis,
  onSuccess, onClose, onSubHeaderChange, onFooterChange,
}: ServiceFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!initial;
  const [grants, setGrants] = useState<AssetGrantsInput>(initialGrants ?? defaultGrants(user));
  // Only operator-editable fields: scan state and service_type are kept by
  // the server (PUT merges onto the stored row).
  const [form, setForm] = useState({
    nickname: initial?.nickname || "",
    description: initial?.description || "",
    service_kind: initial?.service_kind || "",
    service_subtype: initial?.service_subtype || "",
    technology_stack: initial?.technology_stack || "",
    deploy_approach: initial?.deploy_approach || "",
    orchestrator_tool: initial?.orchestrator_tool || "",
    environment: initial?.environment || "",
    port: initial?.port || "",
    version: initial?.version || "",
    project_id: initial?.project_id ?? (null as number | null),
    is_directly_managed: initial?.is_directly_managed ?? true,
    is_responsible: initial?.is_responsible ?? true,
    developed_by: initial?.developed_by || "internal",
    is_external_dependency: initial?.is_external_dependency || false,
    external_provider: initial?.external_provider || "",
    external_url: initial?.external_url || "",
    external_contact: initial?.external_contact || "",
    repository_url: initial?.repository_url || "",
    documentation_url: initial?.documentation_url || "",
    grafana_dashboard_uid: initial?.grafana_dashboard_uid || "",
  });
  const [hostIds, setHostIds] = useState<number[]>(initialHostIds ?? []);
  const [dnsIds, setDnsIds] = useState<number[]>(initialDnsIds ?? []);
  const [dependsOnIds, setDependsOnIds] = useState<number[]>(initialDependsOnIds ?? []);
  const [tags, setTags] = useState<string[]>(initialTags ?? []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initialResponsaveis ?? []);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const enumOf = (category: string) => ({ queryKey: ["enums", category], queryFn: () => enumsAPI.list(category) });
  const { data: serviceSubtypes = [] } = useQuery(enumOf("service_subtype"));
  const { data: techStacks = [] } = useQuery(enumOf("technology_stack"));
  const { data: deployApproaches = [] } = useQuery(enumOf("deploy_approach"));
  const { data: orchestratorTools = [] } = useQuery(enumOf("orchestrator_tool"));
  const { data: environments = [] } = useQuery(enumOf("environment"));
  const relationOptions = useRelationOptions(["hosts", "dns", "services"]);
  const dependencyOptions = relationOptions.services.filter((o) => o.id !== initial?.id);

  const set = (key: keyof typeof form, value: unknown) => setForm((f) => ({ ...f, [key]: value }));
  const enumOpts = (arr: { value: string }[], current: string) => {
    const opts = arr.map((e) => ({ value: e.value, label: e.value }));
    // A scan-written value that isn't an enum option still shows as selected.
    return current && !opts.some((o) => o.value === current) ? [{ value: current, label: current }, ...opts] : opts;
  };

  const mutation = useMutation({
    mutationFn: () => {
      const data = {
        ...form,
        nickname: form.nickname.trim(),
        tags,
        host_ids: hostIds,
        dns_ids: dnsIds,
        depends_on_ids: dependsOnIds,
        responsaveis: responsaveis.filter((r) => r.name),
        ...grants,
      };
      return initial ? servicesAPI.update(initial.id, data) : servicesAPI.create(data);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const errors: Record<string, string> = {};
  if (!form.nickname.trim()) errors.nickname = t("form.required");
  for (const k of ["repository_url", "documentation_url", "external_url"] as const) {
    if (form[k] && form[k] !== (initial?.[k] ?? "") && !URL_RE.test(form[k].trim())) errors[k] = t("form.urlInvalid");
  }
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  const sections = [
    { id: "svc-identity", label: t("form.section.identity") },
    { id: "svc-operation", label: t("form.section.operation") },
    { id: "svc-origin", label: t("form.section.origin") },
    { id: "svc-owners", label: t("form.section.owners") },
    { id: "svc-links", label: t("form.section.links") },
    { id: "svc-notes", label: t("form.section.notes") },
  ];

  return (
    <EntityFormShell
      id="service-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("form.createService")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="svc-identity" title={t("form.section.identity")}>
        <div className="sm:col-span-2">
          <Input label={t("service.nickname")} value={form.nickname} onChange={(e) => set("nickname", e.target.value)} required
            placeholder={t("service.namePlaceholder")} error={err("nickname")} aria-invalid={!!err("nickname")} />
        </div>
        <Select label={t("service.category")} value={form.service_kind} onChange={(e) => set("service_kind", e.target.value)}
          options={SERVICE_KINDS.map((k) => ({ value: k, label: t(`service.kind.${k}`) }))} />
        <Select label={t("form.software")} hint={t("form.softwareHint")} value={form.service_subtype} onChange={(e) => set("service_subtype", e.target.value)}
          options={enumOpts(serviceSubtypes, form.service_subtype)} />
        <div className="sm:col-span-2">
          <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} placeholder={t("service.descriptionPlaceholder")} />
        </div>
        <div className="sm:col-span-2">
          <Select label={t("service.projectLabel")} value={form.project_id?.toString() || ""} onChange={(e) => set("project_id", e.target.value ? parseInt(e.target.value) : null)}
            options={projects.map((p) => ({ value: p.id.toString(), label: p.name }))} />
        </div>
      </FormSection>

      <FormSection id="svc-operation" title={t("form.section.operation")}>
        <Select label={t("service.technologyStack")} value={form.technology_stack} onChange={(e) => set("technology_stack", e.target.value)} options={enumOpts(techStacks, form.technology_stack)} />
        <Select label={t("service.environment")} value={form.environment} onChange={(e) => set("environment", e.target.value)} options={enumOpts(environments, form.environment)} />
        <Input label={t("service.port")} value={form.port} onChange={(e) => set("port", e.target.value)} placeholder="8080" className="font-mono" inputMode="numeric" />
        <Input label={t("service.version")} value={form.version} onChange={(e) => set("version", e.target.value)} placeholder="v1.2.3" className="font-mono" />
        <Select label={t("service.deployApproach")} value={form.deploy_approach} onChange={(e) => set("deploy_approach", e.target.value)} options={enumOpts(deployApproaches, form.deploy_approach)} />
        <Select label={t("service.orchestratorTool")} value={form.orchestrator_tool} onChange={(e) => set("orchestrator_tool", e.target.value)} options={enumOpts(orchestratorTools, form.orchestrator_tool)} />
        <div className="sm:col-span-2">
          <GrafanaUidField kind="service" target={initial?.id} value={form.grafana_dashboard_uid} onChange={(v) => set("grafana_dashboard_uid", v)} />
        </div>
      </FormSection>

      <FormSection id="svc-origin" title={t("form.section.origin")} description={t("form.section.originHint")}>
        <Select label={t("service.developedBy")} value={form.developed_by} onChange={(e) => set("developed_by", e.target.value)}
          options={[{ value: "internal", label: t("service.internal") }, { value: "external", label: t("service.external") }]} />
        <FormField label={t("form.management")}>
          <div className="flex flex-col gap-2 pt-1">
            <Checkbox label={t("service.isExternalDependency")} checked={form.is_external_dependency} onChange={(v) => set("is_external_dependency", v)} />
            {!form.is_external_dependency && (
              <>
                <Checkbox label={t("project.isDirectlyManaged")} checked={form.is_directly_managed} onChange={(v) => set("is_directly_managed", v)} />
                <Checkbox label={t("project.isResponsible")} checked={form.is_responsible} onChange={(v) => set("is_responsible", v)} />
              </>
            )}
          </div>
        </FormField>
        {form.is_external_dependency && (
          <>
            <Input label={t("service.externalProvider")} value={form.external_provider} onChange={(e) => set("external_provider", e.target.value)} placeholder={t("service.externalProviderPlaceholder")} />
            <Input label={t("service.externalContact")} value={form.external_contact} onChange={(e) => set("external_contact", e.target.value)} />
            <div className="sm:col-span-2">
              <Input label={t("service.externalUrl")} value={form.external_url} onChange={(e) => set("external_url", e.target.value)} type="url"
                error={err("external_url")} aria-invalid={!!err("external_url")} />
            </div>
          </>
        )}
        <Input label={t("service.repositoryUrlLabel")} value={form.repository_url} onChange={(e) => set("repository_url", e.target.value)} type="url"
          placeholder="https://gitlab.com/..." error={err("repository_url")} aria-invalid={!!err("repository_url")} />
        <Input label={t("project.documentationUrl")} value={form.documentation_url} onChange={(e) => set("documentation_url", e.target.value)} type="url"
          placeholder="https://docs..." error={err("documentation_url")} aria-invalid={!!err("documentation_url")} />
      </FormSection>

      <FormSection id="svc-owners" title={t("form.section.owners")} description={t("form.section.ownersHint")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
        <ResponsavelList value={responsaveis} onChange={setResponsaveis} contacts={contacts} t={t} />
      </FormSection>

      <FormSection id="svc-links" title={t("form.section.links")} description={t("form.section.linksHint")} stack>
        <RelationPicker label={t("nav.hosts")} options={relationOptions.hosts} selected={hostIds} onChange={setHostIds} />
        <RelationPicker label={t("topology.dnsRecords")} options={relationOptions.dns} selected={dnsIds} onChange={setDnsIds} />
        <RelationPicker label={t("service.dependsOn")} options={dependencyOptions} selected={dependsOnIds} onChange={setDependsOnIds} />
      </FormSection>

      <FormSection id="svc-notes" title={t("form.section.notes")} stack>
        <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="service" />
      </FormSection>
    </EntityFormShell>
  );
}
