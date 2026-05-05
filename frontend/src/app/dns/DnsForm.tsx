"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { dnsAPI, hostsAPI, enumsAPI, contactsAPI } from "@/lib/api";
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
import type { DNSRecord, EntityResponsavel } from "@/lib/types";

interface DnsFormProps {
  initial?: DNSRecord | null;
  initialTags?: string[];
  initialHostIds?: number[];
  initialResponsaveis?: EntityResponsavel[];
  onSuccess: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}

export default function DnsForm({
  initial,
  initialTags,
  initialHostIds,
  initialResponsaveis,
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
  });
  const [tags, setTags] = useState<string[]>(initialTags || initial?.tags || []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initialResponsaveis || []);
  const [error, setError] = useState("");

  const { data: hosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
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
    stepLabels: ["DNS Info", t("host.responsaveis") || "Responsaveis", "Links & Tags"],
    onSubmit: () => mutation.mutate(),
    canProceed: step === 1 ? !!form.domain.trim() : true,
    isPending: mutation.isPending,
    submitLabel: initial ? t("common.save") : t("common.create"),
    t,
    onFooterChange,
    onSubHeaderChange,
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  const hasExternalFooter = !!onFooterChange;

  return (
    <div className="space-y-4">
      <FormError message={error} />

      {step === 1 && (
        <div className="space-y-4 animate-fade-in">
          <Input label={t("dns.domain")} value={form.domain} onChange={(e) => set("domain", e.target.value)} required placeholder="e.g. example.com" />
          <Select label={t("host.situacao")} value={form.situacao} onChange={(e) => set("situacao", e.target.value)} options={situacoes.map((e) => ({ value: e.value, label: e.value }))} />
          <Checkbox label={t("dns.hasHttps")} checked={form.has_https} onChange={(v) => set("has_https", v)} />
          <Input label={t("common.observacoes")} value={form.observacoes} onChange={(e) => set("observacoes", e.target.value)} placeholder="Notes about this DNS record..." />
          {!hasExternalFooter && (
            <Button type="button" className="w-full" disabled={!form.domain.trim()} onClick={() => setStep(2)}>
              {t("host.nextStep")}
            </Button>
          )}
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
          {!hasExternalFooter && (
            <div className="flex gap-2 pt-2">
              <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(1)}>{t("common.back")}</Button>
              <Button type="button" className="flex-1" onClick={() => setStep(3)}>{t("host.nextStep")}</Button>
            </div>
          )}
        </div>
      )}

      {step === 3 && (
        <div className="space-y-4 animate-fade-in">
          <TagInput label={t("common.tags")} tags={tags} onChange={setTags} entityType="dns" />
          <CheckboxList label="Linked Hosts" items={hosts.map((h) => ({ id: h.id, name: h.nickname }))} selected={form.host_ids} onChange={(ids) => set("host_ids", ids)} />
          {!hasExternalFooter && (
            <div className="flex gap-2 pt-2">
              <Button type="button" variant="secondary" className="flex-1" onClick={() => setStep(2)}>{t("common.back")}</Button>
              <Button type="button" className="flex-1" onClick={() => mutation.mutate()} loading={mutation.isPending}>
                {initial ? t("common.save") : t("common.create")}
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
