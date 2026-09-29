import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { dnsAPI, hostsAPI, projectsAPI, servicesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { serviceTitle } from "@/lib/serviceDisplay";
import type { RelationOption } from "./RelationPicker";

type Kind = "hosts" | "services" | "dns" | "projects";

/**
 * RelationPicker options for the lists a form links to, with the context that
 * tells same-named items apart: a host's address, where a service runs and
 * what it is. Reads the inventory queries the list pages already cache; only
 * the kinds asked for are fetched.
 */
export function useRelationOptions(kinds: Kind[]): Record<Kind, RelationOption[]> {
  const { t } = useLocale();
  const want = (k: Kind) => kinds.includes(k);
  // Service context needs host names, so hosts load whenever services do.
  const { data: hosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list(), enabled: want("hosts") || want("services") });
  const { data: services = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list, enabled: want("services") });
  const { data: dns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list, enabled: want("dns") });
  const { data: projects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list, enabled: want("projects") });

  return useMemo(() => {
    const hostName = new Map(hosts.map((h) => [h.id, h.nickname]));
    return {
      hosts: hosts.map((h) => ({ id: h.id, label: h.nickname, secondary: h.hostname || h.oficial_slug })),
      services: services.map((s) => {
        const where = (s.host_ids ?? []).map((id) => hostName.get(id)).filter(Boolean).join(", ");
        const kind = s.service_kind ? t(`service.kind.${s.service_kind}`) : "";
        return { id: s.id, label: serviceTitle(s).title, secondary: [where, kind].filter(Boolean).join(" · ") || undefined };
      }),
      dns: dns.map((d) => ({ id: d.id, label: d.domain, mono: true })),
      projects: projects.map((p) => ({ id: p.id, label: p.name })),
    };
  }, [hosts, services, dns, projects, t]);
}
