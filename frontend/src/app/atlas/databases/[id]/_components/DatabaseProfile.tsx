"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { entidadesAPI, hostsAPI } from "@/lib/api";
import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import { useLocale } from "@/contexts/LocaleContext";
import type { Database } from "@/lib/types";
import { useApiLinkNames } from "@/app/atlas/apis/_components/apiDisplay";
import { endpointLabel, isExternal } from "../../_components/databaseInsights";

/**
 * The left column of a database's "Visão geral": where it lives, how Bridge
 * reaches it, how it is classified, who owns it. An empty field is a muted "–".
 */
export default function DatabaseProfile({ db, onEditResponsaveis }: { db: Database; onEditResponsaveis?: () => void }) {
  const { t, locale } = useLocale();
  const names = useApiLinkNames();
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list, enabled: !!db.entidades });
  const { data: hosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list(), enabled: db.tunnel_host_id != null });
  const entidadeName = (id: number) => entidades.find((e) => e.id === id)?.name ?? `#${id}`;
  const grants = db.entidades;
  const inst = db.instance;
  const tunnelHost = hosts.find((h) => h.id === db.tunnel_host_id);
  const block = "pt-4 mt-4 border-t border-[var(--border-subtle)]";
  const label = "text-[var(--text-muted)] text-xs font-medium block mb-0.5";

  return (
    <>
      <SectionCard title={t("atlas.databases.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field label={t("atlas.databases.engineLabel")} value={t(`atlas.databases.engine.${db.engine}`)} />
          <Field label={t("atlas.databases.dbName")} value={db.db_name} mono />
          {isExternal(db) ? (
            <Field className="col-span-2" label={t("atlas.databases.provider")} value={db.external_provider} />
          ) : (
            <div className="col-span-2">
              <span className={label}>{t("atlas.databases.instance")}</span>
              {inst ? (
                <p className="text-sm">
                  <Link href={`/services/${inst.id}`} className="text-[var(--accent)] hover:underline">{inst.nickname}</Link>
                  {inst.host_nickname && <> · <Link href={`/hosts/${inst.host_slug}`} className="text-[var(--accent)] hover:underline">{inst.host_nickname}</Link></>}
                  {(inst.container_image || inst.service_subtype) && <span className="ml-2 font-mono text-xs text-[var(--text-muted)]">{inst.container_image || inst.service_subtype}</span>}
                </p>
              ) : <p className="text-sm text-[var(--text-muted)]">{t("atlas.databases.instanceHidden")}</p>}
            </div>
          )}
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("atlas.databases.section.connection")}</SectionHeading>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("atlas.databases.endpoint")} value={endpointLabel(db)} mono />
            <Field label={t("atlas.databases.connectionMode")} value={t(`atlas.databases.mode.${db.connection_mode}`)} />
            {db.connection_mode === "ssh_tunnel" && (
              <>
                <Field label={t("atlas.databases.tunnelHost")} value={tunnelHost?.nickname ?? (db.tunnel_host_id ? `#${db.tunnel_host_id}` : "")} />
                <Field label={t("atlas.databases.tunnelTarget")} value={db.tunnel_target || inst?.container_name || ""} mono />
              </>
            )}
            {db.connection_mode !== "none" && <Field label={t("atlas.databases.tls")} value={t(`atlas.databases.tlsMode.${db.tls_mode}`)} />}
            <Field label={t("atlas.databases.credential")} value={db.has_credential ? t("atlas.databases.hasCredential") : t("atlas.databases.noCredential")} />
          </div>
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("atlas.databases.section.classification")}</SectionHeading>
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("atlas.databases.sensitivityLabel")} value={t(`atlas.databases.sensitivity.${db.sensitivity}`)} />
            <Field label={t("atlas.databases.environmentLabel")} value={t(`atlas.databases.environment.${db.environment}`)} />
            <Field className="col-span-2" label={t("atlas.databases.backupPolicy")} value={db.backup_policy} />
            <Field label={t("atlas.databases.retentionDays")} value={db.backup_retention_days != null ? String(db.backup_retention_days) : ""} />
            <Field label={t("atlas.databases.lastBackup")} value={db.last_backup_at ? new Date(db.last_backup_at).toLocaleDateString(locale) : ""} />
          </div>
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("topology.projects")}</SectionHeading>
          {(db.project_ids ?? []).length === 0 ? <p className="text-sm text-[var(--text-muted)]">–</p> : (
            <div className="flex flex-wrap gap-x-3 gap-y-1">
              {db.project_ids.map((id) => <Link key={id} href={`/projects/${id}`} className="text-sm text-[var(--accent)] hover:underline">{names.project.get(id) ?? `#${id}`}</Link>)}
            </div>
          )}
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
      <ResponsaveisSection responsaveis={db.responsaveis ?? []} t={t} onEdit={onEditResponsaveis} />
    </>
  );
}
