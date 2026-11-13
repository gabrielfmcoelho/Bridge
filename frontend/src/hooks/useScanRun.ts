import { useQuery } from "@tanstack/react-query";
import { scanAPI } from "@/lib/api";

/** A background scan run, polled every 3 s while it runs (null: none). */
export function useScanRun(id: number | null) {
  return useQuery({
    queryKey: ["scan-run", id],
    queryFn: () => scanAPI.run(id!),
    enabled: id != null,
    refetchInterval: (q) => (q.state.data?.status === "running" ? 3000 : false),
  });
}
