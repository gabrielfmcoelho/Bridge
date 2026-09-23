"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { dnsAPI, hostsAPI, servicesAPI, projectsAPI, enumsAPI, contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useMultiStepFormEffects } from "@/hooks/useMultiStepForm";
import Button from "@/components/ui/Button";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import CheckboxList from "@/components/ui/CheckboxList";
import FormError from "@/components/ui/FormError";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import { useAuth } from "@/contexts/AuthContext";
import type { DNSRecord, EntityResponsavel, AssetGrants, AssetGrantsInput } from "@/lib/types";

interface DnsFormProps {
  initial?: DNSRecord | null;
  initialTags?: string[];
  initialHostIds?: number[];
  initialServiceIds?: number[];
  initialProjectIds?: number[];
  initialResponsaveis?: EntityResponsavel[];
  initialGrants?: AssetGrants | null;
  onSuccess: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

export default function DnsForm({
  initial,
  initialTags,
  initialHostIds,
  initialServiceIds,
  initialProjectIds,
  initialResponsaveis,
  initialGrants,
  onSuccess,
  onFooterChange,
  onSubHeaderChange,
}: DnsFormProps) {
  const { t } = useLocale();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState({
    domain: initial?.domain || "",
    has_https: initial?.has_https || false,
    situacao: initial?.situacao || "active",
    observacoes: initial?.observacoes || "",
    host_ids: initialHostIds || initial?.host_ids || ([] as number[]),
    service_ids: initialServiceIds || ([] as number[]),
    project_ids: initialProjectIds || ([] as number[]),
  });
  const [tags, setTags] = useState<string[]>(initialTags || initial?.tags || []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initialResponsaveis || []);
  const { user } = useAuth();
  const [grants, setGrants] = useState<AssetGrantsInput>(initialGrants ?? defaultGrants(user));
  const [error, setError] = useState("");

  const { data: hosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: services = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });
  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];

  const mutation = useMutation({
    mutationFn: () => {
      const data = {
        ...form,
        tags,
        host_ids: form.host_ids.length ? form.host_ids : undefined,
        responsaveis: responsaveis.filter((r) => r.name),
        ...grants,
      };
      return initial ? dnsAPI.update(initial.id, data) : dnsAPI.create(data);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  useMultiStepFormEffects({
    step,
    setStep,
    totalSteps: 3,
    stepLabels: [t("dns.info"), t("host.responsaveis"), t("dns.stepLinksTags")],
    onSubmit: () => mutation.mutate(),
    canProceed: step === 1 ? !!form.domain.trim() : true,
    isPending: mutation.isPending,
    submitLabel: initial ? t("common.save") : t("common.create"),
    t,
    onFooterChange,
    onSubHeaderChange,
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));


  return (
    <div className="space-y-4">
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          <Input label={t("dns.domain")} value={form.domain} onChange={(e) => set("domain", e.target.value)} required placeholder={t("dns.domainPlaceholder")} />
          <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((e) => ({ value: e.value, label: e.value }))} />
          <Checkbox label={t("dns.hasHttps")} checked={form.has_https} onChange={(v) => set("has_https", v)} />
          <Input label={t("common.observacoes")} value={form.observacoes} onChange={(e) => set("observacoes", e.target.value)} placeholder={t("dns.notesPlaceholder")} />
        </div>
      )}

      {step === 2 && (
        <div className="space-y-4 animate-fade-in">
          <ResponsavelList
            value={responsaveis}
            onChange={setResponsaveis}
            contacts={contacts}
            t={t}
          />
        </div>
      )}

      {step === 3 && (
        <div className="space-y-4 animate-fade-in">
          <EntidadeScopeFields value={grants} onChange={setGrants} compact />
          <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="dns" />
          <CheckboxList label={t("dns.linkedHosts")} items={hosts.map((h) => ({ id: h.id, name: h.nickname }))} selected={form.host_ids} onChange={(ids) => set("host_ids", ids)} />
          <CheckboxList label={t("dns.linkedServices")} items={services.map((s) => ({ id: s.id, name: s.nickname }))} selected={form.service_ids} onChange={(ids) => set("service_ids", ids)} />
          <CheckboxList label={t("dns.linkedProjects")} items={projects.map((p) => ({ id: p.id, name: p.name }))} selected={form.project_ids} onChange={(ids) => set("project_ids", ids)} />
        </div>
      )}
    </div>
  );
}
