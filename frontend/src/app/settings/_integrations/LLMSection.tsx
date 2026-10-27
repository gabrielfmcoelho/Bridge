"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { integrationsAPI } from "@/lib/api";
import Input from "@/components/ui/Input";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import TestConnectionButton, { failedLine } from "./TestConnectionButton";
import { useIntegrationForm, MASKED } from "./useIntegrationForm";

export default function LLMSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("llm");

  // Tests what is typed; an untouched (masked) key falls back to the stored one.
  const runTest = async () => {
    const key = f.form.llm_api_key ?? "";
    const res = await integrationsAPI.testLLM({
      base_url: f.form.llm_base_url ?? "",
      api_key: key === MASKED ? "" : key,
      model: f.form.llm_model_text ?? "",
    });
    if (!res.success) {
      const stage = res.stage === "chat" ? t("settings.integrations.chatFailed") : undefined;
      return { success: false, message: failedLine(t, res.error, stage) };
    }
    // Endpoint → model catalog → chat round-trip.
    const parts = [t("settings.integrations.llm.endpointOk", { count: String(res.models_count ?? 0) })];
    if (res.model) {
      parts.push(res.model_available
        ? t("settings.integrations.llm.modelAvailable", { model: res.model })
        : t("settings.integrations.llm.modelNotInList", { model: res.model }));
    }
    if (res.chat_ok) {
      parts.push(res.chat_reply
        ? t("settings.integrations.llm.chatOkWithReply", { reply: res.chat_reply })
        : t("settings.integrations.llm.chatOk"));
    } else if (res.warning) {
      parts.push(res.warning);
    }
    return { success: true, message: parts.join(" · ") };
  };

  return (
    <IntegrationCard
      form={f}
      title={t("settings.integrations.llm.title")}
      enabledKey="llm_enabled"
      toggleLabel={t("settings.integrations.llm.ariaEnableLlm")}
    >
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <Input
          label={t("settings.integrations.llm.apiBaseUrl")}
          value={f.form.llm_base_url ?? ""}
          onChange={(e) => f.set("llm_base_url", e.target.value)}
          placeholder="https://api.sobdemanda.mandu.piaui.pro/v1"
        />
        <SecretInputWithClear form={f} name="llm_api_key" label={t("settings.integrations.llm.apiKey")} />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Input
          label={t("settings.integrations.llm.textModel")}
          value={f.form.llm_model_text ?? "Qwen/Qwen3-30B-A3B"}
          onChange={(e) => f.set("llm_model_text", e.target.value)}
          placeholder="Qwen/Qwen3-30B-A3B"
        />
        <Input
          label={t("settings.integrations.llm.visionModel")}
          value={f.form.llm_model_vision ?? "Qwen/Qwen3-VL-30B-A3B-Thinking"}
          onChange={(e) => f.set("llm_model_vision", e.target.value)}
          placeholder="Qwen/Qwen3-VL-30B-A3B-Thinking"
        />
        <Input
          label={t("settings.integrations.llm.maxTokens")}
          type="number"
          value={f.form.llm_max_tokens ?? "2000"}
          onChange={(e) => f.set("llm_max_tokens", e.target.value)}
          placeholder="2000"
        />
      </div>
      <TestConnectionButton run={runTest} />
    </IntegrationCard>
  );
}
