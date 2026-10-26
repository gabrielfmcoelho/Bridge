"use client";

import { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { contactsAPI, databasesAPI, type DatabasePayload } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import FormField from "@/components/ui/FormField";
import PillButton from "@/components/ui/PillButton";
import ResponsavelList from "@/components/inventory/ResponsavelList";
import EntidadeScopeFields, { defaultGrants } from "@/components/entidades/EntidadeScopeFields";
import EntityFormShell from "@/components/forms/EntityFormShell";
import FormSection from "@/components/forms/FormSection";
import RelationPicker from "@/components/forms/RelationPicker";
import { useRelationOptions } from "@/components/forms/useRelationOptions";
import type {
  AssetGrantsInput, Database, DatabaseConnectionMode, DatabaseEngine, DatabaseEnvironment, DatabaseSensitivity,
  DatabaseTLSMode, EntityResponsavel,
} from "@/lib/types";
import { ENGINES, ENVIRONMENTS, SENSITIVITIES } from "./_components/databaseInsights";

/** What a create form can start with (a database added from an instance, a service or a project). */
export interface DatabaseFormPrefill {
  instanceServiceId?: number;
  engine?: DatabaseEngine;
  dbName?: string;
  consumerServiceIds?: number[];
  projectIds?: number[];
}

interface DatabaseFormProps {
  /** The detail response (with responsaveis and entidades) on edit. */
  initial?: Database | null;
  prefill?: DatabaseFormPrefill;
  onSuccess: (db: Database) => void;
  onClose?: () => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
  onFooterChange?: (footer: React.ReactNode) => void;
}

const MODES: DatabaseConnectionMode[] = ["none", "direct", "ssh_tunnel"];
const TLS: DatabaseTLSMode[] = ["disable", "prefer", "require", "verify"];

/** Create or edit a catalogued database. Update sends every field (the
 *  endpoint is partial, so this is just the full form state). */
export default function DatabaseForm({ initial, prefill, onSuccess, onClose, onSubHeaderChange, onFooterChange }: DatabaseFormProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isEdit = !!initial;
  const isAdmin = user?.role === "admin";
  const [grants, setGrants] = useState<AssetGrantsInput>(initial?.entidades ?? defaultGrants(user));
  const [name, setName] = useState(initial?.name ?? "");
  const [dbName, setDbName] = useState(initial?.db_name ?? prefill?.dbName ?? "");
  const [engine, setEngine] = useState<DatabaseEngine>(initial?.engine ?? prefill?.engine ?? "postgres");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [useCases, setUseCases] = useState(initial?.use_cases ?? "");
  const [external, setExternal] = useState(initial ? initial.instance_service_id == null : false);
  const [instanceId, setInstanceId] = useState<string>(String(initial?.instance_service_id ?? prefill?.instanceServiceId ?? ""));
  const [provider, setProvider] = useState(initial?.external_provider ?? "");
  const [host, setHost] = useState(initial?.host ?? "");
  const [port, setPort] = useState(initial?.port != null ? String(initial.port) : "");
  const [mode, setMode] = useState<DatabaseConnectionMode>(initial?.connection_mode ?? "none");
  const [tunnelHostId, setTunnelHostId] = useState(String(initial?.tunnel_host_id ?? ""));
  const [tunnelTarget, setTunnelTarget] = useState(initial?.tunnel_target ?? "");
  const [tls, setTls] = useState<DatabaseTLSMode>(initial?.tls_mode ?? "prefer");
  const [sensitivity, setSensitivity] = useState<DatabaseSensitivity>(initial?.sensitivity ?? "interno");
  const [environment, setEnvironment] = useState<DatabaseEnvironment>(initial?.environment ?? "prod");
  const [backupPolicy, setBackupPolicy] = useState(initial?.backup_policy ?? "");
  const [retention, setRetention] = useState(initial?.backup_retention_days != null ? String(initial.backup_retention_days) : "");
  const [lastBackup, setLastBackup] = useState(initial?.last_backup_at?.slice(0, 10) ?? "");
  const [consumerIds, setConsumerIds] = useState<number[]>(initial?.consumer_service_ids ?? prefill?.consumerServiceIds ?? []);
  const [projectIds, setProjectIds] = useState<number[]>(initial?.project_ids ?? prefill?.projectIds ?? []);
  const [responsaveis, setResponsaveis] = useState<EntityResponsavel[]>(initial?.responsaveis ?? []);
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const { data: rawContacts } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const contacts = Array.isArray(rawContacts) ? rawContacts : [];
  const { data: instances = [] } = useQuery({ queryKey: ["database-instances"], queryFn: databasesAPI.instances });
  const relationOptions = useRelationOptions(["services", "projects", "hosts"]);

  const pickInstance = (v: string) => {
    setInstanceId(v);
    const inst = instances.find((i) => String(i.id) === v);
    if (inst && inst.engine !== "other" && !isEdit) setEngine(inst.engine);
  };

  const num = (v: string) => (v.trim() === "" ? null : Number(v));
  const mutation = useMutation({
    mutationFn: () => {
      const body: DatabasePayload = {
        name: name.trim(), db_name: dbName.trim(), engine, description, use_cases: useCases,
        instance_service_id: external ? null : Number(instanceId),
        external_provider: external ? provider.trim() : "",
        connection_mode: mode,
        tunnel_host_id: mode === "ssh_tunnel" ? Number(tunnelHostId) : null,
        tunnel_target: mode === "ssh_tunnel" ? tunnelTarget.trim() : "",
        tls_mode: tls, sensitivity, environment,
        backup_policy: backupPolicy.trim(), backup_retention_days: num(retention),
        last_backup_at: lastBackup ? new Date(`${lastBackup}T00:00:00Z`).toISOString() : null,
        consumer_service_ids: consumerIds, project_ids: projectIds,
        responsaveis: responsaveis.filter((r) => r.name).map((r) => ({ contact_id: r.contact_id, is_main: r.is_main })),
        ...grants,
      };
      // The endpoint is where Bridge connects to: only admins send it.
      if (isAdmin) Object.assign(body, { host: host.trim(), port: num(port) });
      return initial ? databasesAPI.update(initial.id, body) : databasesAPI.create(body);
    },
    onSuccess: (db) => onSuccess(db),
    onError: (err) => setError(err instanceof Error ? err.message : t("form.saveFailed")),
  });

  const errors: Record<string, string> = {};
  if (!name.trim()) errors.name = t("form.required");
  if (!external && !instanceId) errors.instance = t("form.required");
  if (external && !provider.trim() && !host.trim()) errors.provider = t("atlas.databases.providerRequired");
  if (port.trim() && !(Number(port) > 0 && Number(port) < 65536)) errors.port = t("atlas.databases.portInvalid");
  if (retention.trim() && !(Number(retention) >= 0)) errors.retention = t("atlas.databases.retentionInvalid");
  if (mode === "ssh_tunnel" && !tunnelHostId) errors.tunnelHost = t("form.required");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = () => {
    setAttempted(true);
    if (Object.keys(errors).length) return false;
    setError("");
    mutation.mutate();
  };

  const pills = <V extends string>(values: readonly V[], value: V, set: (v: V) => void, label: (v: V) => string) => (
    <div className="flex flex-wrap gap-1.5">
      {values.map((v) => <PillButton key={v} active={value === v} onClick={() => set(v)}>{label(v)}</PillButton>)}
    </div>
  );

  const sections = [
    { id: "db-identity", label: t("form.section.identity") },
    { id: "db-location", label: t("atlas.databases.section.location") },
    { id: "db-connection", label: t("atlas.databases.section.connection") },
    { id: "db-classification", label: t("atlas.databases.section.classification") },
    { id: "db-owners", label: t("form.section.owners") },
    { id: "db-links", label: t("form.section.links") },
    { id: "db-entidades", label: t("entidades.title") },
  ];

  return (
    <EntityFormShell
      id="database-form"
      sections={sections}
      isEdit={isEdit}
      isPending={mutation.isPending}
      submitLabel={isEdit ? t("form.saveChanges") : t("atlas.databases.register")}
      error={error}
      onSubmit={submit}
      onCancel={onClose}
      onFooterChange={onFooterChange}
      onSubHeaderChange={onSubHeaderChange}
    >
      <FormSection id="db-identity" title={t("form.section.identity")}>
        <Input label={t("atlas.databases.name")} value={name} onChange={(e) => setName(e.target.value)} required
          placeholder={t("atlas.databases.namePlaceholder")} error={err("name")} aria-invalid={!!err("name")} />
        <Input label={t("atlas.databases.dbName")} hint={t("atlas.databases.dbNameHint")} value={dbName} onChange={(e) => setDbName(e.target.value)}
          className="font-mono" placeholder="sei_prod" />
        <div className="sm:col-span-2">
          <FormField label={t("atlas.databases.engineLabel")}>
            {pills(ENGINES, engine, setEngine, (v) => t(`atlas.databases.engine.${v}`))}
          </FormField>
        </div>
        <div className="sm:col-span-2">
          <MarkdownEditor label={t("common.description")} value={description} onChange={setDescription} rows={5}
            placeholder={t("atlas.databases.descriptionPlaceholder")} />
        </div>
        <div className="sm:col-span-2">
          <MarkdownEditor label={t("atlas.apis.useCases")} value={useCases} onChange={setUseCases} rows={4}
            placeholder={t("atlas.databases.useCasesPlaceholder")} />
        </div>
      </FormSection>

      <FormSection id="db-location" title={t("atlas.databases.section.location")} description={t("atlas.databases.locationHint")}>
        <div className="sm:col-span-2 flex gap-1.5">
          <PillButton active={!external} onClick={() => setExternal(false)}>{t("atlas.databases.onInstance")}</PillButton>
          <PillButton active={external} onClick={() => setExternal(true)}>{t("atlas.databases.external")}</PillButton>
        </div>
        {external ? (
          <div className="sm:col-span-2">
            <Input label={t("atlas.databases.provider")} hint={t("atlas.databases.providerHint")} value={provider}
              onChange={(e) => setProvider(e.target.value)} error={err("provider")} aria-invalid={!!err("provider")} />
          </div>
        ) : (
          <div className="sm:col-span-2">
            <Select label={t("atlas.databases.instance")} hint={t("atlas.databases.instanceHint")} required value={instanceId}
              onChange={(e) => pickInstance(e.target.value)} error={err("instance")}
              options={[{ value: "", label: t("atlas.databases.pickInstance") }, ...instances.map((i) => ({
                value: String(i.id),
                label: [i.nickname, i.host_nickname, t(`atlas.databases.engine.${i.engine}`)].filter(Boolean).join(" · "),
              }))]} />
          </div>
        )}
        <Input label={t("atlas.databases.host")} hint={isAdmin ? t("atlas.databases.hostHint") : t("atlas.databases.hostAdminOnly")}
          value={host} onChange={(e) => setHost(e.target.value)} disabled={!isAdmin} className="font-mono" placeholder="db.example.gov.br" />
        <Input label={t("atlas.databases.port")} value={port} onChange={(e) => setPort(e.target.value)} disabled={!isAdmin}
          inputMode="numeric" className="font-mono" placeholder="5432" error={err("port")} aria-invalid={!!err("port")} />
      </FormSection>

      <FormSection id="db-connection" title={t("atlas.databases.section.connection")} description={t("atlas.databases.connectionHint")}>
        <div className="sm:col-span-2">
          <FormField label={t("atlas.databases.connectionMode")} hint={t(`atlas.databases.modeHint.${mode}`)}>
            {pills(MODES, mode, setMode, (v) => t(`atlas.databases.mode.${v}`))}
          </FormField>
        </div>
        {mode === "ssh_tunnel" && (
          <>
            <Select label={t("atlas.databases.tunnelHost")} required value={tunnelHostId} onChange={(e) => setTunnelHostId(e.target.value)}
              error={err("tunnelHost")} options={[{ value: "", label: t("atlas.databases.pickHost") },
                ...relationOptions.hosts.map((h) => ({ value: String(h.id), label: h.label }))]} />
            <Input label={t("atlas.databases.tunnelTarget")} hint={t("atlas.databases.tunnelTargetHint")} value={tunnelTarget}
              onChange={(e) => setTunnelTarget(e.target.value)} className="font-mono" placeholder="127.0.0.1" />
          </>
        )}
        {mode !== "none" && (
          <div className="sm:col-span-2">
            <FormField label={t("atlas.databases.tls")}>
              {pills(TLS, tls, setTls, (v) => t(`atlas.databases.tlsMode.${v}`))}
            </FormField>
          </div>
        )}
      </FormSection>

      <FormSection id="db-classification" title={t("atlas.databases.section.classification")}>
        <div className="sm:col-span-2">
          <FormField label={t("atlas.databases.sensitivityLabel")} hint={t(`atlas.databases.sensitivityHint.${sensitivity}`)}>
            {pills(SENSITIVITIES, sensitivity, setSensitivity, (v) => t(`atlas.databases.sensitivity.${v}`))}
          </FormField>
        </div>
        <div className="sm:col-span-2">
          <FormField label={t("atlas.databases.environmentLabel")}>
            {pills(ENVIRONMENTS, environment, setEnvironment, (v) => t(`atlas.databases.environment.${v}`))}
          </FormField>
        </div>
        <div className="sm:col-span-2">
          <Input label={t("atlas.databases.backupPolicy")} hint={t("atlas.databases.backupPolicyHint")} value={backupPolicy}
            onChange={(e) => setBackupPolicy(e.target.value)} placeholder={t("atlas.databases.backupPolicyPlaceholder")} />
        </div>
        <Input label={t("atlas.databases.retentionDays")} value={retention} onChange={(e) => setRetention(e.target.value)}
          inputMode="numeric" error={err("retention")} aria-invalid={!!err("retention")} />
        <Input label={t("atlas.databases.lastBackup")} type="date" value={lastBackup} onChange={(e) => setLastBackup(e.target.value)} />
      </FormSection>

      <FormSection id="db-owners" title={t("form.section.owners")} stack>
        <ResponsavelList value={responsaveis} onChange={setResponsaveis} contacts={contacts} t={t} />
      </FormSection>

      <FormSection id="db-links" title={t("form.section.links")} description={t("atlas.databases.linksHint")} stack>
        <RelationPicker label={t("atlas.databases.consumers")} options={relationOptions.services} selected={consumerIds} onChange={setConsumerIds} />
        <RelationPicker label={t("topology.projects")} options={relationOptions.projects} selected={projectIds} onChange={setProjectIds} />
      </FormSection>

      <FormSection id="db-entidades" title={t("entidades.title")} stack>
        <EntidadeScopeFields value={grants} onChange={setGrants} compact />
      </FormSection>
    </EntityFormShell>
  );
}
