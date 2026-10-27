"use client";

import { useState } from "react";
import SectionCard from "@/components/ui/SectionCard";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import VulnDrawer from "@/components/vulns/VulnDrawer";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import DependencyList, { type DependencyFlag, type DependencyKindFilter } from "@/app/vulnerabilities/_components/DependencyList";

/** The host's dependency versions (on the host and in its containers), grouped
 *  like Vulnerabilidades › Dependências — under the host's findings. */
export default function HostDependencies({ hostId }: { hostId: number }) {
  const { t } = useLocale();
  const [kind, setKind] = useState<DependencyKindFilter>("components");
  const [flag, setFlag] = useState<DependencyFlag>("");
  const [findingId, setFindingId] = useState<number | null>(null);
  return (
    <SectionCard
      as="h3"
      title={t("vuln.view.dependencies")}
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
      <DependencyList search="" kind={kind} flag={flag} hostId={hostId} onOpenFinding={setFindingId} />
      <VulnDrawer id={findingId} onClose={() => setFindingId(null)} />
    </SectionCard>
  );
}
