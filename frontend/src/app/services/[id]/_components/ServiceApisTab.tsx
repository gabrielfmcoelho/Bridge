"use client";

import LinkedApisCard from "@/app/atlas/apis/_components/LinkedApisCard";
import type { DNSRecord } from "@/lib/types";

/** A service's APIs. "+" prefills the link, its project and — from its first
 *  DNS record — the base URL and the usual /openapi.json spec URL. */
export default function ServiceApisTab({ serviceId, projectId, dns, canEdit }: {
  serviceId: number;
  projectId?: number | null;
  dns: Pick<DNSRecord, "domain" | "has_https">[];
  canEdit: boolean;
}) {
  const primary = dns[0];
  const baseUrl = primary ? `${primary.has_https ? "https" : "http"}://${primary.domain}` : undefined;
  return (
    <LinkedApisCard
      filter={{ service_id: serviceId }}
      canEdit={canEdit}
      prefill={{
        serviceIds: [serviceId],
        projectIds: projectId ? [projectId] : [],
        baseUrl,
        specUrl: baseUrl ? `${baseUrl}/openapi.json` : undefined,
      }}
    />
  );
}
