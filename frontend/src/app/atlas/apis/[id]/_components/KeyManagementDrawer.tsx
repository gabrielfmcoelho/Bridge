"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiKeysAPI } from "@/lib/api";
import type { ApiCatalog, ApiKeyManagement } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import Drawer from "@/components/ui/Drawer";
import FormFooter from "@/components/ui/FormFooter";
import FormError from "@/components/ui/FormError";
import RadioGroup from "@/components/ui/RadioGroup";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";

const MODES: ApiKeyManagement[] = ["none", "manual", "keycloak"];

// Admin-only: how an API's keys are handled. In keycloak mode the API's base
// URL (its /escopos and /admin/uso) and its scope prefix; the Keycloak
// connection itself is the keycloak_apis integration setting. Bridge's own
// entry has no base URL: its scopes are local.
export default function KeyManagementDrawer({ api, open, onClose }: { api: ApiCatalog; open: boolean; onClose: () => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const [mode, setMode] = useState<ApiKeyManagement>(api.key_management);
  const [baseUrl, setBaseUrl] = useState(api.admin_base_url ?? "");
  const [prefix, setPrefix] = useState(api.scope_prefix ?? "");
  const [error, setError] = useState("");
  const [test, setTest] = useState<{ success: boolean; error?: string; keys?: number; scopes?: number } | null>(null);

  const keycloak = mode === "keycloak";
  const bridge = api.scope_prefix === "bridge";
  const payload = () => ({ key_management: mode, admin_base_url: baseUrl.trim(), scope_prefix: prefix.trim() });

  const save = useMutation({
    mutationFn: () => {
      if (keycloak && !prefix.trim()) throw new Error(t("atlas.apis.keys.scopePrefixRequired"));
      if (keycloak && !bridge && !baseUrl.trim()) throw new Error(t("atlas.apis.keys.baseUrlRequired"));
      return apiKeysAPI.setManagement(api.id, payload());
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-catalog"] });
      flag({ appearance: "success", title: t("atlas.apis.keys.managementSaved") });
      onClose();
    },
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const runTest = useMutation({
    mutationFn: () => apiKeysAPI.testManagement(api.id, payload()),
    onSuccess: setTest,
    onError: (err) => setTest({ success: false, error: err instanceof Error ? err.message : String(err) }),
  });

  // Saved settings only: the server reads the stored base URL and prefix.
  const syncScopes = useMutation({
    mutationFn: () => apiKeysAPI.syncScopes(api.id),
    onSuccess: (r) => flag({ appearance: "success", title: t("atlas.apis.keys.scopesSynced", { created: String(r.created), total: String(r.total) }) }),
    onError: (err) => flag({ appearance: "error", title: t("atlas.apis.keys.syncScopesFailed"), description: err instanceof Error ? err.message : undefined }),
  });

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("atlas.apis.keys.managementTitle")}
      footer={<FormFooter onCancel={onClose} submitLabel={t("common.save")} onSubmit={() => { setError(""); save.mutate(); }} loading={save.isPending} />}
    >
      <div className="space-y-5">
        <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.keys.managementIntro")}</p>
        <RadioGroup
          label={t("atlas.apis.keys.mode")}
          name="key-management"
          value={mode}
          onChange={(v) => { setMode(v as ApiKeyManagement); setTest(null); }}
          options={MODES.map((m) => ({ value: m, label: t(`atlas.apis.keys.modes.${m}`) }))}
        />
        <p className="text-xs text-[var(--text-muted)]">{t(`atlas.apis.keys.modeHint.${mode}`)}</p>

        {keycloak && (
          <div className="space-y-4">
            {!bridge && (
              <Input
                label={t("atlas.apis.keys.adminBaseUrl")}
                hint={t("atlas.apis.keys.adminBaseUrlHint")}
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder="http://10.0.122.91:8000"
                required
              />
            )}
            <Input
              label={t("atlas.apis.keys.scopePrefix")}
              hint={t("atlas.apis.keys.scopePrefixHint")}
              value={prefix}
              onChange={(e) => setPrefix(e.target.value)}
              placeholder="servidores"
              disabled={bridge}
              required
            />
            <p className="text-xs text-[var(--text-muted)]">{t("atlas.apis.keys.keycloakSettingsHint")}</p>
            <div className="flex flex-wrap items-center gap-3">
              <Button size="sm" variant="secondary" onClick={() => runTest.mutate()} loading={runTest.isPending} disabled={!prefix.trim() || (!bridge && !baseUrl.trim())}>
                {t("atlas.apis.keys.testConnection")}
              </Button>
              {api.key_management === "keycloak" && (
                <Button size="sm" variant="secondary" onClick={() => syncScopes.mutate()} loading={syncScopes.isPending} title={t("atlas.apis.keys.syncScopesHint")}>
                  {t("atlas.apis.keys.syncScopes")}
                </Button>
              )}
            </div>
            {test && (
              <StatusAlert variant={test.success ? "success" : "error"}>
                {test.success
                  ? t("atlas.apis.keys.testOk", { count: String(test.keys ?? 0), scopes: String(test.scopes ?? 0) })
                  : test.error || t("atlas.apis.keys.testFailed")}
              </StatusAlert>
            )}
          </div>
        )}
        <FormError message={error} />
      </div>
    </Drawer>
  );
}
