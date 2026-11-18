import { useQuery } from "@tanstack/react-query";
import { appDepsAPI } from "@/lib/api";
import type { AppDepAssetLink, AppDepEntityType } from "@/lib/types";

const NONE: AppDepAssetLink[] = [];

/** Application links of one asset ({entity_type, entity_id}) or one CI ({ci}),
 *  or every link ({}). Fetched only while enabled. */
export function useAppDepLinks(f: { ci?: string; entity_type?: AppDepEntityType; entity_id?: number }, enabled = true) {
  const { data, isLoading } = useQuery({
    queryKey: ["app-dep-links", f.ci ?? "", f.entity_type ?? "", f.entity_id ?? 0],
    queryFn: () => appDepsAPI.links(f),
    enabled,
    staleTime: 30_000,
  });
  return { links: data ?? NONE, loading: enabled && isLoading };
}
