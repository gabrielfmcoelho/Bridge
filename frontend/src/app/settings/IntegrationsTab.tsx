"use client";

import { useQuery } from "@tanstack/react-query";
import { integrationsAPI } from "@/lib/api";
import GeneralAuthSection from "./integrations/GeneralAuthSection";
import LDAPSection from "./integrations/LDAPSection";
import KeycloakSection from "./integrations/KeycloakSection";
import GitLabIntegrationSection from "./integrations/GitLabIntegrationSection";
import OutlineIntegrationSection from "./integrations/OutlineIntegrationSection";
import GLPIIntegrationSection from "./integrations/GLPIIntegrationSection";
import GrafanaIntegrationSection from "./integrations/GrafanaIntegrationSection";
import LLMSection from "./integrations/LLMSection";
import CoolifySection from "./integrations/CoolifySection";
import ProxmoxSection from "./integrations/ProxmoxSection";

export default function IntegrationsTab() {
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const activeProvider = data?.general?.auth_active_provider ?? "local";

  return (
    <div className="space-y-6">
      <GeneralAuthSection />
      {activeProvider === "ldap" && <LDAPSection />}
      {activeProvider === "keycloak" && <KeycloakSection />}
      <GitLabIntegrationSection ssoActive={activeProvider === "gitlab"} />
      <GrafanaIntegrationSection />
      <OutlineIntegrationSection />
      <GLPIIntegrationSection />
      <LLMSection />
      <CoolifySection />
      <ProxmoxSection />
    </div>
  );
}
