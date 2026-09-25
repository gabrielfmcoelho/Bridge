"use client";

import SectionCard from "@/components/ui/SectionCard";
import TopologyGraph from "@/components/graph/TopologyGraph";
import { useLocale } from "@/contexts/LocaleContext";
import type { GraphData } from "@/lib/types";

interface TopologyTabProps {
  filteredGraph: GraphData;
}

export default function TopologyTab({ filteredGraph }: TopologyTabProps) {
  const { t } = useLocale();
  return (
    <div className="animate-fade-in">
      {filteredGraph.nodes.length > 0 ? (
        <SectionCard title={t("project.topologyHeading")} body="flush">
          <div className="h-[calc(50vh-3.5rem)] lg:h-[calc(100vh-21.5rem)]">
            <TopologyGraph data={filteredGraph} className="w-full h-full" />
          </div>
        </SectionCard>
      ) : (
        <p className="text-sm text-[var(--text-faint)] text-center py-8">{t("project.noTopologyData")}</p>
      )}
    </div>
  );
}
