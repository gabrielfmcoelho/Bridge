"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { dnsAPI, enumsAPI, contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useSituacao } from "@/hooks/useSituacao";
import { useDefaultSituacao } from "@/hooks/useDefaultSituacao";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import TagInput from "@/components/ui/TagInput";
import FormField from "@/components/ui/FormField";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import type { DNSRecord, EntityResponsavel, AssetGrants, AssetGrantsInput } from "@/lib/types";

// A hostname, optionally a leading wildcard label: "*.sead.pi.gov.br".
const DOMAIN_RE = /^(\*\.)?([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9-]{2,63}$/i;

interface DnsFormProps {
  initial?: DNSRecord | null;
  initialTags?: string[];
  initialHostIds?: number[];
  initialServiceIds?: number[];
  initialProjectIds?: number[];
  initialResponsaveis?: EntityResponsavel[];
  initialGrants?: AssetGrants | null;
  onSuccess: () => void;
  onClose?: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

export default function DnsForm({
  initial, initialTags, initialHostIds, initialServiceIds, initialProjectIds, initialResponsaveis, initialGrants,
  onSuccess, onClose, onFooterChange, onSubHeaderChange,
}: DnsFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!initial;
  const { valueOf: situacaoValueOf } = useSituacao();
  const [form, setForm] = useState({
    domain: initial?.domain || "",
    has_https: initial?.has_https || false,
    situacao: initial?.situacao || situacaoValueOf("active"),
    observacoes: initial?.observacoes || "",
  });
  const [hostIds, setHostIds] = useState<number[]>(initialHostIds || initial?.host_ids || []);
  const [serviceIds, setServiceIds] = useState<number[]>(initialServiceIds || []);
  const [projectIds, setProjectIds] = useState<number[]>(initialProjectIds || []);
  const [tags, setTags] = useState<string[]>(initialTags || initial?.tags || []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initialResponsaveis || []);
  const [grants, setGrants] = useState<AssetGrantsInput>(initialGrants ?? defaultGrants(user));
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: situacoes = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const relationOptions = useRelationOptions(["hosts", "services", "projects"]);

  const set = (key: keyof typeof form, value: unknown) => setForm((f) => ({ ...f, [key]: value }));
  useDefaultSituacao(form.situacao, (v) => set("situacao", v), !isEdit);

  const mutation = useMutation({
    mutationFn: () => {
      const data = {
        ...form,
        domain: form.domain.trim(),
        tags,
        // Always sent, empty included: an empty list is how the last link goes.
        host_ids: hostIds,
        service_ids: serviceIds,
        project_ids: projectIds,
        responsaveis: responsaveis.filter((r) => r.name),
        ...grants,
      };
      return initial ? dnsAPI.update(initial.id, data) : dnsAPI.create(data);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const errors: Record<string, string> = {};
  if (!form.domain.trim()) errors.domain = t("form.required");
  // Format only when typed here: an older record saved in another shape stays editable.
  else if (form.domain.trim() !== (initial?.domain ?? "") && !DOMAIN_RE.test(form.domain.trim())) errors.domain = t("form.domainInvalid");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  const sections = [
    { id: "dns-identity", label: t("form.section.identity") },
    { id: "dns-owners", label: t("form.section.owners") },
    { id: "dns-links", label: t("form.section.links") },
    { id: "dns-notes", label: t("form.section.notes") },
  ];

  return (
    <EntityFormShell
      id="dns-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("form.createDns")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="dns-identity" title={t("form.section.identity")}>
        <div className="sm:col-span-2">
          <Input label={t("dns.domain")} value={form.domain} onChange={(e) => set("domain", e.target.value)} required
            placeholder={t("dns.domainPlaceholder")} className="font-mono" error={err("domain")} aria-invalid={!!err("domain")} />
        </div>
        <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((e) => ({ value: e.value, label: e.value }))} />
        <FormField label="HTTPS" hint={t("form.httpsHint")}>
          <Checkbox label={t("dns.hasHttps")} checked={form.has_https} onChange={(v) => set("has_https", v)} />
        </FormField>
      </FormSection>

      <FormSection id="dns-owners" title={t("form.section.owners")} description={t("form.section.ownersHint")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
        <ResponsavelList value={responsaveis} onChange={setResponsaveis} contacts={contacts} t={t} />
      </FormSection>

      <FormSection id="dns-links" title={t("form.section.links")} description={t("form.section.linksHint")} stack>
        <RelationPicker label={t("nav.hosts")} options={relationOptions.hosts} selected={hostIds} onChange={setHostIds} />
        <RelationPicker label={t("topology.services")} options={relationOptions.services} selected={serviceIds} onChange={setServiceIds} />
        <RelationPicker label={t("topology.projects")} options={relationOptions.projects} selected={projectIds} onChange={setProjectIds} />
      </FormSection>

      <FormSection id="dns-notes" title={t("form.section.notes")} stack>
        <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="dns" />
        <FormField label={t("common.observacoes")}>
          <MarkdownEditor value={form.observacoes} onChange={(v) => set("observacoes", v)} rows={4} placeholder={t("dns.notesPlaceholder")} />
        </FormField>
      </FormSection>
    </EntityFormShell>
  );
}
