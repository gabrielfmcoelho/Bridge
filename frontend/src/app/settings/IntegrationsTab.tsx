"use client";

import { useQuery } from "@tanstack/react-query";
import { integrationsAPI } from "@/lib/api";
import GeneralAuthSection from "./_integrations/GeneralAuthSection";
import LDAPSection from "./_integrations/LDAPSection";
import KeycloakSection from "./_integrations/KeycloakSection";
import KeycloakAPIsSection from "./_integrations/KeycloakAPIsSection";
import GitLabIntegrationSection from "./_integrations/GitLabIntegrationSection";
import OutlineIntegrationSection from "./_integrations/OutlineIntegrationSection";
import GLPIIntegrationSection from "./_integrations/GLPIIntegrationSection";
import GrafanaIntegrationSection from "./_integrations/GrafanaIntegrationSection";
import SignozIntegrationSection from "./_integrations/SignozIntegrationSection";
import LLMSection from "./_integrations/LLMSection";
import CoolifySection from "./_integrations/CoolifySection";
import SentinelSection from "./_integrations/SentinelSection";
import ProxmoxSection from "./_integrations/ProxmoxSection";
import WazuhSection from "./_integrations/WazuhSection";
import DependenciesSection from "./_integrations/DependenciesSection";
import SMTPSection from "./_integrations/SMTPSection";

export default function IntegrationsTab() {
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const activeProvider = data?.general?.auth_active_provider ?? "local";

  return (
    <div className="space-y-6">
      <GeneralAuthSection />
      {activeProvider === "ldap" && <LDAPSection />}
      {activeProvider === "keycloak" && <KeycloakSection />}
      <KeycloakAPIsSection />
      <GitLabIntegrationSection ssoActive={activeProvider === "gitlab"} />
      <GrafanaIntegrationSection />
      <SignozIntegrationSection />
      <OutlineIntegrationSection />
      <GLPIIntegrationSection />
      <SMTPSection />
      <LLMSection />
      <CoolifySection />
      <SentinelSection />
      <ProxmoxSection />
      <WazuhSection />
      <DependenciesSection />
    </div>
  );
}
