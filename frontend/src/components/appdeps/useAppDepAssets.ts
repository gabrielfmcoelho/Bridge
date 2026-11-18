import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { dnsAPI, hostsAPI, projectsAPI, servicesAPI } from "@/lib/api";
import type { AppDepEntityType, DNSRecord, Host, Project, Service } from "@/lib/types";

export interface AppDepAsset {
  id: number;
  label: string;
  href?: string;
}

/** The hosts, services, DNS records and projetos the caller can see (live
 *  ones: what the lists return), to name the ends of application links and
 *  to pick a new one. Fetched only while enabled. */
export function useAppDepAssets(enabled: boolean) {
  const opts = { enabled, staleTime: 60_000 };
  const hosts = useQuery({ queryKey: ["app-dep-assets", "host"], queryFn: () => hostsAPI.list(), ...opts });
  const services = useQuery({ queryKey: ["app-dep-assets", "service"], queryFn: () => servicesAPI.list(), ...opts });
  const dns = useQuery({ queryKey: ["app-dep-assets", "dns"], queryFn: () => dnsAPI.list(), ...opts });
  const projects = useQuery({ queryKey: ["app-dep-assets", "project"], queryFn: () => projectsAPI.list(), ...opts });

  const byType = useMemo<Record<AppDepEntityType, AppDepAsset[]>>(() => ({
    host: (hosts.data ?? []).map((h: Host) => ({ id: h.id, label: h.nickname, href: h.oficial_slug ? `/hosts/${h.oficial_slug}` : undefined })),
    service: (services.data ?? []).map((s: Service) => ({ id: s.id, label: s.nickname, href: `/services/${s.id}` })),
    dns: (dns.data ?? []).map((d: DNSRecord) => ({ id: d.id, label: d.domain, href: `/dns/${d.id}` })),
    project: (projects.data ?? []).map((p: Project) => ({ id: p.id, label: p.name, href: `/projects/${p.id}` })),
  }), [hosts.data, services.data, dns.data, projects.data]);

  return {
    hosts: hosts.data ?? [],
    services: services.data ?? [],
    dns: dns.data ?? [],
    projects: projects.data ?? [],
    byType,
    loading: enabled && (hosts.isLoading || services.isLoading || dns.isLoading || projects.isLoading),
  };
}
