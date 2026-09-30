"use client";

import { useQuery } from "@tanstack/react-query";
import { projectsAPI, servicesAPI } from "@/lib/api";
import { serviceTitle } from "@/lib/serviceDisplay";
import type { ApiCatalog } from "@/lib/types";

/** id → name of the projects and services APIs link to. Reads the inventory
 *  lists the other pages already cache (same query keys). */
export function useApiLinkNames() {
  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list, staleTime: 60_000 });
  const { data: services = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list, staleTime: 60_000 });
  return {
    project: new Map(projects.map((p) => [p.id, p.name])),
    service: new Map(services.map((s) => [s.id, serviceTitle(s).title])),
  };
}

/** "Portal, SGI +2" — linked names for one line, or "" when none. */
export function linkedNames(ids: number[] | undefined, names: Map<number, string>, max = 2): string {
  const list = (ids ?? []).map((id) => names.get(id) ?? `#${id}`);
  return list.length > max ? `${list.slice(0, max).join(", ")} +${list.length - max}` : list.join(", ");
}

/** The host of an API's base URL (what Test Request calls), or "". */
export function baseHost(api: Pick<ApiCatalog, "base_url" | "external_url">): string {
  const raw = api.base_url || api.external_url;
  if (!raw) return "";
  try {
    return new URL(raw).host;
  } catch {
    return raw;
  }
}

const METHOD_TONE: Record<string, string> = {
  GET: "text-[var(--success)] border-[var(--success)]/30 bg-[var(--success)]/10",
  POST: "text-[var(--cyan)] border-[var(--cyan)]/30 bg-[var(--cyan)]/10",
  PUT: "text-[var(--warning)] border-[var(--warning)]/30 bg-[var(--warning)]/10",
  PATCH: "text-[var(--warning)] border-[var(--warning)]/30 bg-[var(--warning)]/10",
  DELETE: "text-[var(--rose)] border-[var(--rose)]/30 bg-[var(--rose)]/10",
};

/** An HTTP method as a fixed-width chip. */
export function MethodBadge({ method }: { method: string }) {
  const m = method.toUpperCase();
  const tone = METHOD_TONE[m] ?? "text-[var(--text-secondary)] border-[var(--border-default)] bg-[var(--bg-overlay)]";
  return <span className={`shrink-0 min-w-[3.25rem] text-center text-2xs font-bold font-mono px-1.5 py-0.5 rounded-[var(--radius-sm)] border ${tone}`}>{m}</span>;
}
