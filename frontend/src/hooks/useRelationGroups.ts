"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { relationsAPI, hostsAPI, dnsAPI, servicesAPI, projectsAPI, contactsAPI, entidadesAPI } from "@/lib/api";
import { groupItems, type GroupEntity, type ItemGroup } from "@/lib/grouping";
import { useLocale } from "@/contexts/LocaleContext";

// Each related entity's list, reduced to id → display name.
const NAME_SOURCES: Record<GroupEntity, () => Promise<{ id: number; name: string }[]>> = {
  host: () => hostsAPI.list().then((l) => l.map((h) => ({ id: h.id, name: h.nickname }))),
  dns: () => dnsAPI.list().then((l) => l.map((d) => ({ id: d.id, name: d.domain }))),
  service: () => servicesAPI.list().then((l) => l.map((s) => ({ id: s.id, name: s.nickname }))),
  project: () => projectsAPI.list().then((l) => l.map((p) => ({ id: p.id, name: p.name }))),
  contact: () => contactsAPI.list().then((l) => l.map((c) => ({ id: c.id, name: c.name }))),
  entidade: () => entidadesAPI.list().then((l) => l.map((e) => ({ id: e.id, name: e.name }))),
};

/**
 * `items` (of type `entity`) grouped by `by`, or undefined when not grouping
 * or while the links and names load. Nothing is fetched until `by` is set.
 */
export function useRelationGroups<T extends { id: number }>(entity: GroupEntity, by: GroupEntity | "", items: T[]) {
  const { t } = useLocale();
  const relations = useQuery({ queryKey: ["relations"], queryFn: relationsAPI.list, enabled: !!by, staleTime: 30_000 });
  const names = useQuery({
    queryKey: ["group-names", by],
    queryFn: () => NAME_SOURCES[by as GroupEntity](),
    enabled: !!by,
    staleTime: 30_000,
  });

  const groups = useMemo<ItemGroup<T>[] | undefined>(() => {
    if (!by || !relations.data || !names.data) return undefined;
    const nameMap = new Map(names.data.map((n) => [n.id, n.name]));
    return groupItems(items, entity, by, relations.data, nameMap, t(`inventory.unlinked.${by}`));
  }, [by, entity, items, relations.data, names.data, t]);

  return { groups, isLoading: !!by && (relations.isLoading || names.isLoading) };
}
