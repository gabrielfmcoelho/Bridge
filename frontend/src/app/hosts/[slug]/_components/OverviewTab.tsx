"use client";

import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Field from "@/components/ui/Field";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import PasswordField from "./PasswordField";
import type { Host, HostResponsavel, HostChamado } from "@/lib/types";

interface OverviewTabProps {
  host: Host;
  tags: string[];
  responsaveis: HostResponsavel[];
  chamados: HostChamado[];
  canEdit: boolean;
  isAdmin: boolean;
  slug: string;
  t: (key: string) => string;
}

export default function OverviewTab({ host, tags, responsaveis, chamados, canEdit, isAdmin, slug, t }: OverviewTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      <SectionHeading>{t("host.basicInfo")}</SectionHeading>
      <Card hover={false}>
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4 text-sm">
          <Field label={t("host.hostname")} value={host.hostname} mono />
          <Field label={t("host.hospedagem")} value={host.hospedagem} />
          <Field label={t("host.tipoMaquina")} value={host.tipo_maquina} />
        </div>
        {tags && tags.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-4 pt-4 border-t border-[var(--border-subtle)]">
            {tags.map((tag) => <Badge key={tag}>{tag}</Badge>)}
          </div>
        )}
      </Card>

      <SectionHeading>{t("common.observacoes")}</SectionHeading>
      <Card hover={false}>
        {host.observacoes ? (
          <MarkdownContent content={host.observacoes} />
        ) : (
          <p className="text-sm text-[var(--text-faint)]">-</p>
        )}
      </Card>

      <ResponsaveisSection responsaveis={responsaveis} t={t} />

      {(host.recurso_cpu || host.recurso_ram || host.recurso_armazenamento) && (
        <>
        <SectionHeading>{t("host.resources")}</SectionHeading>
        <Card hover={false}>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-sm">
            <Field label={t("host.recursoCpu")} value={host.recurso_cpu} mono />
            <Field label={t("host.recursoRam")} value={host.recurso_ram} mono />
            <Field label={t("host.recursoArmazenamento")} value={host.recurso_armazenamento} mono />
          </div>
        </Card>
        </>
      )}

      {canEdit && (
        <>
        <SectionHeading>
          {t("host.sshConnection")}
          <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
        </SectionHeading>
        <Card hover={false}>
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <Field label={t("host.user")} value={host.user} />
            <Field label={t("host.port")} value={host.port || "22"} />
            <Field label={t("host.proxyJump")} value={host.proxy_jump} />
            <Field label={t("host.sshKey")} value={host.has_key ? t("host.sshKeyStored") : ""} />
            {host.has_password && isAdmin && <PasswordField slug={slug} />}
          </div>
        </Card>
        </>
      )}
    </div>
  );
}
