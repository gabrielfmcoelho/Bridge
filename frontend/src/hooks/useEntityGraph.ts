import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { graphAPI } from "@/lib/api";
import type { GraphData } from "@/lib/types";

const EMPTY: GraphData = { nodes: [], edges: [] };

/** The topology around one asset ("host-7", "dns-3", "service-12"), fetched only
 *  while its tab is open. Returns the whole graph with `focus` set: what is shown
 *  (related assets, collapsed host siblings, groups) is decided in lib/topology. */
export function useEntityGraph(nodeId: string | undefined, enabled: boolean) {
  const { data, isLoading } = useQuery({ queryKey: ["graph"], queryFn: graphAPI.get, enabled });
  const graph = useMemo(
    () => (enabled && data && nodeId ? { ...data, focus: nodeId } : EMPTY),
    [data, nodeId, enabled],
  );
  return { graph, loading: enabled && isLoading };
}
