"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { projectsAPI, enumsAPI, contactsAPI, integrationsAPI, glpiAPI } from "@/lib/api";
import { contactsToOptions } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import { useMultiStepFormEffects } from "@/hooks/useMultiStepForm";
import Button from "@/components/ui/Button";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import FormError from "@/components/ui/FormError";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import GitLabLinksEditor from "./[id]/_components/GitLabLinksEditor";
import type { Project, EntityResponsavel } from "@/lib/types";

interface ProjectFormProps {
  initial?: Project | null;
  onSuccess: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

export default function ProjectForm({ initial, onSuccess, onSubHeaderChange }: ProjectFormProps) {
  const { t } = useLocale();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState({
    name: initial?.name || "",
    description: initial?.description || "",
    situacao: initial?.situacao || "active",
    setor_responsavel: initial?.setor_responsavel || "",
    responsavel: initial?.responsavel || "",
    tem_empresa_externa_responsavel: initial?.tem_empresa_externa_responsavel || false,
    contato_empresa_responsavel: initial?.contato_empresa_responsavel || "",
    is_directly_managed: initial?.is_directly_managed ?? true,
    is_responsible: initial?.is_responsible ?? true,
    gitlab_url: initial?.gitlab_url || "",
    documentation_url: initial?.documentation_url || "",
    outline_collection_id: initial?.outline_collection_id || "",
    glpi_token_id: initial?.glpi_token_id ?? null,
    glpi_entity_id: initial?.glpi_entity_id ?? 0,
    glpi_category_id: initial?.glpi_category_id ?? 0,
  });
  const [tags, setTags] = useState<string[]>(initial?.tags || []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>([]);
  const [error, setError] = useState("");

  // When editing, load the project's existing responsaveis (separate from the
  // Project model itself — they live on the project_responsaveis junction).
  const { data: projectDetails } = useQuery({
    queryKey: ["project", initial?.id, "for-edit"],
    queryFn: () => projectsAPI.get(initial!.id),
    enabled: !!initial?.id,
  });
  useEffect(() => {
    if (projectDetails?.responsaveis) {
      setResponsaveis(projectDetails.responsaveis);
    }
  }, [projectDetails]);

  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];

  // Best-effort read of the configured GitLab base URL for pretty link labels.
  // Non-admins can't read /api/settings/integrations; that's fine — we fall back to gitlab.com.
  const { data: integrations } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    enabled: !!initial?.id,
    retry: false,
  });
  const gitlabBaseURL = integrations?.gitlab?.auth_gitlab_base_url || "https://gitlab.com";
  const outlineEnabled = integrations?.outline?.outline_enabled === "true";
  const glpiEnabled = integrations?.glpi?.glpi_enabled === "true";
  const { data: glpiProfiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: glpiEnabled,
    retry: false,
  });

  const mutation = useMutation({
    mutationFn: async () => {
      const payload = {
        ...form,
        tags,
        responsaveis: responsaveis.map((r) => ({ contact_id: r.contact_id, is_main: r.is_main })),
      };
      return initial ? projectsAPI.update(initial.id, payload) : projectsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  useMultiStepFormEffects({
    step,
    setStep,
    totalSteps: 2,
    stepLabels: [t("common.basicInfo"), t("project.responsaveis") + " & " + t("common.tags")],
    onSubmit: () => mutation.mutate(),
    canProceed: step === 1 ? !!form.name.trim() : true,
    isPending: mutation.isPending,
    submitLabel: initial ? t("common.save") : t("common.create"),
    t,
    onSubHeaderChange,
  });

  return (
    <div className="space-y-4">
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          <Input label={t("project.name")} value={form.name} onChange={(e) => set("name", e.target.value)} required placeholder="Project name" />
          <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} placeholder="Brief description" />
          <Select
            label={t("host.situacao")}
            value={form.situacao}
            onChange={(e) => set("situacao", e.target.value)}
            options={situacoes.map((e) => ({ value: e.value, label: e.value }))}
          />
          <Input label={t("project.setorResponsavel")} value={form.setor_responsavel} onChange={(e) => set("setor_responsavel", e.target.value)} placeholder="e.g. TI" />
          <div className="flex flex-wrap gap-4">
            <Checkbox label={t("project.isDirectlyManaged")} checked={form.is_directly_managed} onChange={(v) => set("is_directly_managed", v)} />
            <Checkbox label={t("project.isResponsible")} checked={form.is_responsible} onChange={(v) => set("is_responsible", v)} />
            <Checkbox label={t("project.temEmpresaExterna")} checked={form.tem_empresa_externa_responsavel} onChange={(v) => set("tem_empresa_externa_responsavel", v)} />
          </div>
          {form.tem_empresa_externa_responsavel && (
            <div className="p-3 rounded-[var(--radius-md)] border border-amber-500/20 bg-amber-500/5">
              <Input label={t("project.temEmpresaExterna") + " - Contato"} value={form.contato_empresa_responsavel} onChange={(e) => set("contato_empresa_responsavel", e.target.value)} placeholder="Contact info" />
            </div>
          )}
          <Input label={t("project.documentationUrl")} value={form.documentation_url} onChange={(e) => set("documentation_url", e.target.value)} type="url" placeholder="https://docs..." />
          <Button type="button" className="w-full" disabled={!form.name.trim()} onClick={() => setStep(2)}>
            {t("host.nextStep")}
          </Button>
        </div>
      )}

      {step === 2 && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Select
            label={t("project.responsavelOwner") || t("project.responsaveis")}
            value={form.responsavel}
            onChange={(e) => set("responsavel", e.target.value)}
            options={contactsToOptions(contacts)}
          />
          <div className="space-y-2">
            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">{t("project.responsaveis")}</label>
            <ResponsavelList
              value={responsaveis}
              onChange={setResponsaveis}
              contacts={contacts}
              t={t}
            />
          </div>
          <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="project" />

          <section className="pt-2">
            <h3 className="text-xs font-semibold text-[var(--text-secondary)] tracking-wide uppercase mb-2">
              Vínculos
            </h3>
            {initial?.id ? (
              <GitLabLinksEditor projectId={initial.id} canEdit={true} gitlabBaseURL={gitlabBaseURL} />
            ) : (
              <p className="text-[11px] text-[var(--text-muted)] italic">
                GitLab sources can be linked after the project is created.
              </p>
            )}
            {outlineEnabled && (
              <div className="mt-3 space-y-1">
                <Input
                  label="Outline collection ID"
                  value={form.outline_collection_id}
                  onChange={(e) => set("outline_collection_id", e.target.value)}
                  placeholder="e.g. d3b71c7e-8f7e-4f9d-b5a2-xxxxxxxxxxxx"
                />
                <p className="text-[11px] text-[var(--text-muted)]">
                  Paste the UUID of the Outline collection that holds this project&apos;s documentation. The Wiki tab will list recent docs from it.
                </p>
              </div>
            )}
            {glpiEnabled && (
              <div className="mt-3 space-y-2">
                <div>
                  <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">GLPI token profile</label>
                  <select
                    value={form.glpi_token_id == null ? "" : String(form.glpi_token_id)}
                    onChange={(e) => set("glpi_token_id", e.target.value ? parseInt(e.target.value, 10) : null)}
                    className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm focus:outline-none focus:border-[var(--accent)]"
                  >
                    <option value="">(none)</option>
                    {(glpiProfiles ?? []).map((p) => (
                      <option key={p.id} value={p.id}>{p.name}{p.description ? ` — ${p.description}` : ""}</option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <Input
                    label="Entity ID (0 = profile default)"
                    type="number"
                    value={String(form.glpi_entity_id ?? 0)}
                    onChange={(e) => set("glpi_entity_id", parseInt(e.target.value || "0", 10))}
                  />
                  <Input
                    label="Category ID (optional filter)"
                    type="number"
                    value={String(form.glpi_category_id ?? 0)}
                    onChange={(e) => set("glpi_category_id", parseInt(e.target.value || "0", 10))}
                  />
                </div>
                <p className="text-[11px] text-[var(--text-muted)]">
                  Picks which GLPI account sshcm uses for this project&apos;s Chamados tab. Entity/category scope the listing.
                </p>
              </div>
            )}
          </section>

          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{initial ? t("common.save") : t("common.create")}</Button>
          </div>
        </form>
      )}
    </div>
  );
}
