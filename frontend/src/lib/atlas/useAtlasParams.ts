"use client";

import { useCallback, useMemo } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import type { AtlasFilters, TableRole } from "./types";

/**
 * Centralized URL state for the Atlas pages (catalog + pipeline).
 *
 * Query keys:
 *   ?view=<mode>                      view-mode toggle, page-specific values
 *   ?domain=<csv>                     domain filter
 *   ?layer=<csv>                      layer filter
 *   ?role=source|built                role filter (catalog only)
 *   ?q=<string>                       search text
 *   ?selected=<node-id>               table or pipeline-node id
 *   ?col=<column-id>                  drilled-into column inside a selected table
 */
export function useAtlasParams<View extends string>(defaultView: View) {
  const router = useRouter();
  const pathname = usePathname();
  const sp = useSearchParams();

  const view = (sp.get("view") as View | null) ?? defaultView;
  const selected = sp.get("selected");
  const col = sp.get("col");
  const filters = useMemo<AtlasFilters>(() => ({
    domains: parseCsv(sp.get("domain")),
    layers: parseCsv(sp.get("layer")),
    role: (sp.get("role") as TableRole | "all" | null) ?? "all",
    query: sp.get("q") ?? "",
  }), [sp]);

  const patch = useCallback((updates: Record<string, string | string[] | null | undefined>) => {
    const next = new URLSearchParams(sp.toString());
    for (const [k, v] of Object.entries(updates)) {
      if (v === null || v === undefined || v === "" || (Array.isArray(v) && v.length === 0)) {
        next.delete(k);
      } else if (Array.isArray(v)) {
        next.set(k, v.join(","));
      } else {
        next.set(k, v);
      }
    }
    const qs = next.toString();
    const url = qs ? `${pathname}?${qs}` : pathname;
    // history.replaceState keeps state local — no RSC fetch, fully client.
    if (typeof window !== "undefined") {
      window.history.replaceState(null, "", url);
      router.replace(url, { scroll: false });
    }
  }, [pathname, router, sp]);

  return {
    view,
    setView: (v: View) => patch({ view: v === defaultView ? null : v }),
    filters,
    setFilters: (next: Partial<AtlasFilters>) => patch({
      domain: next.domains,
      layer: next.layers,
      role: next.role && next.role !== "all" ? next.role : null,
      q: next.query,
    }),
    selected,
    setSelected: (id: string | null) => patch({ selected: id, col: null }),
    col,
    setCol: (id: string | null) => patch({ col: id }),
  };
}

function parseCsv(v: string | null): string[] {
  if (!v) return [];
  return v.split(",").map(s => s.trim()).filter(Boolean);
}
