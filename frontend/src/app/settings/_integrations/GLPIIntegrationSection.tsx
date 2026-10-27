"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { glpiAPI, type GlpiTokenProfile } from "@/lib/api";
import DropdownCatalogueSection from "@/components/glpi/DropdownCatalogueSection";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import SectionCard from "@/components/ui/SectionCard";
import IntegrationCard from "./IntegrationCard";
import SecretInputWithClear from "./SecretInputWithClear";
import { useIntegrationForm } from "./useIntegrationForm";

export default function GLPIIntegrationSection() {
  const { t } = useLocale();
  const f = useIntegrationForm("glpi");
  const { data: profiles } = useQuery({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    retry: false,
  });

  return (
    <IntegrationCard
      form={f}
      title={t("settings.integrations.glpi.title")}
      hint={t("settings.integrations.glpi.hint")}
      enabledKey="glpi_enabled"
      toggleLabel={t("settings.integrations.glpi.ariaEnableIntegration")}
    >
      <Input
        label={t("settings.integrations.baseUrl")}
        value={f.form.glpi_base_url ?? ""}
        onChange={(e) => f.set("glpi_base_url", e.target.value)}
        placeholder="https://glpi.example.org"
      />
      <SecretInputWithClear form={f} name="glpi_app_token" label={t("settings.integrations.glpi.appToken")} />
      <p className="text-xs text-[var(--text-muted)] -mt-2">{t("settings.integrations.glpi.appTokenHint")}</p>
      <Input
        label={t("settings.integrations.glpi.defaultEntityIdFallback")}
        type="number"
        value={f.form.glpi_default_entity_id ?? "0"}
        onChange={(e) => f.set("glpi_default_entity_id", e.target.value)}
      />

      <DropdownCatalogueSection />

      {/* Profiles save on their own (per-row API), not through the card's Save. */}
      <SectionCard as="h3" variant="plain" title={t("settings.integrations.glpi.tokenProfilesTitle")} count={profiles?.length ?? 0} className="pt-2">
        <GlpiProfileList profiles={profiles ?? []} />
      </SectionCard>
    </IntegrationCard>
  );
}

// GlpiProfileList — inline CRUD for GLPI user-token profiles. Lives inside the
// GLPI integration card. Each profile is one named GLPI account (user-token).
function GlpiProfileList({ profiles }: { profiles: GlpiTokenProfile[] }) {
  const confirm = useConfirm();
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [newToken, setNewToken] = useState("");
  const [newEntity, setNewEntity] = useState("0");
  const [error, setError] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<number, { ok: boolean; message: string }>>({});

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["glpi-profiles"] });

  const createMutation = useMutation({
    mutationFn: () => glpiAPI.createProfile({
      name: newName.trim(),
      description: newDesc.trim(),
      user_token: newToken.trim(),
      default_entity_id: parseInt(newEntity || "0", 10),
    }),
    onSuccess: () => {
      setNewName(""); setNewDesc(""); setNewToken(""); setNewEntity("0");
      setAdding(false); setError(null);
      invalidate();
    },
    onError: (err: Error) => setError(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => glpiAPI.deleteProfile(id),
    onSuccess: invalidate,
  });

  const testMutation = useMutation({
    mutationFn: (id: number) => glpiAPI.testProfile(id).then((res) => ({ id, res })),
    onSuccess: ({ id, res }) => {
      setTestResults((prev) => ({
        ...prev,
        [id]: {
          ok: res.success,
          message: res.success
            ? t("settings.integrations.glpi.connectedProfiles", { profiles: res.profiles?.slice(0, 3).join(", ") || t("settings.integrations.glpi.noProfilesFallback") })
            : (res.error || t("settings.integrations.testFailed")),
        },
      }));
    },
  });

  return (
    <div className="space-y-3">
      {profiles.length === 0 && !adding && (
        <p className="text-xs text-[var(--text-muted)]">{t("settings.integrations.glpi.emptyProfiles")}</p>
      )}

      {profiles.map((p) => (
        <div key={p.id} className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] px-3 py-2">
          <div className="flex items-center justify-between gap-3 flex-wrap">
            <div className="min-w-0">
              <p className="text-sm font-medium text-[var(--text-primary)]">{p.name}</p>
              {p.description && <p className="text-xs text-[var(--text-muted)]">{p.description}</p>}
              <p className="text-2xs text-[var(--text-muted)] mt-0.5">
                {t("settings.integrations.glpi.tokenStatusLine", {
                  status: p.has_token ? t("settings.integrations.glpi.tokenStored") : t("settings.integrations.glpi.noTokenStored"),
                  id: String(p.default_entity_id),
                })}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={testMutation.isPending}
                onClick={() => testMutation.mutate(p.id)}
              >
                {t("settings.integrations.glpi.testButton")}
              </Button>
              <Button
                type="button"
                variant="danger"
                size="sm"
                onClick={async () => {
                  if (!(await confirm({ title: t("confirm.deleteTitle", { name: `"${p.name}"` }), message: t("settings.integrations.glpi.confirmDeleteProfile", { name: p.name }), danger: true, confirmLabel: t("common.delete") }))) return;
                  deleteMutation.mutate(p.id);
                }}
              >
                {t("common.delete")}
              </Button>
            </div>
          </div>
          {testResults[p.id] && (
            <p className={`text-xs mt-1 ${testResults[p.id].ok ? "text-[var(--success)]" : "text-[var(--danger)]"}`}>
              {testResults[p.id].message}
            </p>
          )}
        </div>
      ))}

      {adding ? (
        <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] p-3 space-y-3">
          <Input label={t("common.name")} value={newName} onChange={(e) => setNewName(e.target.value)} placeholder={t("settings.integrations.glpi.profileNamePlaceholder")} />
          <Input label={t("settings.integrations.glpi.profileDescLabel")} value={newDesc} onChange={(e) => setNewDesc(e.target.value)} />
          <Input label={t("settings.integrations.glpi.userTokenLabel")} type="password" value={newToken} onChange={(e) => setNewToken(e.target.value)} placeholder={t("settings.integrations.glpi.userTokenPlaceholder")} />
          <Input label={t("settings.integrations.glpi.entityIdLabel")} type="number" value={newEntity} onChange={(e) => setNewEntity(e.target.value)} />
          {error && <FormError message={error} />}
          <div className="flex gap-2">
            <Button type="button" size="sm" onClick={() => createMutation.mutate()} loading={createMutation.isPending} disabled={!newName.trim() || !newToken.trim()}>
              {t("common.add")}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => { setAdding(false); setError(null); }}>
              {t("common.cancel")}
            </Button>
          </div>
        </div>
      ) : (
        <Button type="button" size="sm" variant="secondary" onClick={() => setAdding(true)}>
          {t("settings.integrations.glpi.addProfileButton")}
        </Button>
      )}
    </div>
  );
}
