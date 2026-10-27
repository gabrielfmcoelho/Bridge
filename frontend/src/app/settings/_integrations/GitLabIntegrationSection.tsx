"use client";

import type { ReactNode } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import Toggle from "@/components/ui/Toggle";
import SectionCard from "@/components/ui/SectionCard";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import CallbackUrl from "./CallbackUrl";
import TestConnectionButton from "./TestConnectionButton";
import { useIntegrationForm, MASKED } from "./useIntegrationForm";

// One GitLab application, two uses: SSO login and code management (repos,
// MRs). Each has its own switch; the credentials above them are shared.
export default function GitLabIntegrationSection({ ssoActive }: { ssoActive: boolean }) {
  const { t } = useLocale();
  const f = useIntegrationForm("gitlab");
  const ssoEnabled = f.form.auth_gitlab_enabled === "true";
  const codeEnabled = f.form.gitlab_integration_enabled === "true";

  // In-card subsection: a plain SectionCard (header + rule, no second card chrome).
  const subsection = (title: string, hint: string, key: string, on: boolean, ariaLabel: string, body: ReactNode) => (
    <SectionCard
      as="h3"
      variant="plain"
      title={title}
      description={hint}
      controls={<Toggle checked={on} onChange={(v) => f.set(key, v ? "true" : "false")} ariaLabel={ariaLabel} />}
      className="pt-2"
    >
      {on && body}
    </SectionCard>
  );

  return (
    <IntegrationCard form={f} title={t("settings.integrations.gitlab.title")} hint={t("settings.integrations.gitlab.hint")}>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.gitlab.baseUrl")}
          value={f.form.auth_gitlab_base_url ?? "https://gitlab.com"}
          onChange={(e) => f.set("auth_gitlab_base_url", e.target.value)}
          placeholder="https://gitlab.com"
        />
        <Input
          label={t("settings.integrations.gitlab.applicationId")}
          value={f.form.auth_gitlab_client_id ?? ""}
          onChange={(e) => f.set("auth_gitlab_client_id", e.target.value)}
          placeholder="your-app-id"
        />
      </div>
      <SecretInputWithClear form={f} name="auth_gitlab_client_secret" label={t("settings.integrations.clientSecret")} />

      {subsection(
          t("settings.integrations.gitlab.ssoSectionTitle"),
          ssoEnabled
            ? (ssoActive ? t("settings.integrations.gitlab.ssoActiveHint") : t("settings.integrations.gitlab.ssoEnabledHint"))
            : t("settings.integrations.disabled"),
          "auth_gitlab_enabled", ssoEnabled, t("settings.integrations.gitlab.ariaEnableSso"),
          <div className="space-y-3">
            <CallbackUrl label={t("settings.integrations.gitlab.redirectUriHint")} path="/api/auth/oauth/gitlab/callback" />
            <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.gitlab.scopesHint")}</p>
          </div>,
      )}

      {subsection(
          t("settings.integrations.gitlab.codeManagementTitle"),
          t("settings.integrations.gitlab.codeManagementHint"),
          "gitlab_integration_enabled", codeEnabled, t("settings.integrations.gitlab.ariaEnableCodeManagement"),
          <div className="space-y-4">
            <SecretInputWithClear form={f} name="gitlab_code_service_token" label={t("settings.integrations.gitlab.serviceAccessToken")} />
            <p className="text-xs text-[var(--text-muted)] -mt-2">{t("settings.integrations.gitlab.serviceTokenHint")}</p>
            <Input
              label={t("settings.integrations.gitlab.defaultBranch")}
              value={f.form.gitlab_code_default_ref ?? ""}
              onChange={(e) => f.set("gitlab_code_default_ref", e.target.value)}
              placeholder="main"
            />
            {/* Tests what is typed; an untouched (masked) token falls back to the stored one. */}
            <TestConnectionButton
              run={async () => {
                const token = f.form.gitlab_code_service_token ?? "";
                const res = await integrationsAPI.testGitLabCode({
                  base_url: f.form.auth_gitlab_base_url ?? "",
                  token: token === MASKED ? "" : token,
                });
                return {
                  success: res.success,
                  message: res.success
                    ? t("settings.integrations.gitlab.connectedAs", { username: res.username ?? t("settings.integrations.gitlab.unknownUser") })
                    : res.error || t("settings.integrations.connectionFailed"),
                };
              }}
            />
          </div>,
      )}
    </IntegrationCard>
  );
}
