"use client";

import { useState } from "react";
import SectionCard from "@/components/ui/SectionCard";
import ToolbarSelect from "@/components/ui/ToolbarSelect";
import EntityVulnsTab from "@/components/vulns/EntityVulnsTab";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { UpdateHost } from "../../_components/OSUpdateModal";
import HostDependencies from "./HostDependencies";

type Origin = "scanners" | "inventory";
const SOURCES: Record<Origin, readonly string[]> = { scanners: ["etipi", "wazuh"], inventory: ["inventory"] };

/** The host's Vulnerabilidades tab, by what is affected: its software, each
 *  version with its own CVEs, then the findings no version explains (scanner
 *  reports). Inventory findings are read under their component; the flat list
 *  of them stays one select away (it also keeps those of versions now gone). */
export default function HostVulns({ hostId, host }: { hostId: number; host: UpdateHost }) {
  const { t } = useLocale();
  const [origin, setOrigin] = useState<Origin>("scanners");
  return (
    <div className="space-y-6">
      <HostDependencies hostId={hostId} host={host} />
      <SectionCard
        as="h3"
        title={t("vuln.host.otherFindings")}
        description={t("vuln.host.otherFindingsHint")}
        controls={
          <ToolbarSelect<Origin>
            name={t("vuln.source")}
            icon={ICON_PATHS.filter}
            value={origin}
            onChange={setOrigin}
            options={[
              { value: "scanners", label: t("vuln.host.originScanners") },
              { value: "inventory", label: t("vuln.host.originInventory") },
            ]}
          />
        }
      >
        <EntityVulnsTab key={origin} entityType="host" entityId={hostId} sources={SOURCES[origin]} />
      </SectionCard>
    </div>
  );
}
