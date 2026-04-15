"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { projectsAPI, enumsAPI, contactsAPI } from "@/lib/api";
import { contactsToOptions } from "@/lib/utils";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import StepIndicator from "@/components/ui/StepIndicator";
import FormError from "@/components/ui/FormError";
import type { Project } from "@/lib/types";

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
  });
  const [tags, setTags] = useState<string[]>(initial?.tags || []);
  const [responsaveis, setResponsaveis] = useState<{ nome: string; contato: string }[]>([]);
  const [error, setError] = useState("");

  const { data: situacoes = [] } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
  });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];

  const mutation = useMutation({
    mutationFn: async () => {
      for (const r of responsaveis) {
        if (r.nome && r.contato) await contactsAPI.create({ name: r.nome, phone: r.contato }).catch(() => {});
      }
      if (form.responsavel) await contactsAPI.create({ name: form.responsavel, phone: "" }).catch(() => {});
      // Convert legacy {nome, contato} to EntityResponsavel format for the new API
      const convertedResponsaveis = responsaveis
        .filter((r) => r.nome)
        .map((r, i) => ({
          is_main: i === 0,
          is_externo: false,
          name: r.nome,
          phone: r.contato,
          role: "",
          entity: "",
        }));
      const payload = { ...form, tags, responsaveis: convertedResponsaveis };
      return initial ? projectsAPI.update(initial.id, payload) : projectsAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  const stepLabels = [t("common.basicInfo"), t("project.responsaveis") + " & " + t("common.tags")];
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    onSubHeaderChange?.(<StepIndicator steps={stepLabels} current={step} />);
  }, [step]);

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
          <Input label={t("project.gitlabUrl")} value={form.gitlab_url} onChange={(e) => set("gitlab_url", e.target.value)} type="url" placeholder="https://gitlab.com/..." />
          <Input label={t("project.documentationUrl")} value={form.documentation_url} onChange={(e) => set("documentation_url", e.target.value)} type="url" placeholder="https://docs..." />
          <Button type="button" className="w-full" disabled={!form.name.trim()} onClick={() => setStep(2)}>
            {t("host.nextStep")}
          </Button>
        </div>
      )}

      {step === 2 && (
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4 animate-fade-in">
          <Select
            label={t("project.responsaveis")}
            value={form.responsavel}
            onChange={(e) => set("responsavel", e.target.value)}
            options={contactsToOptions(contacts)}
          />
          <div className="space-y-2">
            <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">{t("project.responsaveis")}</label>
            {responsaveis.map((r, i) => (
              <div key={i} className="flex gap-2">
                <Select value={r.nome} onChange={(e) => { const u = [...responsaveis]; u[i] = { ...u[i], nome: e.target.value }; setResponsaveis(u); }} options={contactsToOptions(contacts)} />
                <Input value={r.contato} onChange={(e) => { const u = [...responsaveis]; u[i] = { ...u[i], contato: e.target.value }; setResponsaveis(u); }} placeholder="Contact" />
                <Button type="button" variant="danger" size="sm" onClick={() => setResponsaveis(responsaveis.filter((_, j) => j !== i))}>&times;</Button>
              </div>
            ))}
            <Button type="button" variant="ghost" size="sm" onClick={() => setResponsaveis([...responsaveis, { nome: "", contato: "" }])}>+ {t("common.add")}</Button>
          </div>
          <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="project" />
          <div className="flex gap-2 pt-2">
            <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
            <Button type="submit" className="flex-1" loading={mutation.isPending}>{initial ? t("common.save") : t("common.create")}</Button>
          </div>
        </form>
      )}
    </div>
  );
}
