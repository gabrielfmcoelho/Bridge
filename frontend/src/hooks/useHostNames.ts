"use client";

import { useQuery } from "@tanstack/react-query";
import { hostsAPI } from "@/lib/api";

/** id → nickname for every host, fetched once and shared by every card that
 *  names a linked host (DNS cards' subtitle). Under the ["hosts"] prefix, so
 *  host mutations that invalidate ["hosts"] refresh it too. */
export function useHostNames(): Map<number, string> {
  const { data } = useQuery({
    queryKey: ["hosts", "names"],
    queryFn: () => hostsAPI.list().then((hs) => hs.map((h) => [h.id, h.nickname] as const)),
    staleTime: 60_000,
  });
  return new Map(data ?? []);
}
