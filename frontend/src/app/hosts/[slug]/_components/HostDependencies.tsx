"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import SectionCard from "@/components/ui/SectionCard";
import StatusAlert from "@/components/ui/StatusAlert";
import Button from "@/components/ui/Button";
import { useAuth } from "@/contexts/AuthContext";
import { vulnAPI } from "@/lib/api";
import OSUpdateModal, { type UpdateHost } from "../../_components/OSUpdateModal";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import VulnDrawer from "@/components/vulns/VulnDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import DependencyList, { type DependencyFlag, type DependencyKindFilter } from "@/app/vulnerabilities/_components/DependencyList";

/** The host's software (on the host and in its containers), one row per
 *  version, grouped like Vulnerabilidades › Dependências. It opens on what has
 *  CVEs; a row expands to them, each with its finding's status. */
export default function HostDependencies({ hostId, host }: { hostId: number; host?: UpdateHost }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin" && !!host;
  // Distro packages on the host itself with CVEs: a security update is the fix.
  const params = { host_id: String(hostId), kind: "package", has_cves: "true", scope: "host", per_page: "200" };
  const { data: vulnerable } = useQuery({ queryKey: ["vulnerabilities", "dependency-groups", params], queryFn: () => vulnAPI.dependencyGroups(params), enabled: isAdmin });
  const fixable = (vulnerable?.data ?? []).filter((g) => g.lookup === "osv").length;
  // End-of-life versions have no CVE of their own, so the "with CVEs" list would hide them.
  const eolParams = { host_id: String(hostId), kind: "os,runtime,package", eol: "true", per_page: "1" };
  const { data: eol } = useQuery({ queryKey: ["vulnerabilities", "dependency-groups", eolParams], queryFn: () => vulnAPI.dependencyGroups(eolParams) });
  const eolCount = eol?.meta.total ?? 0;
  const [updating, setUpdating] = useState(false);
  const [kind, setKind] = useState<DependencyKindFilter>("all");
  const [flag, setFlag] = useState<DependencyFlag>("cves");
  const [findingId, setFindingId] = useState<number | null>(null);
  return (
    <SectionCard
      as="h3"
      title={t("vuln.host.software")}
      description={t("vuln.host.softwareHint")}
      controls={<>
        <ToolbarSelect<DependencyKindFilter>
          name={t("vuln.deps.kindLabel")}
          icon={ICON_PATHS.filter}
          value={kind}
          onChange={setKind}
          options={(["components", "packages", "all"] as const).map((k) => ({ value: k, label: t(`vuln.deps.kindFilter.${k}`) }))}
        />
        <ToolbarSelect<DependencyFlag>
          name={t("vuln.deps.flag")}
          icon={ICON_PATHS.filter}
          value={flag}
          onChange={setFlag}
          options={[
            { value: "", label: t("vuln.deps.flagAny") },
            { value: "cves", label: t("vuln.deps.flagCves") },
            { value: "eol", label: t("vuln.deps.flagEol") },
          ]}
        />
      </>}
    >
      {isAdmin && fixable > 0 && (
        <div className="mb-3">
          <StatusAlert variant="info">
            <span className="flex flex-wrap items-center justify-between gap-2">
              {t("vuln.deps.fixableCallout", { count: String(fixable) })}
              <Button size="sm" onClick={() => setUpdating(true)}>{t("vuln.deps.updateSecurity")}</Button>
            </span>
          </StatusAlert>
        </div>
      )}
      {eolCount > 0 && flag !== "eol" && (
        <div className="mb-3">
          <StatusAlert variant="warning">
            <span className="flex flex-wrap items-center justify-between gap-2">
              {t("vuln.host.eolCallout", { count: String(eolCount) })}
              <Button size="sm" variant="secondary" onClick={() => { setKind("all"); setFlag("eol"); }}>{t("common.show")}</Button>
            </span>
          </StatusAlert>
        </div>
      )}
      <DependencyList search="" kind={kind} flag={flag} hostId={hostId} onOpenFinding={setFindingId} />
      {updating && host && <OSUpdateModal target={{ by: "hosts", hosts: [host] }} preset="security"
        onClose={() => setUpdating(false)} onStarted={() => setUpdating(false)} />}
      <VulnDrawer id={findingId} onClose={() => setFindingId(null)} />
    </SectionCard>
  );
}
