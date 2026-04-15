"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { servicesAPI, hostsAPI, dnsAPI, projectsAPI, enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import CheckboxList from "@/components/ui/CheckboxList";
import StepIndicator from "@/components/ui/StepIndicator";
import FormError from "@/components/ui/FormError";
import type { Service } from "@/lib/types";

interface ServiceFormProps {
  initial?: Service | null;
  onSuccess: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

export default function ServiceForm({ initial, onSuccess, onSubHeaderChange }: ServiceFormProps) {
  const { t } = useLocale();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState({
    nickname: initial?.nickname || "",
    description: initial?.description || "",
    service_type: initial?.service_type || "",
    service_subtype: initial?.service_subtype || "",
    technology_stack: initial?.technology_stack || "",
    deploy_approach: initial?.deploy_approach || "",
    orchestrator_tool: initial?.orchestrator_tool || "",
    environment: initial?.environment || "",
    port: initial?.port || "",
    version: initial?.version || "",
    project_id: initial?.project_id ?? null as number | null,
    orchestrator_managed: initial?.orchestrator_managed || false,
    is_directly_managed: initial?.is_directly_managed ?? true,
    is_responsible: initial?.is_responsible ?? true,
    developed_by: initial?.developed_by || "internal",
    is_external_dependency: initial?.is_external_dependency || false,
    external_provider: initial?.external_provider || "",
    external_url: initial?.external_url || "",
    external_contact: initial?.external_contact || "",
    repository_url: initial?.repository_url || "",
    gitlab_url: initial?.gitlab_url || "",
    documentation_url: initial?.documentation_url || "",
    host_ids: initial?.host_ids || [] as number[],
    dns_ids: initial?.dns_ids || [] as number[],
    depends_on_ids: initial?.depends_on_ids || [] as number[],
  });
  const [tags, setTags] = useState<string[]>(initial?.tags || []);
  const [error, setError] = useState("");

  const { data: hosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: dnsRecords = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list });
  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: serviceTypes = [] } = useQuery({ queryKey: ["enums", "service_type"], queryFn: () => enumsAPI.list("service_type") });
  const { data: serviceSubtypes = [] } = useQuery({ queryKey: ["enums", "service_subtype"], queryFn: () => enumsAPI.list("service_subtype") });
  const { data: techStacks = [] } = useQuery({ queryKey: ["enums", "technology_stack"], queryFn: () => enumsAPI.list("technology_stack") });
  const { data: deployApproaches = [] } = useQuery({ queryKey: ["enums", "deploy_approach"], queryFn: () => enumsAPI.list("deploy_approach") });
  const { data: orchestratorTools = [] } = useQuery({ queryKey: ["enums", "orchestrator_tool"], queryFn: () => enumsAPI.list("orchestrator_tool") });
  const { data: environments = [] } = useQuery({ queryKey: ["enums", "environment"], queryFn: () => enumsAPI.list("environment") });

  const mutation = useMutation({
    mutationFn: () => {
      const data = { ...form, tags };
      return initial ? servicesAPI.update(initial.id, data) : servicesAPI.create(data);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));
  const enumOpts = (arr: { value: string }[]) => arr.map((e) => ({ value: e.value, label: e.value }));

  const stepLabels = ["Identity", "Technical", "Deploy & Config", "Links & Tags"];
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    onSubHeaderChange?.(<StepIndicator steps={stepLabels} current={step} />);
  }, [step]);

  return (
    <div className="space-y-4">
      <FormError message={error} />

      {/* Step 1: Identity */}
      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          <Input label={t("service.nickname")} value={form.nickname} onChange={(e) => set("nickname", e.target.value)} required placeholder="Service name" />
          <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} placeholder="Brief description" />
          <Select label="Project" value={form.project_id?.toString() || ""} onChange={(e) => set("project_id", e.target.value ? parseInt(e.target.value) : null)} options={projects.map((p) => ({ value: p.id.toString(), label: p.name }))} />
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("service.serviceType")} value={form.service_type} onChange={(e) => set("service_type", e.target.value)} options={enumOpts(serviceTypes)} />
            <Select label={t("service.serviceSubtype")} value={form.service_subtype} onChange={(e) => set("service_subtype", e.target.value)} options={enumOpts(serviceSubtypes)} />
          </div>
          <Select label={t("service.technologyStack")} value={form.technology_stack} onChange={(e) => set("technology_stack", e.target.value)} options={enumOpts(techStacks)} />
          <Button type="button" className="w-full" disabled={!form.nickname.trim()} onClick={() => setStep(2)}>
            {t("host.nextStep")}
          </Button>
        </div>
      )}

      {/* Step 2: Technical / Ownership */}
      {step === 2 && (
        <div className="space-y-4 animate-fade-in">
          <Select label={t("service.developedBy")} value={form.developed_by} onChange={(e) => set("developed_by", e.target.value)} options={[{ value: "internal", label: t("service.internal") }, { value: "external", label: t("service.external") }]} />
          <div className="flex flex-wrap gap-4">
            <Checkbox label={t("service.isExternalDependency")} checked={form.is_external_dependency} onChange={(v) => set("is_external_dependency", v)} />
            {!form.is_external_dependency && (
              <>
                <Checkbox label={t("project.isDirectlyManaged")} checked={form.is_directly_managed} onChange={(v) => set("is_directly_managed", v)} />
                <Checkbox label={t("project.isResponsible")} checked={form.is_responsible} onChange={(v) => set("is_responsible", v)} />
              </>
            )}
          </div>
          {form.is_external_dependency && (
            <div className="space-y-3 p-3 rounded-[var(--radius-md)] border border-amber-500/20 bg-amber-500/5">
              <Input label={t("service.externalProvider")} value={form.external_provider} onChange={(e) => set("external_provider", e.target.value)} placeholder="AWS, Stripe, etc." />
              <Input label={t("service.externalContact")} value={form.external_contact} onChange={(e) => set("external_contact", e.target.value)} />
              <Input label={t("service.externalUrl")} value={form.external_url} onChange={(e) => set("external_url", e.target.value)} type="url" />
            </div>
          )}
          <Input label="Repository URL" value={form.repository_url} onChange={(e) => set("repository_url", e.target.value)} type="url" placeholder="https://gitlab.com/..." />
          <Input label={t("project.documentationUrl")} value={form.documentation_url} onChange={(e) => set("documentation_url", e.target.value)} type="url" placeholder="https://docs..." />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="button" className="flex-1" onClick={() => setStep(3)}>{t("host.nextStep")}</Button>
          </div>
        </div>
      )}

      {/* Step 3: Deploy & Config */}
      {step === 3 && (
        <div className="space-y-4 animate-fade-in">
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("service.deployApproach")} value={form.deploy_approach} onChange={(e) => set("deploy_approach", e.target.value)} options={enumOpts(deployApproaches)} />
            <Select label={t("service.orchestratorTool")} value={form.orchestrator_tool} onChange={(e) => set("orchestrator_tool", e.target.value)} options={enumOpts(orchestratorTools)} />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Select label={t("service.environment")} value={form.environment} onChange={(e) => set("environment", e.target.value)} options={enumOpts(environments)} />
            <Input label={t("service.port")} value={form.port} onChange={(e) => set("port", e.target.value)} placeholder="8080" />
          </div>
          <Input label={t("service.version")} value={form.version} onChange={(e) => set("version", e.target.value)} placeholder="v1.2.3" />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(2)}>{t("common.back")}</Button>
            <Button type="button" className="flex-1" onClick={() => setStep(4)}>{t("host.nextStep")}</Button>
          </div>
        </div>
      )}

      {/* Step 4: Links & Tags */}
      {step === 4 && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="service" />
          <CheckboxList label="Hosts" items={hosts.map((h) => ({ id: h.id, name: h.nickname }))} selected={form.host_ids} onChange={(ids) => set("host_ids", ids)} />
          <CheckboxList label="DNS" items={dnsRecords.map((d) => ({ id: d.id, name: d.domain }))} selected={form.dns_ids} onChange={(ids) => set("dns_ids", ids)} />
          <CheckboxList label={t("service.dependencies")} items={allServices.map((s) => ({ id: s.id, name: s.nickname }))} selected={form.depends_on_ids} onChange={(ids) => set("depends_on_ids", ids)} />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(3)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{t("common.create")}</Button>
          </div>
        </form>
      )}
    </div>
  );
}
