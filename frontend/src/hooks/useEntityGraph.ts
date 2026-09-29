import { useQuery } from "@tanstack/react-query";
import { graphAPI } from "@/lib/api";
import { useFilteredGraph } from "@/hooks/useFilteredGraph";

/** The topology subgraph around one asset ("host-7", "dns-3", "service-12"),
 *  fetched only while its tab is open, plus whether it is still loading. */
export function useEntityGraph(nodeId: string | undefined, enabled: boolean) {
  const { data, isLoading } = useQuery({ queryKey: ["graph"], queryFn: graphAPI.get, enabled });
  const graph = useFilteredGraph(nodeId, data, enabled);
  return { graph, loading: enabled && isLoading };
}
