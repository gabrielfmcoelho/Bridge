"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI, outlineAPI } from "@/lib/api";
import CollectionMultiSelect from "@/components/wiki/CollectionMultiSelect";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm, MASKED } from "./useIntegrationForm";

function parseCollectionCSV(raw: string): string[] {
  return raw
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export default function OutlineIntegrationSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("outline");
  const [fallbackToText, setFallbackToText] = useState(false);

  // The collection picker needs a working saved token to reach Outline, so it
  // loads only once the integration is enabled and configured on the server.
  const saved = f.data?.outline;
  const outlineReady = saved?.outline_enabled === "true" && !!saved?.outline_base_url && saved?.outline_api_token === MASKED;
  const { data: collectionsEnv, isLoading: collectionsLoading, isError: collectionsError } = useQuery({
    queryKey: ["outline-workspace-collections"],
    queryFn: outlineAPI.listWorkspaceCollections,
    enabled: outlineReady,
    retry: false,
    staleTime: 60_000,
  });

  const linkButton = (label: string, onClick: () => void) => (
    <Button type="button" variant="ghost" size="sm" onClick={onClick}>{label}</Button>
  );

  return (
    <IntegrationCard
      form={f}
      title="Outline (wiki)"
      hint={t("settings.integrations.outline.wikiTabHint")}
      enabledKey="outline_enabled"
      toggleLabel={t("settings.integrations.outline.ariaEnableIntegration")}
    >
      <Input
        label={t("settings.integrations.baseUrl")}
        value={f.form.outline_base_url ?? ""}
        onChange={(e) => f.set("outline_base_url", e.target.value)}
        placeholder="https://wiki.example.org"
      />
      <SecretInputWithClear form={f} name="outline_api_token" label={t("settings.integrations.apiToken")} />
      <p className="text-xs text-[var(--text-muted)] -mt-2">{t("settings.integrations.outline.tokenVisibilityHint")}</p>

      {collectionsError || fallbackToText ? (
        <div>
          <Input
            label={t("settings.integrations.outline.commonCollectionIds")}
            value={f.form.outline_common_collection_id ?? ""}
            onChange={(e) => f.set("outline_common_collection_id", e.target.value)}
            placeholder="uuid-1, uuid-2, uuid-3"
          />
          <div className="flex items-center gap-2 flex-wrap mt-1">
            <p className="text-xs text-[var(--text-muted)]">
              {collectionsError
                ? t("settings.integrations.outline.collectionsHintError")
                : t("settings.integrations.outline.collectionsHintManual")}
            </p>
            {!collectionsError && linkButton(t("settings.integrations.outline.switchToPicker"), () => setFallbackToText(false))}
          </div>
        </div>
      ) : (
        <div>
          <CollectionMultiSelect
            label={t("settings.integrations.outline.collectionsLabel")}
            collections={collectionsEnv?.collections ?? []}
            value={parseCollectionCSV(f.form.outline_common_collection_id ?? "")}
            onChange={(ids) => f.set("outline_common_collection_id", ids.join(", "))}
            loading={collectionsLoading}
            emptyHint={
              outlineReady
                ? t("settings.integrations.outline.emptyHintReady")
                : t("settings.integrations.outline.emptyHintNotReady")
            }
          />
          <div className="flex items-center gap-2 flex-wrap mt-1">
            <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.outline.feedsSidebarLong")}</p>
            {linkButton(t("settings.integrations.outline.pasteUuidsInstead"), () => setFallbackToText(true))}
          </div>
        </div>
      )}

      {/* Tests what is typed; an untouched (masked) token falls back to the stored one. */}
      <TestConnectionButton
        run={async () => {
          const token = f.form.outline_api_token ?? "";
          const res = await integrationsAPI.testOutline({
            base_url: f.form.outline_base_url ?? "",
            token: token === MASKED ? "" : token,
          });
          if (!res.success) return { success: false, message: failedLine(t, res.error) };
          const parts: string[] = [];
          if (res.workspace) parts.push(res.workspace);
          if (res.user) parts.push(t("settings.integrations.asUser", { user: res.user }));
          return { success: true, message: parts.join(" · ") || t("settings.integrations.connected") };
        }}
      />
    </IntegrationCard>
  );
}
