"use client";

import { useState, useCallback, useEffect, useMemo, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { sshAPI, hostsAPI, sshKeysAPI, integrationsAPI } from "@/lib/api";
import { resolveAuthMethod } from "@/lib/utils";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useSSHMutation } from "@/hooks/useSSHMutation";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import IconButton from "@/components/ui/IconButton";
import Drawer from "@/components/ui/Drawer";
import StatusAlert from "@/components/ui/StatusAlert";
import OperationOutput from "@/components/ui/OperationOutput";
import VMInfoDisplay from "./VMInfoDisplay";
import IntegrationsSection from "./IntegrationsSection";
import type { VMInfoType, OperationLog, RemoteKeyInfo, DockerStatusType, NginxCleanupStatusType, RemoteUserInfo, NetworkTestResult } from "@/lib/api";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

type ConsoleEntry = {
  label: string;
  status: "success" | "error" | "warning" | "loading";
  content: ReactNode;
  timestamp: number;
};

export default function SSHOperations({ slug, hasPassword, hasKey, preferredAuth, passwordTestStatus, keyTestStatus, dockerGroupStatus, coolifyServerUUID, serverInfo, lastScan, t, locale, isAdmin }: {
  slug: string; hasPassword: boolean; hasKey: boolean;
  preferredAuth?: "password" | "key" | "";
  passwordTestStatus?: "success" | "failed" | null;
  keyTestStatus?: "success" | "failed" | null;
  dockerGroupStatus?: "ok" | "fixed" | "needs_sudo" | "needs_relogin" | "not_installed" | "failed" | null;
  coolifyServerUUID?: string | null;
  serverInfo?: { hostname: string; is_local: boolean; message: string } | null;
  lastScan?: { data: string; scanned_at: string };
  t: (key: string, vars?: Record<string, string>) => string;
  locale: string;
  isAdmin: boolean;
}) {
  // Derive the list of non-system users discovered by the most recent scan
  // (see sshtest.captureVMInfo → RemoteUsers). Used by the delete-user wizard
  // to show a picker instead of a free-form text input.
  const scannedRemoteUsers = useMemo<RemoteUserInfo[]>(() => {
    if (!lastScan?.data) return [];
    try {
      const parsed = JSON.parse(lastScan.data) as VMInfoType;
      return Array.isArray(parsed.remote_users) ? parsed.remote_users : [];
    } catch {
      return [];
    }
  }, [lastScan?.data]);
  const queryClient = useQueryClient();
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string; vm_info?: VMInfoType } | null>(null);
  const [setupStatus, setSetupStatus] = useState<"idle" | "choosing" | "testing" | "installing" | "done" | "error">("idle");
  const [setupError, setSetupError] = useState("");
  const [setupKeySource, setSetupKeySource] = useState<"generate" | "existing">("generate");
  const [setupExistingKeyId, setSetupExistingKeyId] = useState("");
  const [expandedOp, setExpandedOp] = useState<string | null>(null);
  const [customScripts, saveCustomScripts] = useLocalStorage<{ id: string; name: string; command: string }[]>(`custom_scripts_${slug}`, []);
  const [customResult, setCustomResult] = useState<{ id: string; success: boolean; error?: string } | null>(null);

  // Create remote user form
  const [showCreateUser, setShowCreateUser] = useState(false);
  const [createUserName, setCreateUserName] = useState("");
  const [createUserKeyId, setCreateUserKeyId] = useState("");

  // Delete remote user form
  const [showDeleteUser, setShowDeleteUser] = useState(false);
  const [deleteUserName, setDeleteUserName] = useState("");
  const [deleteRemoveHome, setDeleteRemoveHome] = useState(false);

  // Network test form (ping + TCP probe; runs from the SSHCM server, not via SSH)
  const [showNetworkTest, setShowNetworkTest] = useState(false);
  const [networkTestPort, setNetworkTestPort] = useState("");

  // Track which operation is running
  const [runningOp, setRunningOp] = useState<string | null>(null);

  // Console drawer
  const [consoleOpen, setConsoleOpen] = useState(false);
  const [consoleEntry, setConsoleEntry] = useState<ConsoleEntry | null>(null);

  const pushConsole = useCallback((label: string, status: ConsoleEntry["status"], content: ReactNode) => {
    setConsoleEntry({ label, status, content, timestamp: Date.now() });
    setConsoleOpen(true);
    setRunningOp(null);
  }, []);

  const { data: sshKeysList = [] } = useQuery({ queryKey: ["ssh-keys"], queryFn: sshKeysAPI.list });
  const { data: integrationsData } = useQuery({
    queryKey: ["integrations"],
    queryFn: integrationsAPI.get,
    retry: false,
    staleTime: 60_000,
  });
  const grafanaEnabled = integrationsData?.grafana?.grafana_enabled === "true";
  const grafanaRemoteWriteConfigured = !!integrationsData?.grafana?.grafana_prom_remote_write_url;
  const { data: operationLogs = [] } = useQuery({
    queryKey: ["operation-logs", slug],
    queryFn: () => sshAPI.operationLogs(slug),
  });
  const [expandedLogId, setExpandedLogId] = useState<number | null>(null);

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ["host", slug] });
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
    queryClient.invalidateQueries({ queryKey: ["operation-logs", slug] });
  };

  // Shared configuration for test-connection mutations. Each button gets
  // its own mutation instance with a distinct label so the Console panel
  // shows the actual operation name (e.g. "Escanear & Capturar (Chave)")
  // instead of always reading "Testar Conex\u00e3o". The factory captures the
  // label so all three reuse the same onResult/onAfterSuccess machinery.
  const buildTestMutationOptions = useCallback(
    (label: string) => ({
      slug,
      mutationFn: ({ method, capture }: { method: "password" | "key"; capture: boolean }) =>
        sshAPI.testConnection(slug, method, capture),
      label,
      pushConsole,
      onAfterSuccess: (data: { success: boolean; error?: string; vm_info?: VMInfoType }) => setTestResult(data),
      onResult: (data: { success: boolean; error?: string; vm_info?: VMInfoType }) => {
        if (data.success && data.vm_info) {
          const warnings = data.vm_info.warnings || [];
          return {
            status: warnings.length > 0 ? "warning" as const : "success" as const,
            content: (
              <div className="space-y-3">
                {warnings.length > 0 && (
                  <div className="rounded-[var(--radius-md)] p-2.5 bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-[var(--warning)] text-xs">
                    <p className="font-medium mb-1">{t("operation.scanWarnings")}</p>
                    {warnings.map((w, i) => (
                      <p key={i} className="leading-relaxed">{"\u2022"} {w}</p>
                    ))}
                  </div>
                )}
                <VMInfoDisplay info={data.vm_info!} locale={locale} compact />
                <p className="text-[var(--text-faint)] text-2xs">{t("operation.scanSaved")}</p>
              </div>
            ),
          };
        }
        if (data.success) return { status: "success" as const, content: t("operation.connectionSuccessful") };
        return { status: "error" as const, content: data.error || t("filters.failed") };
      },
    }),
    [slug, t, locale, pushConsole],
  );

  const testPasswordMutation = useSSHMutation(useMemo(() => buildTestMutationOptions(t("operation.testPassword")), [buildTestMutationOptions, t]));
  const testKeyMutation = useSSHMutation(useMemo(() => buildTestMutationOptions(t("operation.testKey")), [buildTestMutationOptions, t]));
  const testCapturePasswordMutation = useSSHMutation(useMemo(() => buildTestMutationOptions(t("operation.testCapturePassword")), [buildTestMutationOptions, t]));
  const testCaptureKeyMutation = useSSHMutation(useMemo(() => buildTestMutationOptions(t("operation.testCaptureKey")), [buildTestMutationOptions, t]));

  const testAndSetupKey = async () => {
    setRunningOp("setup-key");
    setSetupStatus("testing");
    setSetupError("");
    try {
      const testRes = await sshAPI.testConnection(slug, "password", true);
      if (!testRes.success) {
        setSetupStatus("error");
        setSetupError(`Connection failed: ${testRes.error}`);
        pushConsole(t("operation.setupKey"), "error", `Connection failed: ${testRes.error}`);
        return;
      }
      setTestResult(testRes);
      invalidateAll();
      setSetupStatus("installing");
      if (setupKeySource === "existing" && setupExistingKeyId) {
        await hostsAPI.update(slug, { ssh_key_id: parseInt(setupExistingKeyId) } as Record<string, unknown>);
        await sshAPI.setupKey(slug, { mode: "existing", use_saved_password: true });
      } else {
        await sshAPI.setupKey(slug, { mode: "generate", use_saved_password: true });
      }
      invalidateAll();
      queryClient.invalidateQueries({ queryKey: ["ssh-keys"] });
      setSetupStatus("done");
      pushConsole(t("operation.setupKey"), "success", t("operation.keyInstalled"));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed";
      setSetupStatus("error");
      setSetupError(msg);
      invalidateAll();
      pushConsole(t("operation.setupKey"), "error", msg);
    }
  };

  const fixDevNullMutation = useSSHMutation({
    slug,
    mutationFn: (method: "password" | "key") => sshAPI.fixDevNull(slug, method),
    label: t("operation.repairDevNull"),
    pushConsole,
    onResult: (data) => ({
      status: data.success ? "success" : "warning",
      content: <OperationOutput data={data} />,
    }),
  });

  const dockerLogsInspectMutation = useSSHMutation({
    slug,
    mutationFn: () => sshAPI.dockerLogsInspect(slug),
    label: t("operation.dockerLogsInspect"),
    pushConsole,
    onResult: (data) => {
      if (!data.success) {
        return { status: "error", content: <OperationOutput data={{ error: data.error }} /> };
      }
      const r = data.report;
      if (!r) return { status: "warning", content: t("host.ops.noReportReturned") };
      return {
        status: r.risk_level === "critical" ? "error" : r.risk_level === "warning" ? "warning" : "success",
        content: <DockerLogsReportView report={r} />,
      };
    },
  });

  const dockerLogsApplyRotationMutation = useSSHMutation({
    slug,
    mutationFn: (opts: { max_size?: string; max_file?: number; driver?: string }) =>
      sshAPI.dockerLogsApplyRotation(slug, opts),
    label: t("operation.dockerLogsApplyRotation"),
    pushConsole,
    onResult: (data) => ({
      status: data.success ? "success" : "error",
      content: <OperationOutput data={data} />,
    }),
  });

  const sudoNopasswdMutation = useSSHMutation({
    slug,
    mutationFn: () => sshAPI.setupSudoNopasswd(slug),
    label: t("operation.setupSudoNopasswd"),
    pushConsole,
    onResult: (data) => ({
      status: data.success ? "success" : "error",
      content: <OperationOutput data={data} />,
    }),
  });

  const createRemoteUserMutation = useSSHMutation({
    slug,
    mutationFn: ({ username, pubKey, force, sshKeyId }: { username: string; pubKey: string; force?: boolean; sshKeyId?: number }) => sshAPI.createRemoteUser(slug, username, pubKey, force, sshKeyId),
    label: t("operation.createRemoteUser"),
    pushConsole,
    onResult: (data) => {
      if (data.user_exists) {
        setRunningOp(null);
        return { status: "error", content: <OperationOutput data={{ error: data.output || data.error }} /> };
      }
      setShowCreateUser(false);
      return { status: data.success ? "success" : "error", content: <OperationOutput data={data} /> };
    },
  });

  const deleteRemoteUserMutation = useSSHMutation({
    slug,
    mutationFn: ({ username, removeHome }: { username: string; removeHome: boolean }) => sshAPI.deleteRemoteUser(slug, username, removeHome),
    label: t("operation.deleteRemoteUser"),
    pushConsole,
    onResult: (data) => {
      // Always close the wizard drawer when the mutation settles — the
      // console drawer will show the outcome, and leaving the wizard open
      // would stack two drawers on top of each other.
      setShowDeleteUser(false);
      if (data.success) {
        setDeleteUserName("");
        setDeleteRemoveHome(false);
      }
      return { status: data.success ? "success" : "error", content: <OperationOutput data={data} /> };
    },
  });

  const listRemoteKeysMutation = useSSHMutation({
    slug,
    mutationFn: () => sshAPI.listRemoteKeys(slug),
    label: t("operation.listRemoteKeys"),
    pushConsole,
    onResult: (data) => {
      if (!data.success) return { status: "error", content: data.error || "Failed" };
      const keys = data.keys || [];
      return {
        status: "success",
        content: keys.length === 0 ? <p className="text-xs">{t("operation.noRemoteKeys")}</p> : (
          <div className="space-y-1.5">
            {keys.map((k: RemoteKeyInfo, i: number) => (
              <div key={`${k.fingerprint}-${i}`} className="flex items-center gap-2 text-xs">
                <Badge>{k.source === "authorized_keys" ? "authorized" : "private"}</Badge>
                <span className="text-[var(--text-muted)] truncate font-mono">{k.fingerprint}</span>
                <span className="text-[var(--text-faint)]">{k.type}</span>
                {k.name && <span className="text-[var(--text-secondary)] truncate">{k.name}</span>}
              </div>
            ))}
          </div>
        ),
      };
    },
  });

  const dockerSetupMutation = useSSHMutation({
    slug,
    mutationFn: (fix: boolean) => sshAPI.dockerSetup(slug, fix),
    label: t("operation.dockerSetup"),
    pushConsole,
    onResult: (data) => {
      if (!data.success || !data.status) return { status: "error", content: data.error || "Failed" };
      const s = data.status as DockerStatusType;
      return {
        status: !s.installed || s.needs_sudo ? "warning" : "success",
        content: (
          <div className="space-y-2 text-xs">
            <p>{s.message}</p>
            {s.installed && (
              <div className="grid grid-cols-3 gap-2 text-2xs">
                <div><span className="text-[var(--text-faint)] block">Docker</span><span className="font-mono">{s.docker_version?.replace("Docker version ", "").split(",")[0] || "-"}</span></div>
                <div><span className="text-[var(--text-faint)] block">Compose</span><span className="font-mono">{s.compose_version?.replace(/.*version\s*/i, "").split(",")[0] || "-"}</span></div>
                <div><span className="text-[var(--text-faint)] block">{t("operation.dockerGroup")}</span><span className={s.user_in_group ? "text-[var(--success)]" : "text-[var(--danger)]"}>{s.user_in_group ? t("common.yes") : t("common.no")}</span></div>
              </div>
            )}
          </div>
        ),
      };
    },
  });

  const grafanaAgentMutation = useSSHMutation({
    slug,
    mutationFn: () => sshAPI.grafanaAgentSetup(slug),
    label: "Install Grafana Agent",
    pushConsole,
    onResult: (data) => {
      if (!data.success) {
        return {
          status: "error" as const,
          content: (
            <div className="space-y-2 text-xs">
              <p>{data.error || t("host.ops.installFailed")}</p>
              {data.output && (
                <pre
                  className="bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded p-2 text-2xs overflow-x-auto whitespace-pre-wrap break-all font-mono"
                >
                  {data.output}
                </pre>
              )}
            </div>
          ),
        };
      }
      return {
        status: "success" as const,
        content: (
          <div className="space-y-2 text-xs">
            <p>{data.message || t("host.ops.grafanaAgentInstalled")}</p>
            {data.output && (
              <details>
                <summary className="cursor-pointer text-[var(--text-muted)]">{t("operation.consoleOutput")}</summary>
                <pre
                  className="mt-2 bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded p-2 text-2xs overflow-x-auto whitespace-pre-wrap break-all font-mono"
                >
                  {data.output}
                </pre>
              </details>
            )}
          </div>
        ),
      };
    },
  });

  const nginxCleanupMutation = useSSHMutation({
    slug,
    mutationFn: (purge: boolean) => sshAPI.nginxCleanup(slug, purge),
    label: t("operation.nginxCleanup"),
    pushConsole,
    onResult: (data) => {
      if (!data.success || !data.status) return { status: "error" as const, content: data.error || "Failed" };
      const s = data.status as NginxCleanupStatusType;
      if (!s.found) return { status: "warning" as const, content: s.message };
      if (s.is_container && !s.is_native) return { status: "warning" as const, content: s.message };
      return {
        status: s.steps.every((st) => st.status === "success" || st.status === "skipped") ? "success" as const : "warning" as const,
        content: (
          <div className="space-y-2 text-xs">
            <p>{s.message}</p>
            {s.backup_path && <p>{t("host.ops.backupLabel")} <span className="font-mono">{s.backup_path}</span></p>}
            <div className="space-y-1">
              {s.steps.map((step, i) => (
                <div key={i} className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${step.status === "success" ? "bg-[var(--success)]" : step.status === "failed" ? "bg-[var(--danger)]" : "bg-[var(--text-faint)]"}`} />
                  <span className="text-[var(--text-primary)]">{step.name}</span>
                  {step.output && <span className="text-[var(--text-faint)] truncate font-mono">{step.output}</span>}
                </div>
              ))}
            </div>
          </div>
        ),
      };
    },
  });

  const networkTestMutation = useSSHMutation({
    slug,
    mutationFn: (port?: number) => sshAPI.networkTest(slug, port),
    label: t("operation.networkTest"),
    pushConsole,
    onAfterSuccess: () => {
      setShowNetworkTest(false);
      setNetworkTestPort("");
    },
    onResult: (data: NetworkTestResult) => {
      const renderPort = (label: string, p: { port: number; ok: boolean; latency_ms: number; error?: string }) => (
        <div className="flex items-center gap-2 text-xs">
          <span className={`w-2 h-2 rounded-full shrink-0 ${p.ok ? "bg-[var(--success)]" : "bg-[var(--danger)]"}`} />
          <span className="text-[var(--text-primary)]">{label}</span>
          <span className="text-[var(--text-faint)] font-mono">tcp/{p.port}</span>
          {p.ok
            ? <span className="text-[var(--text-muted)] tabular-nums">{p.latency_ms}ms</span>
            : <span className="text-[var(--danger)] truncate">{p.error || t("operation.networkTestFailed")}</span>}
        </div>
      );
      const ping = data.ping;
      const pingDot = ping.skipped ? "border border-[var(--border-default)]" : ping.ok ? "bg-[var(--success)]" : "bg-[var(--danger)]";
      return {
        status: data.success ? "success" : "error",
        content: (
          <div className="space-y-2">
            <div className="flex items-center gap-2 text-xs">
              <span className={`w-2 h-2 rounded-full shrink-0 ${pingDot}`} />
              <span className="text-[var(--text-primary)]">{t("operation.networkTestPing")}</span>
              <span className="text-[var(--text-faint)] font-mono">{data.hostname}</span>
              {ping.skipped
                ? <span className="text-[var(--text-faint)] truncate">{ping.error || t("operation.networkTestPingSkipped")}</span>
                : ping.ok
                  ? <span className="text-[var(--text-muted)] tabular-nums">{ping.latency_ms ?? 0}ms</span>
                  : <span className="text-[var(--danger)] truncate">{t("operation.networkTestFailed")}</span>}
            </div>
            {renderPort(t("operation.networkTestSshPort"), data.ssh_port)}
            {data.custom_port && renderPort(t("operation.networkTestCustomPort"), data.custom_port)}
            {ping.output && (
              <details className="text-2xs">
                <summary className="cursor-pointer text-[var(--text-muted)]">{t("operation.networkTestPingDetails")}</summary>
                <pre className="mt-1 bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded p-2 overflow-x-auto whitespace-pre-wrap break-all font-mono">{ping.output}</pre>
              </details>
            )}
          </div>
        ),
      };
    },
  });

  const handleFixDevNull = () => {
    const method = resolveAuthMethod(hasPassword, hasKey, preferredAuth);
    if (!method) {
      setRunningOp(null);
      pushConsole(t("operation.repairDevNull"), "error", t("operation.setPreferredAuth"));
      return;
    }
    if (!window.confirm(t("operation.confirmRepair"))) return;
    setRunningOp("fix-devnull");
    fixDevNullMutation.mutate(method);
  };

  const handleSetupSudoNopasswd = () => {
    if (!window.confirm(t("operation.confirmSudoNopasswd"))) return;
    setRunningOp("setup-sudo-nopasswd");
    sudoNopasswdMutation.mutate();
  };

  const toggleOp = (id: string) => setExpandedOp((prev) => (prev === id ? null : id));

  // Safety net: clear a stale runningOp if no mutation is actually pending.
  // Prevents a Run button from being stuck in its loading/disabled state if a
  // mutation's success/error callback somehow fails to fire (e.g. a Query
  // Client cancellation during a rapid re-click or component unmount race).
  const anyMutationPending =
    testPasswordMutation.isPending ||
    testKeyMutation.isPending ||
    testCapturePasswordMutation.isPending ||
    testCaptureKeyMutation.isPending ||
    fixDevNullMutation.isPending ||
    sudoNopasswdMutation.isPending ||
    createRemoteUserMutation.isPending ||
    deleteRemoteUserMutation.isPending ||
    listRemoteKeysMutation.isPending ||
    dockerSetupMutation.isPending ||
    dockerLogsInspectMutation.isPending ||
    dockerLogsApplyRotationMutation.isPending ||
    nginxCleanupMutation.isPending ||
    networkTestMutation.isPending ||
    grafanaAgentMutation.isPending;

  useEffect(() => {
    if (!anyMutationPending && runningOp && setupStatus !== "testing" && setupStatus !== "installing") {
      setRunningOp(null);
    }
  }, [anyMutationPending, runningOp, setupStatus]);

  type OpDef = {
    id: string;
    label: string;
    description: string;
    command: string;
    disabled?: boolean;
    disabledReason?: string;
    loading?: boolean;
    status?: "success" | "failed" | null;
    showStatus?: boolean;
    onClick: () => void;
  };

  const operations: OpDef[] = [
    {
      id: "network-test",
      label: t("operation.networkTest"),
      description: t("operation.networkTestDesc"),
      command: `ping -c 3 <host> && nc -zv <host> <port>`,
      loading: runningOp === "network-test",
      onClick: () => { setNetworkTestPort(""); setShowNetworkTest(true); },
    },
    {
      id: "test-password",
      label: t("operation.testPassword"),
      description: t("operation.testPasswordDesc"),
      command: `ssh -o StrictHostKeyChecking=no -o BatchMode=yes <user>@<host> -p <port> echo ok`,
      disabled: !hasPassword,
      disabledReason: !hasPassword ? t("host.testPasswordDisabledNoPassword") : undefined,
      loading: testPasswordMutation.isPending,
      status: passwordTestStatus,
      showStatus: true,
      onClick: () => { setRunningOp("test-password"); testPasswordMutation.mutate({ method: "password", capture: false }); },
    },
    {
      id: "test-key",
      label: t("operation.testKey"),
      description: t("operation.testKeyDesc"),
      command: `ssh -o StrictHostKeyChecking=no -i <key_path> <user>@<host> -p <port> echo ok`,
      disabled: !hasKey,
      disabledReason: !hasKey ? t("host.testKeyDisabledNoKey") : undefined,
      loading: testKeyMutation.isPending,
      status: keyTestStatus,
      showStatus: true,
      onClick: () => { setRunningOp("test-key"); testKeyMutation.mutate({ method: "key", capture: false }); },
    },
    // Scan-with-capture is split per auth method (password vs. key) so the
    // operator picks the credential explicitly rather than relying on the
    // host's preferred_auth. Each button is disabled when its credential
    // isn't configured. This mirrors the test-password / test-key pair
    // above for connection-only tests.
    {
      id: "test-capture-password",
      label: t("operation.testCapturePassword"),
      description: t("operation.testCapturePasswordDesc"),
      command: `ssh -o PreferredAuthentications=password <user>@<host> "uname -a && free -h && df -h && docker ps ..."`,
      disabled: !hasPassword,
      disabledReason: !hasPassword ? t("host.testPasswordDisabledNoPassword") : undefined,
      loading: testCapturePasswordMutation.isPending,
      onClick: () => {
        setRunningOp("test-capture-password");
        testCapturePasswordMutation.mutate({ method: "password", capture: true });
      },
    },
    {
      id: "test-capture-key",
      label: t("operation.testCaptureKey"),
      description: t("operation.testCaptureKeyDesc"),
      command: `ssh -i <key_path> <user>@<host> "uname -a && free -h && df -h && docker ps ..."`,
      disabled: !hasKey,
      disabledReason: !hasKey ? t("host.testKeyDisabledNoKey") : undefined,
      loading: testCaptureKeyMutation.isPending,
      onClick: () => {
        setRunningOp("test-capture-key");
        testCaptureKeyMutation.mutate({ method: "key", capture: true });
      },
    },
    ...(hasPassword ? [{
      id: "setup-key",
      label: hasKey ? t("operation.reSetupKey") : t("operation.setupKey"),
      description: t("operation.setupKeyDesc"),
      command: `ssh-keygen -t ed25519 && ssh-copy-id -i <key> <user>@<host>`,
      loading: runningOp === "setup-key",
      onClick: () => { setSetupStatus("choosing"); setSetupError(""); setTestResult(null); },
    } satisfies OpDef] : []),
    {
      id: "fix-devnull",
      label: t("operation.repairDevNull"),
      description: t("operation.repairDevNullDesc"),
      command: `ssh <user>@<host> "sudo rm -f /dev/null && sudo mknod -m 0666 /dev/null c 1 3"`,
      disabled: !hasPassword && !hasKey,
      loading: runningOp === "fix-devnull",
      onClick: handleFixDevNull,
    },
    {
      id: "list-remote-keys",
      label: t("operation.listRemoteKeys"),
      description: t("operation.listRemoteKeysDesc"),
      command: `ssh <user>@<host> "ssh-keygen -lf ~/.ssh/authorized_keys; for f in ~/.ssh/id_*; do ssh-keygen -lf $f.pub; done"`,
      disabled: !hasPassword && !hasKey,
      loading: runningOp === "list-remote-keys",
      onClick: () => { setRunningOp("list-remote-keys"); listRemoteKeysMutation.mutate(); },
    },
    ...(isAdmin ? [{
      id: "docker-setup",
      label: t("operation.dockerSetup"),
      description: t("operation.dockerSetupDesc"),
      command: `docker --version && docker compose version && id -nG | grep docker && docker ps`,
      disabled: !hasPassword && !hasKey,
      loading: runningOp === "docker-setup",
      status: dockerGroupStatus === "ok" || dockerGroupStatus === "fixed" ? "success" : dockerGroupStatus === "failed" || dockerGroupStatus === "needs_sudo" ? "failed" : null,
      showStatus: true,
      onClick: () => { setRunningOp("docker-setup"); dockerSetupMutation.mutate(true); },
    } satisfies OpDef] : []),
    {
      id: "docker-logs-inspect",
      label: t("operation.dockerLogsInspect"),
      description: t("operation.dockerLogsInspectDesc"),
      command: `docker info --format '{{json .}}' && docker ps -aq | xargs docker inspect ... && stat -c '%s %n' /var/lib/docker/containers/*/*-json.log`,
      disabled: !hasPassword && !hasKey,
      loading: runningOp === "docker-logs-inspect",
      onClick: () => { setRunningOp("docker-logs-inspect"); dockerLogsInspectMutation.mutate(); },
    },
    ...(isAdmin && hasPassword ? [{
      id: "docker-logs-apply-rotation",
      label: t("operation.dockerLogsApplyRotation"),
      description: t("operation.dockerLogsApplyRotationDesc"),
      command: `cat /etc/docker/daemon.json # then merge log-driver+log-opts and systemctl reload docker`,
      loading: runningOp === "docker-logs-apply-rotation",
      onClick: () => {
        setRunningOp("docker-logs-apply-rotation");
        dockerLogsApplyRotationMutation.mutate({ max_size: "30m", max_file: 3, driver: "json-file" });
      },
    } satisfies OpDef] : []),
    ...(isAdmin ? [{
      id: "nginx-cleanup",
      label: t("operation.nginxCleanup"),
      description: t("operation.nginxCleanupDesc"),
      command: `systemctl is-active nginx && tar -czf /tmp/nginx-backup.tar.gz /etc/nginx/ && systemctl stop nginx && systemctl disable nginx`,
      disabled: !hasPassword && !hasKey,
      loading: runningOp === "nginx-cleanup",
      onClick: () => {
        if (!window.confirm(t("operation.confirmNginxCleanup"))) return;
        const purge = window.confirm(t("operation.confirmNginxPurge"));
        setRunningOp("nginx-cleanup");
        nginxCleanupMutation.mutate(purge);
      },
    } satisfies OpDef] : []),
    ...(isAdmin && grafanaEnabled ? [{
      id: "grafana-agent-setup",
      label: "Install Grafana Agent",
      description: grafanaRemoteWriteConfigured
        ? "Installs grafana-agent on this host (with node_exporter built in) configured to remote_write to the endpoint in Settings → Integrations → Grafana. Requires sudo."
        : "Disabled — configure the Prometheus remote_write URL in Settings → Integrations → Grafana first.",
      command: `curl -fsSL https://github.com/grafana/agent/releases/... -o grafana-agent && install /usr/local/bin/grafana-agent && systemctl enable --now grafana-agent`,
      disabled: !hasPassword || !grafanaRemoteWriteConfigured,
      loading: runningOp === "grafana-agent-setup",
      onClick: () => {
        if (!window.confirm("Install and start grafana-agent on this host? This requires sudo and will overwrite any existing /etc/grafana-agent/config.yaml.")) return;
        setRunningOp("grafana-agent-setup");
        grafanaAgentMutation.mutate();
      },
    } satisfies OpDef] : []),
    ...(hasPassword && isAdmin ? [{
      id: "setup-sudo-nopasswd",
      label: t("operation.setupSudoNopasswd"),
      description: t("operation.setupSudoNopasswdDesc"),
      command: `echo '<password>' | sudo -S sh -c 'echo "<user> ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/<user>-nopasswd && chmod 440 /etc/sudoers.d/<user>-nopasswd'`,
      loading: runningOp === "setup-sudo-nopasswd",
      onClick: handleSetupSudoNopasswd,
    } satisfies OpDef] : []),
    ...(hasPassword && isAdmin ? [{
      id: "create-remote-user",
      label: t("operation.createRemoteUser"),
      description: t("operation.createRemoteUserDesc"),
      command: `sudo useradd -m -s /bin/bash <username> && sudo mkdir -p /home/<username>/.ssh && echo '<pub_key>' | sudo tee -a /home/<username>/.ssh/authorized_keys && sudo sh -c 'echo "<username> ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/<username>-nopasswd'`,
      loading: runningOp === "create-remote-user",
      onClick: () => setShowCreateUser(true),
    } satisfies OpDef] : []),
    ...(hasPassword && isAdmin ? [{
      id: "delete-remote-user",
      label: t("operation.deleteRemoteUser"),
      description: t("operation.deleteRemoteUserDesc"),
      command: `sudo pkill -KILL -u <username>; sudo userdel [-r] <username> && sudo rm -f /etc/sudoers.d/<username>-nopasswd`,
      loading: runningOp === "delete-remote-user",
      onClick: () => setShowDeleteUser(true),
    } satisfies OpDef] : []),
  ];

  return (
    <div className="space-y-4">
      {serverInfo && !serverInfo.is_local && (
        <p className="text-xs text-[var(--text-faint)]">{t("host.ops.serverLabel")} <span className="text-[var(--text-muted)]">{serverInfo.hostname}</span></p>
      )}

      {!hasPassword && !hasKey && (
        <div className="text-sm text-[var(--text-muted)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-4 text-center">
          {t("operation.noCreds")}
        </div>
      )}

      {/* Built-in operations header */}
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">{t("operation.builtIn")}</h3>
        <IconButton
          variant={consoleEntry ? "active" : "default"}
          onClick={() => setConsoleOpen(true)}
          disabled={!consoleEntry}
          title={t("operation.console")}
        >
          <Icon path={ICON_PATHS.terminal} />
        </IconButton>
      </div>

      {/* Operations list */}
      <div className="space-y-2">
        {operations.map((op) => (
          <div key={op.id} className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
            <div className="flex items-center gap-3 px-4 py-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-medium text-[var(--text-primary)]">{op.label}</span>
                  {op.showStatus && (
                    <span className={`w-2 h-2 rounded-full shrink-0 ${op.status === "success" ? "bg-[var(--success)]" : op.status === "failed" ? "bg-[var(--danger)]" : "border border-[var(--border-default)]"}`} />
                  )}
                </div>
                <p className="text-xs text-[var(--text-muted)] mt-0.5">{op.description}</p>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <button
                  type="button"
                  onClick={() => toggleOp(op.id)}
                  className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                  title={t("operation.showCommand")}
                >
                  <Icon path={ICON_PATHS.chevronDown} className={`w-3.5 h-3.5 transition-transform duration-200 ${expandedOp === op.id ? "rotate-180" : ""}`} />
                </button>
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={op.disabled}
                  loading={op.loading}
                  onClick={op.onClick}
                  title={op.disabled ? op.disabledReason : undefined}
                >
                  {t("common.run")}
                </Button>
              </div>
            </div>
            {expandedOp === op.id && (
              <div className="px-4 pb-3 pt-0">
                <pre className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap font-mono">
                  {op.command}
                </pre>
              </div>
            )}
          </div>
        ))}
      </div>

      {/* Custom scripts */}
      {customScripts.length > 0 && (
      <div className="space-y-2">
        <SectionHeading as="h3">{t("operation.customScripts")}</SectionHeading>
        {customScripts.map((script) => (
          <div key={script.id} className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
            <div className="flex items-center gap-3 px-4 py-3">
              <div className="flex-1 min-w-0">
                <span className="text-sm font-medium text-[var(--text-primary)]">{script.name}</span>
                <p className="text-xs text-[var(--text-muted)] mt-0.5 truncate font-mono">{script.command}</p>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <button type="button" onClick={() => toggleOp(script.id)} className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors" title={t("operation.showCommand")}>
                  <Icon path={ICON_PATHS.chevronDown} className={`w-3.5 h-3.5 transition-transform duration-200 ${expandedOp === script.id ? "rotate-180" : ""}`} />
                </button>
                {isAdmin && (
                  <button type="button" onClick={() => saveCustomScripts(customScripts.filter((s) => s.id !== script.id))} className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--danger)] hover:bg-[var(--danger)]/10 transition-colors" title={t("common.delete")}>
                    <Icon path={ICON_PATHS.trashOutline} className="w-3.5 h-3.5" />
                  </button>
                )}
                <Button size="sm" variant="secondary" disabled={!hasPassword && !hasKey} onClick={async () => {
                  setCustomResult(null);
                  const method = hasPassword && hasKey ? (preferredAuth === "password" ? "password" : "key") : (hasPassword ? "password" : "key");
                  try {
                    const res = await sshAPI.testConnection(slug, method as "password" | "key", false);
                    setCustomResult({ id: script.id, success: res.success, error: res.error });
                  } catch (err) {
                    setCustomResult({ id: script.id, success: false, error: err instanceof Error ? err.message : "Failed" });
                  }
                }}>{t("common.run")}</Button>
              </div>
            </div>
            {expandedOp === script.id && (
              <div className="px-4 pb-3 pt-0">
                <pre className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap font-mono">{script.command}</pre>
              </div>
            )}
            {customResult?.id === script.id && (
              <div className={`mx-4 mb-3 rounded-[var(--radius-sm)] p-2.5 text-xs ${customResult.success ? "bg-[var(--success)]/10 text-[var(--success)]" : "bg-[var(--danger)]/10 text-[var(--danger)]"}`}>
                {customResult.success ? t("operation.executedSuccessfully") : `${t("filters.failed")}: ${customResult.error}`}
              </div>
            )}
          </div>
        ))}
      </div>
      )}

      {/* Key setup wizard (in drawer) */}
      <Drawer
        open={setupStatus === "choosing"}
        onClose={() => setSetupStatus("idle")}
        title={hasKey ? t("operation.reSetupKey") : t("operation.setupKey")}
        footer={
          <div className="flex gap-2 justify-end">
            <Button size="sm" variant="secondary" onClick={() => setSetupStatus("idle")}>{t("common.cancel")}</Button>
            <Button size="sm" onClick={testAndSetupKey} disabled={setupKeySource === "existing" && !setupExistingKeyId}>{t("operation.testInstall")}</Button>
          </div>
        }
      >
        <div className="space-y-3 text-[var(--text-primary)]">
          <p className="text-xs text-[var(--text-muted)]">{t("operation.setupKeyDesc")}</p>
          <div>
            <label className="block text-2xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-1.5">
              {t("operation.chooseKeySource")}
            </label>
            <div className="flex gap-2">
              <button type="button" onClick={() => setSetupKeySource("generate")} className={`flex-1 p-3 rounded-[var(--radius-md)] border text-xs text-left transition ${setupKeySource === "generate" ? "border-[var(--cyan)] bg-[var(--cyan)]/15 text-[var(--cyan)]" : "border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)]"}`}>
                <strong className="block mb-0.5">{t("operation.generateNewKey")}</strong>
                <span className="text-[var(--text-faint)]">{t("operation.generateNewKeyDesc")}</span>
              </button>
              <button type="button" onClick={() => setSetupKeySource("existing")} className={`flex-1 p-3 rounded-[var(--radius-md)] border text-xs text-left transition ${setupKeySource === "existing" ? "border-[var(--cyan)] bg-[var(--cyan)]/15 text-[var(--cyan)]" : "border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)]"}`}>
                <strong className="block mb-0.5">{t("operation.useExistingKey")}</strong>
                <span className="text-[var(--text-faint)]">{t("operation.useExistingKeyDesc")}</span>
              </button>
            </div>
          </div>
          {setupKeySource === "existing" && sshKeysList.length > 0 && (
            <select value={setupExistingKeyId} onChange={(e) => setSetupExistingKeyId(e.target.value)} className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm">
              <option value="">{t("operation.selectKey")}</option>
              {sshKeysList.map((k) => <option key={k.id} value={k.id.toString()}>{k.name}{k.fingerprint ? ` (${k.fingerprint})` : ""}</option>)}
            </select>
          )}
          {setupKeySource === "existing" && sshKeysList.length === 0 && <p className="text-xs text-[var(--text-faint)]">{t("operation.noKeysInDb")}</p>}
        </div>
      </Drawer>

      {/* Inline status for setup wizard (shown after drawer closes) */}
      {setupStatus === "testing" && <StatusAlert variant="loading">{t("operation.testingConnection")}</StatusAlert>}
      {setupStatus === "installing" && <StatusAlert variant="loading">{t("operation.installingKey")}</StatusAlert>}
      {setupStatus === "done" && <StatusAlert variant="success">{t("operation.keyInstalled")}</StatusAlert>}
      {setupStatus === "error" && <StatusAlert variant="error">{setupError}</StatusAlert>}

      {/* Create remote user wizard (in drawer) */}
      {(() => {
        const eligibleKeys = sshKeysList.filter(k => k.credential_type === "key" && k.has_public_key);
        const selectedKey = eligibleKeys.find(k => k.id.toString() === createUserKeyId);
        const nameValid = !!createUserName.trim() && /^[a-z_][a-z0-9_-]{0,31}$/.test(createUserName);
        const canSubmit = nameValid && !!createUserKeyId && eligibleKeys.length > 0;

        const runCreate = (force: boolean) => {
          if (!selectedKey) return;
          setRunningOp("create-remote-user");
          sshKeysAPI.get(selectedKey.id).then(detail => {
            if (!detail.public_key) {
              pushConsole(t("operation.createRemoteUser"), "error", t("operation.createRemoteUserNoPubKey"));
              setRunningOp(null);
              return;
            }
            createRemoteUserMutation.mutate({ username: createUserName.trim(), pubKey: detail.public_key, force, sshKeyId: selectedKey.id });
          }).catch((err) => {
            pushConsole(t("operation.createRemoteUser"), "error", err instanceof Error ? err.message : "Failed to load key");
            setRunningOp(null);
          });
        };

        return (
          <Drawer
            open={showCreateUser}
            onClose={() => setShowCreateUser(false)}
            title={t("operation.createRemoteUser")}
            footer={
              <div className="flex gap-2 justify-end flex-wrap">
                <Button size="sm" variant="secondary" onClick={() => setShowCreateUser(false)}>{t("common.cancel")}</Button>
                <Button
                  size="sm"
                  variant="secondary"
                  className="text-[var(--warning)] border-[var(--warning)]/30"
                  disabled={!canSubmit}
                  loading={createRemoteUserMutation.isPending}
                  onClick={() => runCreate(true)}
                >
                  {t("operation.createRemoteUserForce")}
                </Button>
                <Button
                  size="sm"
                  disabled={!canSubmit}
                  loading={createRemoteUserMutation.isPending}
                  onClick={() => runCreate(false)}
                >
                  {t("operation.createRemoteUserRun")}
                </Button>
              </div>
            }
          >
            <div className="space-y-6">
              <p className="text-xs text-[var(--text-muted)]">{t("operation.createRemoteUserFormDesc")}</p>

              {/* ── Identity section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.createRemoteUserIdentitySection")}
                </SectionHeading>
                <input
                  placeholder={t("operation.createRemoteUserPlaceholder")}
                  value={createUserName}
                  onChange={(e) => setCreateUserName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
                  pattern="^[a-z_][a-z0-9_\-]{0,31}$"
                  maxLength={32}
                  className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                />
                {createUserName && !nameValid && (
                  <p className="mt-1 text-xs text-[var(--warning)]">{t("operation.createRemoteUserInvalidName")}</p>
                )}
              </section>

              {/* ── Access section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.createRemoteUserAccessSection")}
                </SectionHeading>
                {eligibleKeys.length > 0 ? (
                  <div className="space-y-2">
                    <select
                      value={createUserKeyId}
                      onChange={(e) => setCreateUserKeyId(e.target.value)}
                      className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                    >
                      <option value="">{t("operation.selectKey")}</option>
                      {eligibleKeys.map((k) => (
                        <option key={k.id} value={k.id.toString()}>{k.name}{k.fingerprint ? ` (${k.fingerprint})` : ""}</option>
                      ))}
                    </select>
                    <p className="text-2xs text-[var(--text-faint)] leading-relaxed">
                      {t("operation.createRemoteUserKeyHint")}
                    </p>
                    {selectedKey && (
                      <div className="px-2.5 py-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-2xs">
                        <span className="text-[var(--text-faint)]">{t("operation.createRemoteUserKeyPreview")} </span>
                        <span className="text-[var(--text-primary)] font-mono">{selectedKey.name}</span>
                        {selectedKey.fingerprint && (
                          <>
                            <span className="text-[var(--text-faint)]"> · </span>
                            <span className="text-[var(--text-muted)] font-mono">{selectedKey.fingerprint}</span>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-xs text-[var(--warning)] bg-[var(--warning)]/10 border border-[var(--warning)]/25 rounded-[var(--radius-sm)] px-2.5 py-2">
                    {t("operation.createRemoteUserNoEligibleKeys")}
                  </p>
                )}
              </section>
            </div>
          </Drawer>
        );
      })()}

      {/* Delete remote user wizard (in drawer) */}
      {(() => {
        const nameValid = !!deleteUserName.trim() && /^[a-z_][a-z0-9_-]{0,31}$/.test(deleteUserName);
        // Client-side hint list only — the backend is the authority on what's
        // deletable (UID<1000, SSH login user, etc.) via sshtest.ErrUserProtected.
        const isProtected = ["root", "nobody", "daemon", "sync", "bin", "sys", "systemd"].includes(deleteUserName.trim());

        // Only offer deletable candidates: exclude the SSH login user (the
        // scan marks it with is_current) since deleting it would lock us out.
        // Non-login users (shell=/sbin/nologin) are allowed — they may still
        // be real accounts (e.g. service users) the admin wants to clean up.
        const pickerUsers = scannedRemoteUsers.filter((u) => !u.is_current);
        const usePicker = pickerUsers.length > 0;
        const selectedScannedUser = pickerUsers.find((u) => u.name === deleteUserName.trim());
        const canSubmit = nameValid && !isProtected;

        const closeAndReset = () => {
          setShowDeleteUser(false);
          setDeleteUserName("");
          setDeleteRemoveHome(false);
        };

        const runDelete = () => {
          if (!canSubmit) return;
          const confirmMsg = t("operation.deleteRemoteUserConfirm").replace("{username}", deleteUserName.trim());
          if (!window.confirm(confirmMsg)) return;
          setRunningOp("delete-remote-user");
          deleteRemoteUserMutation.mutate({ username: deleteUserName.trim(), removeHome: deleteRemoveHome });
        };

        return (
          <Drawer
            open={showDeleteUser}
            onClose={closeAndReset}
            title={t("operation.deleteRemoteUser")}
            footer={
              <div className="flex gap-2 justify-end">
                <Button size="sm" variant="secondary" onClick={closeAndReset}>{t("common.cancel")}</Button>
                <Button
                  size="sm"
                  variant="danger"
                  disabled={!canSubmit}
                  loading={deleteRemoteUserMutation.isPending}
                  onClick={runDelete}
                >
                  {t("operation.deleteRemoteUserRun")}
                </Button>
              </div>
            }
          >
            <div className="space-y-6">
              <p className="text-xs text-[var(--text-muted)]">{t("operation.deleteRemoteUserFormDesc")}</p>

              {/* ── Target user section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.deleteRemoteUserTargetSection")}
                </SectionHeading>
                {usePicker ? (
                  <div className="space-y-2">
                    <select
                      value={deleteUserName}
                      onChange={(e) => setDeleteUserName(e.target.value)}
                      className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                    >
                      <option value="">{t("operation.deleteRemoteUserPickPlaceholder")}</option>
                      {pickerUsers.map((u) => (
                        <option key={u.name} value={u.name}>
                          {u.name} {t("host.ops.uidSuffix", { uid: String(u.uid) })}{u.has_login ? "" : ` · ${t("operation.deleteRemoteUserNoLogin")}`}
                        </option>
                      ))}
                    </select>
                    {lastScan?.scanned_at && (
                      <p className="text-2xs text-[var(--text-faint)] leading-relaxed">
                        {t("operation.deleteRemoteUserSourceScan").replace("{date}", new Date(lastScan.scanned_at).toLocaleString(locale === "pt-BR" ? "pt-BR" : "en-US"))}
                      </p>
                    )}
                    {selectedScannedUser && (
                      <div className="px-2.5 py-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-2xs space-y-0.5">
                        {selectedScannedUser.home && (
                          <div>
                            <span className="text-[var(--text-faint)]">{t("operation.deleteRemoteUserHomeLabel")} </span>
                            <span className="text-[var(--text-primary)] font-mono">{selectedScannedUser.home}</span>
                          </div>
                        )}
                        {selectedScannedUser.shell && (
                          <div>
                            <span className="text-[var(--text-faint)]">{t("operation.deleteRemoteUserShellLabel")} </span>
                            <span className="text-[var(--text-primary)] font-mono">{selectedScannedUser.shell}</span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="space-y-2">
                    <input
                      placeholder={t("operation.deleteRemoteUserPlaceholder")}
                      value={deleteUserName}
                      onChange={(e) => setDeleteUserName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
                      pattern="^[a-z_][a-z0-9_\-]{0,31}$"
                      maxLength={32}
                      className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                    />
                    <p className="text-2xs text-[var(--text-faint)] leading-relaxed">
                      {t("operation.deleteRemoteUserNoScanHint")}
                    </p>
                  </div>
                )}
                {deleteUserName && !nameValid && (
                  <p className="mt-2 text-xs text-[var(--warning)]">{t("operation.createRemoteUserInvalidName")}</p>
                )}
                {isProtected && (
                  <p className="mt-2 text-xs text-[var(--warning)]">{t("operation.deleteRemoteUserProtected")}</p>
                )}
              </section>

              {/* ── Options section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.deleteRemoteUserOptionsSection")}
                </SectionHeading>
                <label className="flex items-start gap-2 text-xs cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={deleteRemoveHome}
                    onChange={(e) => setDeleteRemoveHome(e.target.checked)}
                    className="mt-0.5 accent-red-400"
                  />
                  <div>
                    <span className="text-[var(--text-primary)] font-medium">{t("operation.deleteRemoteUserRemoveHome")}</span>
                    <p className="text-2xs text-[var(--text-faint)] leading-relaxed">
                      {t("operation.deleteRemoteUserRemoveHomeHint")}
                    </p>
                  </div>
                </label>
              </section>

              {/* ── Warning section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--danger)] uppercase tracking-wider mb-2">
                  {t("operation.deleteRemoteUserWarningSection")}
                </h3>
                <div className="rounded-[var(--radius-sm)] bg-[var(--danger)]/10 border border-[var(--danger)]/25 px-2.5 py-2 text-xs leading-relaxed text-[var(--danger)]">
                  {t("operation.deleteRemoteUserWarning")}
                </div>
              </section>
            </div>
          </Drawer>
        );
      })()}

      {/* Network test wizard (in drawer) */}
      {(() => {
        const trimmed = networkTestPort.trim();
        const portNum = trimmed ? parseInt(trimmed, 10) : 0;
        const portValid = trimmed === "" || (Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535);

        const closeAndReset = () => {
          setShowNetworkTest(false);
          setNetworkTestPort("");
        };

        const runNetworkTest = () => {
          if (!portValid) return;
          setRunningOp("network-test");
          networkTestMutation.mutate(portNum > 0 ? portNum : undefined);
        };

        return (
          <Drawer
            open={showNetworkTest}
            onClose={closeAndReset}
            title={t("operation.networkTest")}
            footer={
              <div className="flex gap-2 justify-end">
                <Button size="sm" variant="secondary" onClick={closeAndReset}>{t("common.cancel")}</Button>
                <Button
                  size="sm"
                  disabled={!portValid}
                  loading={networkTestMutation.isPending}
                  onClick={runNetworkTest}
                >
                  {t("common.run")}
                </Button>
              </div>
            }
          >
            <div className="space-y-6">
              <p className="text-xs text-[var(--text-muted)]">{t("operation.networkTestFormDesc")}</p>

              <section>
                <SectionHeading as="h3">
                  {t("operation.networkTestPortSection")}
                </SectionHeading>
                <input
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={65535}
                  placeholder={t("operation.networkTestPortPlaceholder")}
                  value={networkTestPort}
                  onChange={(e) => setNetworkTestPort(e.target.value.replace(/[^0-9]/g, ""))}
                  className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                />
                <p className="mt-2 text-2xs text-[var(--text-faint)] leading-relaxed">
                  {t("operation.networkTestPortHint")}
                </p>
                {!portValid && (
                  <p className="mt-2 text-xs text-[var(--warning)]">{t("operation.networkTestPortInvalid")}</p>
                )}
              </section>
            </div>
          </Drawer>
        );
      })()}

      {/* Integrations */}
      <IntegrationsSection
        slug={slug}
        keyTestStatus={keyTestStatus}
        coolifyServerUUID={coolifyServerUUID}
        t={t}
        isAdmin={isAdmin}
      />

      {/* Operation Logs */}
      {operationLogs.length > 0 && (
        <div className="space-y-2 pt-2">
          <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">{t("operation.logs")}</h3>
          <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden divide-y divide-[var(--border-subtle)]">
            {operationLogs.map((log) => (
              <div key={log.id}>
                <button
                  type="button"
                  className="w-full flex items-center gap-3 px-4 py-2.5 text-left hover:bg-[var(--bg-elevated)] transition-colors"
                  onClick={() => setExpandedLogId(expandedLogId === log.id ? null : log.id)}
                >
                  <span className={`w-2 h-2 rounded-full shrink-0 ${log.status === "success" ? "bg-[var(--success)]" : "bg-[var(--danger)]"}`} />
                  <span className="text-xs font-medium text-[var(--text-primary)] min-w-0 truncate font-mono">
                    {opTypeLabel(log.operation_type, t)}
                  </span>
                  {log.auth_method && <Badge>{log.auth_method}</Badge>}
                  <span className="text-2xs text-[var(--text-faint)] ml-auto shrink-0 tabular-nums">
                    {formatLogTime(log.created_at, locale)}
                  </span>
                  <span className="text-2xs text-[var(--text-faint)] shrink-0">{log.user_name}</span>
                  {log.output && (
                    <Icon path={ICON_PATHS.chevronDown} className={`w-3 h-3 text-[var(--text-faint)] shrink-0 transition-transform duration-150 ${expandedLogId === log.id ? "rotate-180" : ""}`} />
                  )}
                </button>
                {expandedLogId === log.id && log.output && (
                  <div className="px-4 pb-3 pt-0">
                    <pre className="text-2xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap font-mono">{log.output}</pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Console Drawer — three sections (Wizard / Status / Output) instead
          of the single tinted card. Removing the colored outer wrapper fixes
          the "card inside card" visual noise when the output itself is made
          of VMInfoDisplay cards. */}
      <Drawer
        open={consoleOpen}
        onClose={() => setConsoleOpen(false)}
        onBack={() => setConsoleOpen(false)}
        title={t("operation.console")}
        wide
      >
        {consoleEntry ? (() => {
          const statusLabel =
            consoleEntry.status === "success" ? t("operation.statusSuccess")
            : consoleEntry.status === "error" ? t("operation.statusError")
            : consoleEntry.status === "warning" ? t("operation.statusWarning")
            : t("operation.statusRunning");
          const statusDotClass =
            consoleEntry.status === "success" ? "bg-[var(--success)]"
            : consoleEntry.status === "error" ? "bg-[var(--danger)]"
            : consoleEntry.status === "warning" ? "bg-[var(--warning)]"
            : "bg-[var(--info)] animate-pulse";
          const statusTextClass =
            consoleEntry.status === "success" ? "text-[var(--success)]"
            : consoleEntry.status === "error" ? "text-[var(--danger)]"
            : consoleEntry.status === "warning" ? "text-[var(--warning)]"
            : "text-[var(--info)]";

          return (
            <div className="space-y-6">
              {/* ── Wizard section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.consoleWizard")}
                </SectionHeading>
                <p className="text-sm font-medium text-[var(--text-primary)]">{consoleEntry.label}</p>
              </section>

              {/* ── Status section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.consoleStatus")}
                </SectionHeading>
                <div className="flex items-center gap-2 text-sm">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${statusDotClass}`} />
                  <span className={`font-medium ${statusTextClass}`}>{statusLabel}</span>
                  <span className="text-2xs text-[var(--text-faint)] ml-auto tabular-nums">
                    {new Date(consoleEntry.timestamp).toLocaleTimeString()}
                  </span>
                </div>
              </section>

              {/* ── Output section ── */}
              <section>
                <SectionHeading as="h3">
                  {t("operation.consoleOutput")}
                </SectionHeading>
                <div className="text-sm text-[var(--text-primary)]">
                  {typeof consoleEntry.content === "string" ? <p>{consoleEntry.content}</p> : consoleEntry.content}
                </div>
              </section>
            </div>
          );
        })() : (
          <p className="text-xs text-[var(--text-muted)]">{t("operation.noConsoleOutput")}</p>
        )}
      </Drawer>
    </div>
  );
}

function opTypeLabel(type_: string, t: (k: string) => string): string {
  switch (type_) {
    case "test": return t("operation.testConnection");
    case "network-test": return t("operation.networkTest");
    case "setup-key": return t("operation.setupKey");
    case "fix-dev-null": return t("operation.repairDevNull");
    case "setup-sudo-nopasswd": return t("operation.setupSudoNopasswd");
    case "list-remote-keys": return t("operation.listRemoteKeys");
    case "docker-setup": return t("operation.dockerSetup");
    case "nginx-cleanup": return t("operation.nginxCleanup");
    case "create-remote-user": return t("operation.createRemoteUser");
    case "delete-remote-user": return t("operation.deleteRemoteUser");
    default: return type_;
  }
}

function formatLogTime(dateStr: string, locale: string): string {
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return "-";
  const bcp47 = locale === "pt-BR" ? "pt-BR" : "en-US";
  return d.toLocaleString(bcp47, {
    day: "2-digit", month: "2-digit",
    hour: "2-digit", minute: "2-digit",
  });
}

/* ─── Docker logs report view ─── */

function DockerLogsReportView({ report }: { report: import("@/lib/api").DockerLogsReport }) {
  const { t } = useLocale();
  const riskClass =
    report.risk_level === "critical"
      ? "bg-[var(--danger)]/15 text-[var(--danger)] border-[var(--danger)]/40"
      : report.risk_level === "warning"
      ? "bg-[var(--warning)]/15 text-[var(--warning)] border-[var(--warning)]/40"
      : "bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/40";
  const totalHuman = humanizeBytesClient(report.total_log_bytes);
  const largestHuman = humanizeBytesClient(report.largest_log_bytes);

  return (
    <div className="space-y-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-2xs border ${riskClass}`}>
          <span className={`w-1.5 h-1.5 rounded-full ${
            report.risk_level === "critical" ? "bg-[var(--danger)]"
              : report.risk_level === "warning" ? "bg-[var(--warning)]"
              : "bg-[var(--success)]"
          }`} />
          {report.risk_level.toUpperCase()}
        </span>
        <span className="text-2xs text-[var(--text-muted)]">
          {t("host.ops.dockerLogsDriver")} <span className="font-mono">{report.log_driver || "—"}</span>
          {" · "}{t("host.ops.dockerLogsRotation")} <span className="font-mono">{report.rotation_configured ? t("host.ops.dockerLogsConfigured") : t("host.ops.dockerLogsMissing")}</span>
        </span>
      </div>

      {report.recommendation && (
        <div className="rounded-[var(--radius-md)] p-2.5 bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] leading-relaxed">
          {report.recommendation}
        </div>
      )}

      <div className="grid grid-cols-3 gap-3 text-xs">
        <div>
          <span className="block text-2xs text-[var(--text-faint)] uppercase tracking-wider">{t("host.ops.dockerLogsTotalSize")}</span>
          <span className="text-[var(--text-primary)] font-medium font-mono">{totalHuman}</span>
        </div>
        <div>
          <span className="block text-2xs text-[var(--text-faint)] uppercase tracking-wider">{t("host.ops.dockerLogsLargest")}</span>
          <span className="text-[var(--text-primary)] font-medium font-mono">{largestHuman}</span>
        </div>
        <div>
          <span className="block text-2xs text-[var(--text-faint)] uppercase tracking-wider">{t("host.ops.dockerLogsUnbounded")}</span>
          <span className="text-[var(--text-primary)] font-medium font-mono">{report.unbounded_containers}</span>
        </div>
      </div>

      {report.daemon_json_exists && (
        <div className="text-2xs text-[var(--text-muted)] leading-snug">
          <span className="block">{t("host.ops.dockerLogsDaemonDriver")} <span className="font-mono">{report.daemon_log_driver || t("host.ops.dockerLogsNotSet")}</span></span>
          {report.daemon_log_opts && Object.keys(report.daemon_log_opts).length > 0 && (
            <span className="block">{t("host.ops.dockerLogsOpts")} <span className="font-mono">{JSON.stringify(report.daemon_log_opts)}</span></span>
          )}
          {report.daemon_json_unclean && (
            <span className="block text-[var(--warning)]">{t("host.ops.dockerLogsDaemonUnclean")}</span>
          )}
        </div>
      )}

      {report.containers && report.containers.length > 0 && (
        <div>
          <span className="block text-2xs text-[var(--text-faint)] uppercase tracking-wider mb-1.5">{t("host.ops.dockerLogsContainers")}</span>
          <div className="space-y-1">
            {report.containers.map((c) => (
              <div
                key={c.id}
                className="flex flex-wrap items-center gap-2 px-2 py-1 rounded border border-[var(--border-subtle)]/50 text-xs"
              >
                <span className="font-medium text-[var(--text-primary)] font-mono">
                  {c.name}
                </span>
                <span className="text-2xs text-[var(--text-faint)]">{c.image}</span>
                <span
                  className={`ml-auto shrink-0 px-1.5 py-0.5 rounded text-2xs ${
                    c.has_rotation
                      ? "bg-[var(--success)]/10 text-[var(--success)] border border-[var(--success)]/30"
                      : "bg-[var(--warning)]/10 text-[var(--warning)] border border-[var(--warning)]/30"
                  }`}
                  title={c.has_rotation ? t("host.ops.dockerLogsRotatedTitle") : t("host.ops.dockerLogsUnboundedTitle")}
                >
                  {c.has_rotation ? t("host.ops.dockerLogsRotated") : t("host.ops.dockerLogsUnboundedShort")}
                </span>
                <span
                  className="shrink-0 text-[var(--text-secondary)] font-mono"
                  title={c.log_path}
                >
                  {c.human_size || humanizeBytesClient(c.size_bytes)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function humanizeBytesClient(n: number): string {
  if (!n || n < 1024) return `${n || 0} B`;
  const units = ["KiB", "MiB", "GiB", "TiB", "PiB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(1)} ${units[i]}`;
}
