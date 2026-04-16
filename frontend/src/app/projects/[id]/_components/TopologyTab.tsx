"use client";

import Card from "@/components/ui/Card";
import TopologyGraph from "@/components/graph/TopologyGraph";
import type { GraphData } from "@/lib/types";

interface TopologyTabProps {
  filteredGraph: GraphData;
}

export default function TopologyTab({ filteredGraph }: TopologyTabProps) {
  return (
    <div className="animate-fade-in">
      {filteredGraph.nodes.length > 0 ? (
        <Card hover={false} className="overflow-hidden flex flex-col h-[50vh] lg:h-[calc(100vh-18rem)]">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)] mb-3 shrink-0" style={{ fontFamily: "var(--font-display)" }}>Topology</h2>
          <div className="flex-1 rounded-[var(--radius-md)] overflow-hidden border border-[var(--border-subtle)]">
            <TopologyGraph data={filteredGraph} className="w-full h-full" />
          </div>
        </Card>
      ) : (
        <p className="text-sm text-[var(--text-faint)] text-center py-8">No topology data for this project.</p>
      )}
    </div>
  );
}
