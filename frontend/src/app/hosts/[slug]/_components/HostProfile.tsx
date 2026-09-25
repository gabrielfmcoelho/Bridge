"use client";

import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Field from "@/components/ui/Field";
import Tag from "@/components/ui/Tag";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import ResponsaveisSection from "@/components/inventory/ResponsaveisSection";
import PasswordField from "./PasswordField";
import type { Host, HostResponsavel } from "@/lib/types";

/**
 * The left column of the host "Visão geral": what was *declared* about the
 * host — identity, declared resources, SSH access (editors), tags, notes —
 * then its responsáveis. The scan column beside it shows what was *observed*.
 * Every field always renders; an empty one is a muted "–" (fixed anatomy).
 */
export default function HostProfile({
  host,
  tags,
  responsaveis,
  canEdit,
  isAdmin,
  slug,
  t,
}: {
  host: Host;
  tags: string[];
  responsaveis: HostResponsavel[];
  canEdit: boolean;
  isAdmin: boolean;
  slug: string;
  t: (key: string) => string;
}) {
  const block = "pt-4 mt-4 border-t border-[var(--border-subtle)]";
  return (
    <div className="space-y-5">
      <SectionCard title={t("host.profileTitle")}>
        <div className="grid grid-cols-2 gap-4">
          <Field className="col-span-2" label={t("host.hostname")} value={host.hostname} mono />
          <Field label={t("host.hospedagem")} value={host.hospedagem} />
          <Field label={t("host.tipoMaquina")} value={host.tipo_maquina} />
          <Field label="Proxmox" value={host.proxmox_id ?? ""} mono />
          <Field label={t("host.runsOn")} value={host.parent_host_slug ?? ""} href={host.parent_host_slug ? `/hosts/${host.parent_host_slug}` : undefined} mono />
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-1">{t("host.declared")}</SectionHeading>
          <p className="text-xs text-[var(--text-muted)] mb-3">{t("host.declaredHint")}</p>
          <div className="grid grid-cols-3 gap-4">
            <Field label="CPU" value={host.recurso_cpu} mono />
            <Field label="RAM" value={host.recurso_ram} mono />
            <Field label={t("vm.disk")} value={host.recurso_armazenamento} mono />
          </div>
        </div>

        {canEdit && (
          <div className={block}>
            <SectionHeading as="h3" className="!mb-2">{t("host.sshConnection")}</SectionHeading>
            <div className="grid grid-cols-2 gap-4">
              <Field label={t("host.user")} value={host.user} mono />
              <Field label={t("host.port")} value={host.port || "22"} mono />
              <Field label={t("host.proxyJump")} value={host.proxy_jump} mono />
              <Field label={t("host.sshKey")} value={host.has_key ? t("host.sshKeyStored") : ""} />
              {host.has_password && isAdmin && <div className="col-span-2"><PasswordField slug={slug} /></div>}
            </div>
          </div>
        )}

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("common.tags")}</SectionHeading>
          <div className="flex flex-wrap gap-1.5">
            {tags.length > 0 ? tags.map((tag) => <Tag key={tag}>{tag}</Tag>) : <span className="text-sm text-[var(--text-muted)]">–</span>}
          </div>
        </div>

        <div className={block}>
          <SectionHeading as="h3" className="!mb-2">{t("common.observacoes")}</SectionHeading>
          {host.observacoes ? <MarkdownContent content={host.observacoes} /> : <p className="text-sm text-[var(--text-muted)]">–</p>}
        </div>
      </SectionCard>

      <ResponsaveisSection responsaveis={responsaveis} t={t} compact />
    </div>
  );
}
