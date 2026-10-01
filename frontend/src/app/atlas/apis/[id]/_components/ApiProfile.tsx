"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { entidadesAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalog } from "@/lib/types";
import { useApiLinkNames } from "../../_components/apiDisplay";

/**
 * The left column of an API's "Visão geral": what was declared — spec
 * identity, where it lives, who owns it and what it belongs to. Every field
 * renders; an empty one is a muted "–".
 */
export default function ApiProfile({ api, onEditResponsaveis }: { api: ApiCatalog; onEditResponsaveis?: () => void }) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list, enabled: !!api.entidades });
  const entidadeName = (id: number) => entidades.find((e) => e.id === id)?.name ?? `#${id}`;
  const grants = api.entidades;
  const block = "pt-4 mt-4 border-t border-[var(--border-subtle)]";
  const linkList = (ids: number[], names: Map<number, string>, base: string) =>
    ids.length === 0 ? <p className="text-sm text-[var(--text-muted)]">–</p> : (
      <div className="flex flex-wrap gap-x-3 gap-y-1">
        {ids.map((id) => <Link key={id} href={`${base}/${id}`} className="text-sm text-[var(--accent)] hover:underline">{names.get(id) ?? `#${id}`}</Link>)}
      </div>
    );

  return (
    <>
      <SectionCard title={t("atlas.apis.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("atlas.apis.specTitle")} value={api.title} />
          <Field label={t("atlas.apis.version")} value={api.version_label} mono />
          <Field label={t("atlas.apis.specVersion")} value={api.spec_version} mono />
          <Field label={t("atlas.apis.sourceLabel")} value={t(`atlas.apis.sourceType.${api.source_type}`)} />
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("service.links")}</SectionHeading>
          <div className="grid grid-cols-1 gap-4">
            <Field label={t("atlas.apis.baseUrl")} value={api.base_url || ""} link mono />
            {(api.urls ?? []).map((u) => (
              <Field key={u.url} label={u.label || t("atlas.apis.extraUrl")} value={u.url} link mono />
            ))}
            <Field label={t("atlas.apis.docsUrl")} value={api.docs_url || ""} link mono />
            <Field label={t("atlas.apis.sourceUrl")} value={api.source_url || ""} link mono />
            {api.external_url && api.external_url !== api.base_url && <Field label={t("atlas.apis.specServer")} value={api.external_url} link mono />}
          </div>
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("atlas.apis.links")}</SectionHeading>
          <div className="grid grid-cols-1 gap-4">
            <div>
              <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{t("topology.services")}</span>
              {linkList(api.service_ids ?? [], names.service, "/services")}
            </div>
            <div>
              <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{t("topology.projects")}</span>
              {linkList(api.project_ids ?? [], names.project, "/projects")}
            </div>
          </div>
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("entidades.title")}</SectionHeading>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("entidades.creator")} value={grants?.creator_entidade_id ? entidadeName(grants.creator_entidade_id) : ""} />
            <Field label={t("entidades.global")} value={grants ? (grants.is_global ? t("common.yes") : t("common.no")) : ""} />
            <Field className="col-span-2" label={t("entidades.responsibles")} value={(grants?.responsible_entidade_ids ?? []).map(entidadeName).join(", ")} />
          </div>
        </div>
      </SectionCard>
      <ResponsaveisSection responsaveis={api.responsaveis ?? []} t={t} onEdit={onEditResponsaveis} />
    </>
  );
}
