import { useMemo } from "react";
import type { GraphData } from "@/lib/types";

/**
 * Extracts a subgraph centered on a given entity node, including
 * up to `degrees` levels of connected neighbors.
 */
export function useFilteredGraph(
  entityNodeId: string | undefined,
  graphData: GraphData | undefined,
  enabled: boolean,
  degrees: number = 2,
): GraphData {
  return useMemo((): GraphData => {
    if (!enabled || !graphData || !entityNodeId) return { nodes: [], edges: [] };

    const connectedIds = new Set<string>([entityNodeId]);

    // Expand outwards for `degrees` levels
    for (let d = 0; d < degrees; d++) {
      const edgesThisLevel = graphData.edges.filter(
        (e) => connectedIds.has(e.source) || connectedIds.has(e.target),
      );
      edgesThisLevel.forEach((e) => {
        connectedIds.add(e.source);
        connectedIds.add(e.target);
      });
    }

    const edges = graphData.edges.filter(
      (e) => connectedIds.has(e.source) && connectedIds.has(e.target),
    );
    const nodes = graphData.nodes.filter((n) => connectedIds.has(n.id));

    return { nodes, edges };
  }, [graphData, entityNodeId, enabled, degrees]);
}
