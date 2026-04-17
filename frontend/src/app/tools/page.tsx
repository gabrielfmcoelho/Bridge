"use client";

import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { toolsAPI, servicesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import type { ExternalTool, ServiceCredential } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Checkbox from "@/components/ui/Checkbox";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import PageHeader from "@/components/ui/PageHeader";
import FormError from "@/components/ui/FormError";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import Drawer from "@/components/ui/Drawer";

const TOOL_ICONS: Record<string, string> = {
  outline: "M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253",
  gitlab: "M22 13.5L12 22 2 13.5 4.5 3l3.5 9h8l3.5-9L22 13.5z",
  signoz: "M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z",
  metabase: "M3 3v18h18V3H3zm4 14H5v-4h2v4zm4 0H9V7h2v10zm4 0h-2v-6h2v6zm4 0h-2V5h2v12z",
  default: "M11.42 15.17L17.25 21A2.652 2.652 0 0021 17.25l-5.877-5.877M11.42 15.17l2.496-3.03c.317-.384.74-.626 1.208-.766M11.42 15.17l-4.655 5.653a2.548 2.548 0 11-3.586-3.586l6.837-5.63m5.108-.233c.55-.164 1.163-.188 1.743-.14a4.5 4.5 0 004.486-6.336l-3.276 3.277a3.004 3.004 0 01-2.25-2.25l3.276-3.276a4.5 4.5 0 00-6.336 4.486c.049.58.025 1.193-.14 1.743",
};

function getIconPath(icon: string): string {
  return TOOL_ICONS[icon?.toLowerCase()] || TOOL_ICONS.default;
}

function getEmbedHintKey(url: string): string | null {
  const lower = url.toLowerCase();

  if (lower.includes("minio")) {
    return "tool.embedLikelyBlockedMinio";
  }

  if (lower.includes("airflow")) {
    return "tool.embedLikelyUnstableAirflow";
  }

  return null;
}

export default function ToolsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const isAdmin = user?.role === "admin";
  const [showForm, setShowForm] = useState(false);
  const [showSync, setShowSync] = useState(false);
  const [editTool, setEditTool] = useState<ExternalTool | null>(null);
  const [embedTool, setEmbedTool] = useState<ExternalTool | null>(null);
  const [credsTool, setCredsTool] = useState<ExternalTool | null>(null);
  const [embedBlocked, setEmbedBlocked] = useState(false);
  const [embedLoading, setEmbedLoading] = useState(false);
  const [embedReloadKey, setEmbedReloadKey] = useState(0);
  const [embedErrorKey, setEmbedErrorKey] = useState<string | null>(null);
  const embedLoadedRef = useRef(false);

  const primeEmbedState = (tool: ExternalTool) => {
    embedLoadedRef.current = false;
    setEmbedLoading(true);
    setEmbedBlocked(false);
    setEmbedErrorKey(getEmbedHintKey(tool.url));
  };

  const openEmbedTool = (tool: ExternalTool) => {
    setEmbedReloadKey(0);
    primeEmbedState(tool);
    setEmbedTool(tool);
  };

  const reloadEmbeddedTool = () => {
    if (!embedTool) {
      return;
    }
    primeEmbedState(embedTool);
    setEmbedReloadKey((v) => v + 1);
  };

  useEffect(() => {
    if (!embedTool?.url) {
      return;
    }

    const timeout = setTimeout(() => {
      if (!embedLoadedRef.current) {
        setEmbedLoading(false);
        setEmbedBlocked(true);
      }
    }, 12000);

    return () => clearTimeout(timeout);
  }, [embedTool?.id, embedTool?.url, embedReloadKey]);

  const handleEmbedLoaded = () => {
    embedLoadedRef.current = true;
    setEmbedLoading(false);
    setEmbedBlocked(false);
  };

  const handleEmbedFailed = () => {
    setEmbedLoading(false);
    setEmbedBlocked(true);
  };

  const { data: tools = [], isLoading } = useQuery({
    queryKey: ["tools"],
    queryFn: toolsAPI.list,
  });

  const toolList = Array.isArray(tools) ? tools : [];

  const deleteMutation = useMutation({
    mutationFn: (id: number) => toolsAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["tools"] }),
  });

  const unsyncMutation = useMutation({
    mutationFn: (id: number) => toolsAPI.unsyncService(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["tools"] }),
  });

  // Embed view with optional credentials button
  if (embedTool) {
    return (
      <PageShell>
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <button
              onClick={() => {
                setEmbedTool(null);
                setEmbedReloadKey(0);
                setEmbedErrorKey(null);
              }}
              className="inline-flex items-center gap-1.5 text-sm text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
              </svg>
              {t("common.back")}
            </button>
            <h1 className="text-lg font-bold" style={{ fontFamily: "var(--font-display)" }}>{embedTool.name}</h1>
            {embedTool.source === "service" && (
              <Badge color="cyan">{t("tool.synced")}</Badge>
            )}
          </div>
          <div className="flex items-center gap-3">
            {embedTool.has_credentials && (
              <Button size="sm" variant="secondary" onClick={() => setCredsTool(embedTool)}>
                <svg className="w-3.5 h-3.5 mr-1.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
                </svg>
                {t("tool.viewCredentials")}
              </Button>
            )}
            <a
              href={embedTool.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
            >
              {t("tool.openExternal")}
            </a>
          </div>
        </div>
        <div className="rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-hidden bg-white" style={{ height: "calc(100vh - 160px)" }}>
          <div className="relative w-full h-full">
            {(embedLoading || embedBlocked) && (
              <div className="absolute inset-0 z-10 bg-[var(--bg-surface)]/95 backdrop-blur-sm p-4 flex items-center justify-center">
                <div className="max-w-xl w-full rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-4 space-y-3">
                  {embedLoading ? (
                    <p className="text-sm text-[var(--text-secondary)]">{t("tool.embedLoading")}</p>
                  ) : (
                    <>
                      <p className="text-sm font-semibold text-[var(--text-primary)]">{t("tool.embedBlockedTitle")}</p>
                      <p className="text-sm text-[var(--text-secondary)]">
                        {embedErrorKey ? t(embedErrorKey) : t("tool.embedBlockedDescription")}
                      </p>
                      <div className="flex items-center gap-3 pt-1">
                        <Button size="sm" variant="secondary" onClick={reloadEmbeddedTool}>
                          {t("common.refresh")}
                        </Button>
                        <a
                          href={embedTool.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-xs text-[var(--accent)] hover:underline"
                        >
                          {t("tool.openExternal")}
                        </a>
                      </div>
                    </>
                  )}
                </div>
              </div>
            )}

            <iframe
              key={`embed-${embedTool.id}-${embedReloadKey}`}
              src={embedTool.url}
              className="w-full h-full border-0"
              title={embedTool.name}
              sandbox="allow-same-origin allow-scripts allow-popups allow-forms"
              onLoad={handleEmbedLoaded}
              onError={handleEmbedFailed}
            />
          </div>
        </div>

        {/* Credentials drawer from embed view */}
        <CredentialsDrawer tool={credsTool} onClose={() => setCredsTool(null)} />
      </PageShell>
    );
  }

  return (
    <PageShell>
      <div className="flex items-center justify-between mb-6">
        <PageHeader
          title={t("tool.title")}
          addLabel={isAdmin ? t("tool.addTool") : undefined}
          onAdd={isAdmin ? () => setShowForm(true) : undefined}
        />
        {isAdmin && (
          <Button variant="secondary" size="sm" onClick={() => setShowSync(true)}>
            {t("tool.syncFromService")}
          </Button>
        )}
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : toolList.length === 0 ? (
        <EmptyState
          icon="box"
          title={t("common.noResults")}
          description={t("tool.noTools")}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {toolList.map((tool, i) => (
            <div key={tool.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <Card className="h-full">
                <div className="flex items-start gap-3">
                  <div className="w-10 h-10 rounded-[var(--radius-md)] bg-[var(--accent-muted)] border border-[var(--accent)]/20 flex items-center justify-center shrink-0">
                    <svg className="w-5 h-5 text-[var(--accent)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d={getIconPath(tool.icon)} />
                    </svg>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-[var(--text-primary)] text-sm">{tool.name}</h3>
                      {tool.source === "service" && (
                        <Badge color="cyan" className="text-[10px]">{t("tool.synced")}</Badge>
                      )}
                    </div>
                    <p className="text-xs text-[var(--text-muted)] mt-0.5 line-clamp-2">{tool.description || "-"}</p>
                  </div>
                </div>

                <div className="flex items-center gap-2 mt-4 pt-3 border-t border-[var(--border-subtle)]">
                  {tool.url && (
                    <a
                      href={tool.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-[var(--accent)] hover:underline"
                    >
                      {t("tool.openExternal")}
                    </a>
                  )}
                  {tool.has_credentials && (
                    <button
                      onClick={() => setCredsTool(tool)}
                      className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors"
                    >
                      {t("tool.viewCredentials")}
                    </button>
                  )}
                  {tool.embed_enabled && tool.url && (
                    <button
                      onClick={() => openEmbedTool(tool)}
                      className="text-xs text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors ml-auto"
                    >
                      {t("tool.embed")}
                    </button>
                  )}
                  {isAdmin && (
                    <div className="flex items-center gap-2 ml-auto">
                      {tool.source === "service" && (
                        <button
                          onClick={() => { if (confirm("Unsync this tool?")) unsyncMutation.mutate(tool.id); }}
                          className="text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors"
                        >
                          {t("tool.unsync")}
                        </button>
                      )}
                      <button
                        onClick={() => setEditTool(tool)}
                        className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)] transition-colors"
                      >
                        {t("common.edit")}
                      </button>
                    </div>
                  )}
                </div>
              </Card>
            </div>
          ))}
        </div>
      )}

      {/* Create manual tool modal */}
      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={t("tool.addTool")}>
        <ToolForm
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["tools"] });
          }}
        />
      </ResponsiveModal>

      {/* Edit tool modal */}
      <ResponsiveModal open={!!editTool} onClose={() => setEditTool(null)} title={t("common.edit")}>
        {editTool && (
          <ToolForm
            tool={editTool}
            onSuccess={() => {
              setEditTool(null);
              queryClient.invalidateQueries({ queryKey: ["tools"] });
            }}
            onDelete={() => {
              deleteMutation.mutate(editTool.id);
              setEditTool(null);
            }}
          />
        )}
      </ResponsiveModal>

      {/* Sync from service modal */}
      <ResponsiveModal open={showSync} onClose={() => setShowSync(false)} title={t("tool.syncFromService")}>
        <SyncForm
          onSuccess={() => {
            setShowSync(false);
            queryClient.invalidateQueries({ queryKey: ["tools"] });
          }}
        />
      </ResponsiveModal>

      {/* Credentials modal */}
      <CredentialsModal tool={credsTool} onClose={() => setCredsTool(null)} />
    </PageShell>
  );
}

// --- Manual tool form ---

function ToolForm({ tool, onSuccess, onDelete }: {
  tool?: ExternalTool;
  onSuccess: () => void;
  onDelete?: () => void;
}) {
  const { t } = useLocale();
  const isSynced = tool?.source === "service";
  const [form, setForm] = useState({
    name: tool?.name || "",
    description: tool?.description || "",
    url: tool?.url || "",
    icon: tool?.icon || "",
    embed_enabled: tool?.embed_enabled || false,
    sort_order: tool?.sort_order || 0,
  });
  const [error, setError] = useState("");

  const mutation = useMutation({
    mutationFn: () => {
      if (tool) return toolsAPI.update(tool.id, form);
      return toolsAPI.create(form);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  return (
    <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
      <FormError message={error} />
      {isSynced && (
        <div className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-md)] p-3 border border-[var(--border-subtle)]">
          {t("tool.syncedFrom")}: <span className="font-medium text-[var(--text-secondary)]">{tool.name}</span>
        </div>
      )}
      <Input label={t("tool.name")} value={form.name} onChange={(e) => set("name", e.target.value)} required autoFocus disabled={isSynced} />
      <Input label={t("common.description")} value={form.description} onChange={(e) => set("description", e.target.value)} disabled={isSynced} />
      <Input label="URL" value={form.url} onChange={(e) => set("url", e.target.value)} placeholder="https://..." disabled={isSynced} />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Input label={t("tool.icon")} value={form.icon} onChange={(e) => set("icon", e.target.value)} placeholder="outline, gitlab, signoz, metabase" />
        <Input label={t("tool.sortOrder")} type="number" value={form.sort_order.toString()} onChange={(e) => set("sort_order", parseInt(e.target.value) || 0)} />
      </div>
      <Checkbox label={t("tool.enableEmbed")} checked={form.embed_enabled} onChange={(v) => set("embed_enabled", v)} />

      <div className="flex items-center justify-between pt-2">
        {onDelete ? (
          <button
            type="button"
            onClick={() => { if (confirm("Delete this tool?")) onDelete(); }}
            className="text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors"
          >
            {t("common.delete")}
          </button>
        ) : <div />}
        <Button type="submit" loading={mutation.isPending}>
          {tool ? t("common.save") : t("common.create")}
        </Button>
      </div>
    </form>
  );
}

// --- Sync from service form ---

function SyncForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useLocale();
  const [serviceId, setServiceId] = useState("");
  const [dnsId, setDnsId] = useState("");
  const [embedEnabled, setEmbedEnabled] = useState(false);
  const [icon, setIcon] = useState("");
  const [sortOrder, setSortOrder] = useState(0);
  const [error, setError] = useState("");

  const { data: services = [] } = useQuery({
    queryKey: ["services"],
    queryFn: servicesAPI.list,
  });

  // Fetch DNS IDs for the selected service
  const { data: serviceDetail } = useQuery({
    queryKey: ["service-detail", serviceId],
    queryFn: () => servicesAPI.get(Number(serviceId)),
    enabled: !!serviceId,
  });

  const { data: dnsRecords = [] } = useQuery({
    queryKey: ["dns"],
    queryFn: () => import("@/lib/api").then((m) => m.dnsAPI.list()),
  });

  const linkedDnsIds = serviceDetail?.dns_ids || [];
  const dnsOptions = dnsRecords
    .filter((d) => linkedDnsIds.includes(d.id))
    .map((d) => ({
      value: String(d.id),
      label: `${d.has_https ? "https" : "http"}://${d.domain}`,
    }));

  // Only show services that have DNS links
  const servicesWithDns = services;

  const serviceOptions = servicesWithDns.map((s) => ({
    value: String(s.id),
    label: s.nickname,
  }));

  const mutation = useMutation({
    mutationFn: () =>
      toolsAPI.syncFromService({
        service_id: Number(serviceId),
        dns_id: Number(dnsId),
        embed_enabled: embedEnabled,
        icon,
        sort_order: sortOrder,
      }),
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  return (
    <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
      <FormError message={error} />
      <Select
        label={t("tool.selectService")}
        options={serviceOptions}
        value={serviceId}
        onChange={(e) => { setServiceId(e.target.value); setDnsId(""); }}
      />
      {serviceId && (
        <Select
          label={t("tool.selectDns")}
          options={dnsOptions}
          value={dnsId}
          onChange={(e) => setDnsId(e.target.value)}
        />
      )}
      {serviceId && dnsOptions.length === 0 && serviceDetail && (
        <p className="text-xs text-[var(--text-faint)]">
          This service has no linked DNS records. Link a DNS record to the service first.
        </p>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <Input label={t("tool.icon")} value={icon} onChange={(e) => setIcon(e.target.value)} placeholder="outline, gitlab, signoz, metabase" />
        <Input label={t("tool.sortOrder")} type="number" value={sortOrder.toString()} onChange={(e) => setSortOrder(parseInt(e.target.value) || 0)} />
      </div>
      <Checkbox label={t("tool.enableEmbed")} checked={embedEnabled} onChange={setEmbedEnabled} />

      <div className="flex justify-end pt-2">
        <Button type="submit" loading={mutation.isPending} disabled={!serviceId || !dnsId}>
          {t("tool.syncFromService")}
        </Button>
      </div>
    </form>
  );
}

// --- Credentials modal ---

function CredentialsModal({ tool, onClose }: { tool: ExternalTool | null; onClose: () => void }) {
  const { t } = useLocale();

  const { data: credentials = [] } = useQuery({
    queryKey: ["tool-credentials", tool?.id],
    queryFn: () => toolsAPI.listToolCredentials(tool!.id),
    enabled: !!tool,
  });

  return (
    <ResponsiveModal open={!!tool} onClose={onClose} title={t("tool.credentialsTitle")} subHeader={tool?.name}>
      {credentials.length === 0 ? (
        <p className="text-sm text-[var(--text-muted)] py-4">{t("tool.noCredentials")}</p>
      ) : (
        <div className="space-y-2">
          {credentials.map((cred) => (
            <CredentialRow key={cred.id} toolId={tool!.id} credential={cred} />
          ))}
        </div>
      )}
    </ResponsiveModal>
  );
}

// --- Credentials drawer for embed mode ---

function CredentialsDrawer({ tool, onClose }: { tool: ExternalTool | null; onClose: () => void }) {
  const { t } = useLocale();

  const { data: credentials = [] } = useQuery({
    queryKey: ["tool-credentials", tool?.id],
    queryFn: () => toolsAPI.listToolCredentials(tool!.id),
    enabled: !!tool,
  });

  return (
    <Drawer open={!!tool} onClose={onClose} title={t("tool.credentialsTitle")} subHeader={tool?.name}>
      {credentials.length === 0 ? (
        <p className="text-sm text-[var(--text-muted)] py-4">{t("tool.noCredentials")}</p>
      ) : (
        <div className="space-y-2">
          {credentials.map((cred) => (
            <CredentialRow key={cred.id} toolId={tool!.id} credential={cred} />
          ))}
        </div>
      )}
    </Drawer>
  );
}

function CredentialRow({ toolId, credential }: { toolId: number; credential: ServiceCredential }) {
  const { t } = useLocale();
  const [revealed, setRevealed] = useState(false);
  const [value, setValue] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const reveal = async () => {
    if (revealed) {
      setRevealed(false);
      setValue(null);
      return;
    }
    setLoading(true);
    try {
      const data = await toolsAPI.getToolCredential(toolId, credential.id);
      setValue(data.credentials || "");
      setRevealed(true);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] p-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-[var(--text-primary)]">{credential.role_name}</span>
        <button
          onClick={reveal}
          disabled={loading}
          className="text-xs text-[var(--accent)] hover:underline disabled:opacity-50"
        >
          {loading ? t("common.loading") : revealed ? t("serviceCredentials.hide") : t("serviceCredentials.reveal")}
        </button>
      </div>
      {revealed && value !== null && (
        <pre className="mt-2 text-xs text-[var(--text-secondary)] bg-[var(--bg-surface)] rounded-[var(--radius-sm)] p-2 overflow-x-auto whitespace-pre-wrap break-all border border-[var(--border-subtle)]">
          {value}
        </pre>
      )}
    </div>
  );
}
