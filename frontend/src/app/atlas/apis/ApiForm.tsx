"use client";

import { useId, useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { apiCatalogAPI, contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import FormField, { INPUT_CLASS } from "@/components/ui/FormField";
import PillButton from "@/components/ui/PillButton";
import Button from "@/components/ui/Button";
import Field from "@/components/ui/Field";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import { SPEC_ACCEPT, useSpecActions } from "./_components/useSpecActions";
import type { ApiCatalog, AssetGrantsInput, EntityResponsavel } from "@/lib/types";

const URL_RE = /^https?:\/\/\S+$/i;

/** What a create form can start with (an API added from a service or project). */
export interface ApiFormPrefill {
  serviceIds?: number[];
  projectIds?: number[];
  baseUrl?: string;
  specUrl?: string;
}

interface ApiFormProps {
  /** The detail response (with responsaveis and entidades) on edit. */
  initial?: ApiCatalog | null;
  prefill?: ApiFormPrefill;
  onSuccess: (api: ApiCatalog) => void;
  onClose?: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
  onFooterChange?: (footer: React.ReactNode) => void;
}

/**
 * Create or edit a catalogued API. Create imports the spec (file or URL) with
 * its links and grants in one call; responsáveis follow in a PUT, since the
 * import endpoints don't take them. Edit changes metadata and links; the spec
 * itself is refreshed in place from the Origem section.
 */
export default function ApiForm({ initial, prefill, onSuccess, onClose, onSubHeaderChange, onFooterChange }: ApiFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const fileId = useId();
  const isEdit = !!initial;
  const [grants, setGrants] = useState<AssetGrantsInput>(initial?.entidades ?? defaultGrants(user));
  const [name, setName] = useState(initial?.name ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [sourceMode, setSourceMode] = useState<"upload" | "url">(prefill?.specUrl ? "url" : "upload");
  const [file, setFile] = useState<File | null>(null);
  const [sourceUrl, setSourceUrl] = useState(prefill?.specUrl ?? "");
  const [baseUrl, setBaseUrl] = useState(initial?.base_url ?? prefill?.baseUrl ?? "");
  const [docsUrl, setDocsUrl] = useState(initial?.docs_url ?? "");
  const [serviceIds, setServiceIds] = useState<number[]>(initial?.service_ids ?? prefill?.serviceIds ?? []);
  const [projectIds, setProjectIds] = useState<number[]>(initial?.project_ids ?? prefill?.projectIds ?? []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initial?.responsaveis ?? []);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const relationOptions = useRelationOptions(["services", "projects"]);
  const spec = useSpecActions(initial ?? undefined);

  const owners = () => responsaveis.filter((r) => r.name).map((r) => ({ contact_id: r.contact_id, is_main: r.is_main }));

  const mutation = useMutation({
    mutationFn: async () => {
      const meta = {
        name: name.trim(),
        description: description.trim(),
        base_url: baseUrl.trim(),
        docs_url: docsUrl.trim(),
        service_ids: serviceIds,
        project_ids: projectIds,
        ...grants,
      };
      if (initial) return apiCatalogAPI.update(initial.id, { ...meta, responsaveis: owners() });
      const created = sourceMode === "upload"
        ? await apiCatalogAPI.importUpload(file!, { ...meta, name: meta.name || undefined })
        : await apiCatalogAPI.importURL({ ...meta, name: meta.name || undefined, source_url: sourceUrl.trim() });
      if (owners().length === 0) return created;
      return apiCatalogAPI.update(created.id, {
        name: created.name, description: created.description, base_url: created.base_url, docs_url: created.docs_url, responsaveis: owners(),
      });
    },
    onSuccess: (api) => onSuccess(api),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const errors: Record<string, string> = {};
  if (isEdit && !name.trim()) errors.name = t("form.required");
  if (!isEdit && sourceMode === "upload" && !file) errors.file = t("form.required");
  if (!isEdit && sourceMode === "url" && !URL_RE.test(sourceUrl.trim())) errors.sourceUrl = sourceUrl.trim() ? t("form.urlInvalid") : t("form.required");
  if (baseUrl.trim() && baseUrl !== (initial?.base_url ?? "") && !URL_RE.test(baseUrl.trim())) errors.baseUrl = t("form.urlInvalid");
  if (docsUrl.trim() && docsUrl !== (initial?.docs_url ?? "") && !URL_RE.test(docsUrl.trim())) errors.docsUrl = t("form.urlInvalid");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  const sections = [
    { id: "api-identity", label: t("form.section.identity") },
    { id: "api-origin", label: t("form.section.origin") },
    { id: "api-operation", label: t("form.section.operation") },
    { id: "api-owners", label: t("form.section.owners") },
    { id: "api-links", label: t("form.section.links") },
    { id: "api-entidades", label: t("entidades.title") },
  ];

  return (
    <EntityFormShell
      id="api-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("atlas.apis.import")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="api-identity" title={t("form.section.identity")}>
        <div className="sm:col-span-2">
          <Input label={t("atlas.apis.name")} value={name} onChange={(e) => setName(e.target.value)} required={isEdit}
            placeholder={isEdit ? undefined : t("atlas.apis.namePlaceholder")} error={err("name")} aria-invalid={!!err("name")} />
        </div>
        <div className="sm:col-span-2">
          <Textarea label={t("common.description")} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </div>
      </FormSection>

      <FormSection id="api-origin" title={t("form.section.origin")} description={t("atlas.apis.originHint")}>
        {isEdit && initial ? (
          <>
            <Field label={t("atlas.apis.sourceLabel")} value={t(`atlas.apis.sourceType.${initial.source_type}`)} />
            <Field label={t("atlas.apis.specVersion")} value={[initial.spec_version, initial.version_label].filter(Boolean).join(" · ")} mono />
            {initial.source_url && <Field className="sm:col-span-2" label={t("atlas.apis.sourceUrl")} value={initial.source_url} link mono />}
            <div className="sm:col-span-2 flex flex-wrap gap-2">
              {initial.source_type === "url" && (
                <Button type="button" size="sm" variant="secondary" loading={spec.refetch.isPending} onClick={() => spec.refetch.mutate()}>{t("atlas.apis.refetch")}</Button>
              )}
              <Button type="button" size="sm" variant="secondary" loading={spec.replace.isPending} onClick={spec.pickFile}>{t("atlas.apis.replaceSpec")}</Button>
              {spec.fileInput}
            </div>
          </>
        ) : (
          <>
            <div className="sm:col-span-2 flex gap-1.5">
              <PillButton active={sourceMode === "upload"} onClick={() => setSourceMode("upload")}>{t("atlas.apis.importUpload")}</PillButton>
              <PillButton active={sourceMode === "url"} onClick={() => setSourceMode("url")}>{t("atlas.apis.importUrl")}</PillButton>
            </div>
            <div className="sm:col-span-2">
              {sourceMode === "upload" ? (
                <FormField label={t("atlas.apis.specFile")} htmlFor={fileId} required error={err("file")}>
                  <input id={fileId} type="file" accept={SPEC_ACCEPT} aria-invalid={!!err("file")}
                    onChange={(e) => setFile(e.target.files?.[0] ?? null)}
                    className={`${INPUT_CLASS} file:mr-3 file:border-0 file:bg-transparent file:text-[var(--text-secondary)]`} />
                </FormField>
              ) : (
                <Input label={t("atlas.apis.sourceUrl")} hint={t("atlas.apis.sourceUrlHint")} value={sourceUrl} onChange={(e) => setSourceUrl(e.target.value)}
                  required type="url" className="font-mono" placeholder="https://api.example.com/openapi.json" error={err("sourceUrl")} aria-invalid={!!err("sourceUrl")} />
              )}
            </div>
          </>
        )}
      </FormSection>

      <FormSection id="api-operation" title={t("form.section.operation")}>
        <Input label={t("atlas.apis.baseUrl")} hint={t("atlas.apis.baseUrlHint")} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)}
          type="url" className="font-mono" placeholder="https://api.example.com" error={err("baseUrl")} aria-invalid={!!err("baseUrl")} />
        <Input label={t("atlas.apis.docsUrl")} hint={t("atlas.apis.docsUrlHint")} value={docsUrl} onChange={(e) => setDocsUrl(e.target.value)}
          type="url" className="font-mono" placeholder="https://api.example.com/docs" error={err("docsUrl")} aria-invalid={!!err("docsUrl")} />
      </FormSection>

      <FormSection id="api-owners" title={t("form.section.owners")} stack>
        <ResponsavelList value={responsaveis} onChange={setResponsaveis} contacts={contacts} t={t} />
      </FormSection>

      <FormSection id="api-links" title={t("form.section.links")} description={t("atlas.apis.linksHint")} stack>
        <RelationPicker label={t("topology.services")} options={relationOptions.services} selected={serviceIds} onChange={setServiceIds} />
        <RelationPicker label={t("topology.projects")} options={relationOptions.projects} selected={projectIds} onChange={setProjectIds} />
      </FormSection>

      <FormSection id="api-entidades" title={t("entidades.title")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
      </FormSection>
    </EntityFormShell>
  );
}
