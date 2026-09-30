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
import Checkbox from "@/components/ui/Checkbox";

const MODES: ApiKeyManagement[] = ["none", "manual", "sead"];

// Admin-only: how an API's keys are handled, and the SEAD connection. The
// master key and X-API-Key are write-only: the server only says whether they
// are set, so a blank field keeps the stored value.
export default function KeyManagementDrawer({ api, open, onClose }: { api: ApiCatalog; open: boolean; onClose: () => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const [mode, setMode] = useState<ApiKeyManagement>(api.key_management);
  const [baseUrl, setBaseUrl] = useState(api.admin_base_url ?? "");
  const [adminKey, setAdminKey] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [clearApiKey, setClearApiKey] = useState(false);
  const [error, setError] = useState("");
  const [test, setTest] = useState<{ success: boolean; error?: string; keys?: number } | null>(null);

  const sead = mode === "sead";
  const payload = () => ({
    key_management: mode,
    admin_base_url: baseUrl.trim(),
    admin_key: adminKey.trim() || undefined,
    api_key: apiKey.trim() || undefined,
    clear_api_key: clearApiKey && !apiKey.trim(),
  });

  const save = useMutation({
    mutationFn: () => {
      if (sead && !baseUrl.trim()) throw new Error(t("atlas.apis.keys.baseUrlRequired"));
      if (sead && !adminKey.trim() && !api.has_admin_key) throw new Error(t("atlas.apis.keys.masterKeyRequired"));
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

        {sead && (
          <div className="space-y-4">
            <Input
              label={t("atlas.apis.keys.adminBaseUrl")}
              hint={t("atlas.apis.keys.adminBaseUrlHint")}
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="https://api.folha.sead.gov.br/folha"
              required
            />
            <Input
              label={t("atlas.apis.keys.masterKey")}
              hint={api.has_admin_key ? t("atlas.apis.keys.storedKeepHint") : t("atlas.apis.keys.masterKeyHint")}
              type="password"
              autoComplete="off"
              value={adminKey}
              onChange={(e) => setAdminKey(e.target.value)}
              placeholder={api.has_admin_key ? "••••••••" : ""}
              required={!api.has_admin_key}
            />
            <Input
              label={t("atlas.apis.keys.xApiKey")}
              hint={t("atlas.apis.keys.xApiKeyHint")}
              type="password"
              autoComplete="off"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={api.has_api_key ? "••••••••" : ""}
            />
            {api.has_api_key && (
              <Checkbox checked={clearApiKey} onChange={setClearApiKey} label={t("atlas.apis.keys.clearXApiKey")} />
            )}
            <div className="flex items-center gap-3">
              <Button size="sm" variant="secondary" onClick={() => runTest.mutate()} loading={runTest.isPending} disabled={!baseUrl.trim()}>
                {t("atlas.apis.keys.testConnection")}
              </Button>
            </div>
            {test && (
              <StatusAlert variant={test.success ? "success" : "error"}>
                {test.success ? t("atlas.apis.keys.testOk", { count: String(test.keys ?? 0) }) : test.error || t("atlas.apis.keys.testFailed")}
              </StatusAlert>
            )}
          </div>
        )}
        <FormError message={error} />
      </div>
    </Drawer>
  );
}
