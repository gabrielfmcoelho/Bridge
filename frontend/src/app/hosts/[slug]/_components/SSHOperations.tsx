"use client";

import { useState, useCallback, useEffect, useMemo, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { sshAPI, hostsAPI, sshKeysAPI } from "@/lib/api";
import { resolveAuthMethod } from "@/lib/utils";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { useSSHMutation } from "@/hooks/useSSHMutation";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import IconButton from "@/components/ui/IconButton";
import Drawer from "@/components/ui/Drawer";
import StatusAlert from "@/components/ui/StatusAlert";
import OperationOutput from "@/components/ui/OperationOutput";
import VMInfoDisplay from "./VMInfoDisplay";
import IntegrationsSection from "./IntegrationsSection";
import type { VMInfoType, OperationLog, RemoteKeyInfo, DockerStatusType, NginxCleanupStatusType, RemoteUserInfo } from "@/lib/api";

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
  t: (key: string) => string;
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

  // Shared configuration for test-connection mutations. Each button gets its
  // own mutation instance so rapid clicks across buttons can't drop a
  // previous invocation's callbacks (which used to leave runningOp stuck,
  // making the Run button appear permanently disabled).
  const testMutationOptions = useMemo(() => ({
    slug,
    mutationFn: ({ method, capture }: { method: "password" | "key"; capture: boolean }) =>
      sshAPI.testConnection(slug, method, capture),
    label: t("operation.testConnection"),
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
                <div className="rounded-[var(--radius-md)] p-2.5 bg-amber-500/10 border border-amber-500/25 text-amber-300 text-xs">
                  <p className="font-medium mb-1">{t("operation.scanWarnings")}</p>
                  {warnings.map((w, i) => (
                    <p key={i} className="leading-relaxed">{"\u2022"} {w}</p>
                  ))}
                </div>
              )}
              <VMInfoDisplay info={data.vm_info!} locale={locale} compact />
              <p className="text-[var(--text-faint)] text-[10px]">{t("operation.scanSaved")}</p>
            </div>
          ),
        };
      }
      if (data.success) return { status: "success" as const, content: t("operation.connectionSuccessful") };
      return { status: "error" as const, content: data.error || "Failed" };
    },
  }), [slug, t, locale, pushConsole]);

  const testPasswordMutation = useSSHMutation(testMutationOptions);
  const testKeyMutation = useSSHMutation(testMutationOptions);
  const testCaptureMutation = useSSHMutation(testMutationOptions);

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
                <span className="text-[var(--text-muted)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{k.fingerprint}</span>
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
              <div className="grid grid-cols-3 gap-2 text-[10px]">
                <div><span className="text-[var(--text-faint)] block">Docker</span><span style={{ fontFamily: "var(--font-mono)" }}>{s.docker_version?.replace("Docker version ", "").split(",")[0] || "-"}</span></div>
                <div><span className="text-[var(--text-faint)] block">Compose</span><span style={{ fontFamily: "var(--font-mono)" }}>{s.compose_version?.replace(/.*version\s*/i, "").split(",")[0] || "-"}</span></div>
                <div><span className="text-[var(--text-faint)] block">{t("operation.dockerGroup")}</span><span className={s.user_in_group ? "text-emerald-400" : "text-red-400"}>{s.user_in_group ? t("common.yes") : t("common.no")}</span></div>
              </div>
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
            {s.backup_path && <p>Backup: <span style={{ fontFamily: "var(--font-mono)" }}>{s.backup_path}</span></p>}
            <div className="space-y-1">
              {s.steps.map((step, i) => (
                <div key={i} className="flex items-center gap-2">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${step.status === "success" ? "bg-emerald-400" : step.status === "failed" ? "bg-red-400" : "bg-gray-400"}`} />
                  <span className="text-[var(--text-primary)]">{step.name}</span>
                  {step.output && <span className="text-[var(--text-faint)] truncate" style={{ fontFamily: "var(--font-mono)" }}>{step.output}</span>}
                </div>
              ))}
            </div>
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
    testCaptureMutation.isPending ||
    fixDevNullMutation.isPending ||
    sudoNopasswdMutation.isPending ||
    createRemoteUserMutation.isPending ||
    deleteRemoteUserMutation.isPending ||
    listRemoteKeysMutation.isPending ||
    dockerSetupMutation.isPending ||
    nginxCleanupMutation.isPending;

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
    {
      id: "test-capture",
      label: t("operation.testCapture"),
      description: t("operation.testCaptureDesc"),
      command: `ssh <user>@<host> "uname -a && free -h && df -h && docker ps --format '{{.Names}}' ..."`,
      disabled: !hasPassword && !hasKey,
      disabledReason: !hasPassword && !hasKey ? t("operation.noCreds") : undefined,
      loading: testCaptureMutation.isPending,
      onClick: () => { setRunningOp("test-capture"); testCaptureMutation.mutate({
        method: hasPassword && hasKey
          ? (preferredAuth === "password" ? "password" : "key")
          : (hasPassword ? "password" : "key"),
        capture: true,
      }); },
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
        <p className="text-xs text-[var(--text-faint)]">Server — <span className="text-[var(--text-muted)]">{serverInfo.hostname}</span></p>
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
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
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
                    <span className={`w-2 h-2 rounded-full shrink-0 ${op.status === "success" ? "bg-emerald-400" : op.status === "failed" ? "bg-red-400" : "border border-[var(--border-default)]"}`} />
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
                  <svg className={`w-3.5 h-3.5 transition-transform duration-200 ${expandedOp === op.id ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
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
                <pre className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap" style={{ fontFamily: "var(--font-mono)" }}>
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
        <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">{t("operation.customScripts")}</h3>
        {customScripts.map((script) => (
          <div key={script.id} className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
            <div className="flex items-center gap-3 px-4 py-3">
              <div className="flex-1 min-w-0">
                <span className="text-sm font-medium text-[var(--text-primary)]">{script.name}</span>
                <p className="text-xs text-[var(--text-muted)] mt-0.5 truncate" style={{ fontFamily: "var(--font-mono)" }}>{script.command}</p>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <button type="button" onClick={() => toggleOp(script.id)} className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors" title={t("operation.showCommand")}>
                  <svg className={`w-3.5 h-3.5 transition-transform duration-200 ${expandedOp === script.id ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" /></svg>
                </button>
                {isAdmin && (
                  <button type="button" onClick={() => saveCustomScripts(customScripts.filter((s) => s.id !== script.id))} className="w-7 h-7 flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-red-400 hover:bg-red-500/10 transition-colors" title={t("common.delete")}>
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}><path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
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
                <pre className="text-xs text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap" style={{ fontFamily: "var(--font-mono)" }}>{script.command}</pre>
              </div>
            )}
            {customResult?.id === script.id && (
              <div className={`mx-4 mb-3 rounded-[var(--radius-sm)] p-2.5 text-xs ${customResult.success ? "bg-emerald-500/10 text-emerald-400" : "bg-red-500/10 text-red-400"}`}>
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
            <label className="block text-[10px] font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-1.5">
              {t("operation.chooseKeySource")}
            </label>
            <div className="flex gap-2">
              <button type="button" onClick={() => setSetupKeySource("generate")} className={`flex-1 p-3 rounded-[var(--radius-md)] border text-xs text-left transition-all ${setupKeySource === "generate" ? "border-cyan-400 bg-cyan-500/15 text-cyan-300" : "border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)]"}`}>
                <strong className="block mb-0.5">{t("operation.generateNewKey")}</strong>
                <span className="text-[var(--text-faint)]">{t("operation.generateNewKeyDesc")}</span>
              </button>
              <button type="button" onClick={() => setSetupKeySource("existing")} className={`flex-1 p-3 rounded-[var(--radius-md)] border text-xs text-left transition-all ${setupKeySource === "existing" ? "border-cyan-400 bg-cyan-500/15 text-cyan-300" : "border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)]"}`}>
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
                  className="text-amber-400 border-amber-500/30"
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
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.createRemoteUserIdentitySection")}
                </h3>
                <input
                  placeholder={t("operation.createRemoteUserPlaceholder")}
                  value={createUserName}
                  onChange={(e) => setCreateUserName(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ""))}
                  pattern="^[a-z_][a-z0-9_\-]{0,31}$"
                  maxLength={32}
                  className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm"
                />
                {createUserName && !nameValid && (
                  <p className="mt-1 text-xs text-amber-400">{t("operation.createRemoteUserInvalidName")}</p>
                )}
              </section>

              {/* ── Access section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.createRemoteUserAccessSection")}
                </h3>
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
                    <p className="text-[10px] text-[var(--text-faint)] leading-relaxed">
                      {t("operation.createRemoteUserKeyHint")}
                    </p>
                    {selectedKey && (
                      <div className="px-2.5 py-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-[10px]">
                        <span className="text-[var(--text-faint)]">{t("operation.createRemoteUserKeyPreview")} </span>
                        <span className="text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{selectedKey.name}</span>
                        {selectedKey.fingerprint && (
                          <>
                            <span className="text-[var(--text-faint)]"> · </span>
                            <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{selectedKey.fingerprint}</span>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-xs text-amber-400 bg-amber-500/10 border border-amber-500/25 rounded-[var(--radius-sm)] px-2.5 py-2">
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
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.deleteRemoteUserTargetSection")}
                </h3>
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
                          {u.name} (uid {u.uid}){u.has_login ? "" : ` · ${t("operation.deleteRemoteUserNoLogin")}`}
                        </option>
                      ))}
                    </select>
                    {lastScan?.scanned_at && (
                      <p className="text-[10px] text-[var(--text-faint)] leading-relaxed">
                        {t("operation.deleteRemoteUserSourceScan").replace("{date}", new Date(lastScan.scanned_at).toLocaleString(locale === "pt-BR" ? "pt-BR" : "en-US"))}
                      </p>
                    )}
                    {selectedScannedUser && (
                      <div className="px-2.5 py-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-[10px] space-y-0.5">
                        {selectedScannedUser.home && (
                          <div>
                            <span className="text-[var(--text-faint)]">{t("operation.deleteRemoteUserHomeLabel")} </span>
                            <span className="text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{selectedScannedUser.home}</span>
                          </div>
                        )}
                        {selectedScannedUser.shell && (
                          <div>
                            <span className="text-[var(--text-faint)]">{t("operation.deleteRemoteUserShellLabel")} </span>
                            <span className="text-[var(--text-primary)]" style={{ fontFamily: "var(--font-mono)" }}>{selectedScannedUser.shell}</span>
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
                    <p className="text-[10px] text-[var(--text-faint)] leading-relaxed">
                      {t("operation.deleteRemoteUserNoScanHint")}
                    </p>
                  </div>
                )}
                {deleteUserName && !nameValid && (
                  <p className="mt-2 text-xs text-amber-400">{t("operation.createRemoteUserInvalidName")}</p>
                )}
                {isProtected && (
                  <p className="mt-2 text-xs text-amber-400">{t("operation.deleteRemoteUserProtected")}</p>
                )}
              </section>

              {/* ── Options section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.deleteRemoteUserOptionsSection")}
                </h3>
                <label className="flex items-start gap-2 text-xs cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={deleteRemoveHome}
                    onChange={(e) => setDeleteRemoveHome(e.target.checked)}
                    className="mt-0.5 accent-red-400"
                  />
                  <div>
                    <span className="text-[var(--text-primary)] font-medium">{t("operation.deleteRemoteUserRemoveHome")}</span>
                    <p className="text-[10px] text-[var(--text-faint)] leading-relaxed">
                      {t("operation.deleteRemoteUserRemoveHomeHint")}
                    </p>
                  </div>
                </label>
              </section>

              {/* ── Warning section ── */}
              <section>
                <h3 className="text-xs font-semibold text-red-400 uppercase tracking-wider mb-2">
                  {t("operation.deleteRemoteUserWarningSection")}
                </h3>
                <div className="rounded-[var(--radius-sm)] bg-red-500/10 border border-red-500/25 px-2.5 py-2 text-[11px] leading-relaxed text-red-300">
                  {t("operation.deleteRemoteUserWarning")}
                </div>
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
                  <span className={`w-2 h-2 rounded-full shrink-0 ${log.status === "success" ? "bg-emerald-400" : "bg-red-400"}`} />
                  <span className="text-xs font-medium text-[var(--text-primary)] min-w-0 truncate" style={{ fontFamily: "var(--font-mono)" }}>
                    {opTypeLabel(log.operation_type, t)}
                  </span>
                  {log.auth_method && <Badge>{log.auth_method}</Badge>}
                  <span className="text-[10px] text-[var(--text-faint)] ml-auto shrink-0 tabular-nums">
                    {formatLogTime(log.created_at, locale)}
                  </span>
                  <span className="text-[10px] text-[var(--text-faint)] shrink-0">{log.user_name}</span>
                  {log.output && (
                    <svg className={`w-3 h-3 text-[var(--text-faint)] shrink-0 transition-transform duration-150 ${expandedLogId === log.id ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                    </svg>
                  )}
                </button>
                {expandedLogId === log.id && log.output && (
                  <div className="px-4 pb-3 pt-0">
                    <pre className="text-[10px] text-[var(--text-muted)] bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] p-3 overflow-x-auto whitespace-pre-wrap" style={{ fontFamily: "var(--font-mono)" }}>{log.output}</pre>
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
            consoleEntry.status === "success" ? "bg-emerald-400"
            : consoleEntry.status === "error" ? "bg-red-400"
            : consoleEntry.status === "warning" ? "bg-amber-400"
            : "bg-blue-400 animate-pulse";
          const statusTextClass =
            consoleEntry.status === "success" ? "text-emerald-400"
            : consoleEntry.status === "error" ? "text-red-400"
            : consoleEntry.status === "warning" ? "text-amber-400"
            : "text-blue-400";

          return (
            <div className="space-y-6">
              {/* ── Wizard section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.consoleWizard")}
                </h3>
                <p className="text-sm font-medium text-[var(--text-primary)]">{consoleEntry.label}</p>
              </section>

              {/* ── Status section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.consoleStatus")}
                </h3>
                <div className="flex items-center gap-2 text-sm">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${statusDotClass}`} />
                  <span className={`font-medium ${statusTextClass}`}>{statusLabel}</span>
                  <span className="text-[10px] text-[var(--text-faint)] ml-auto tabular-nums">
                    {new Date(consoleEntry.timestamp).toLocaleTimeString()}
                  </span>
                </div>
              </section>

              {/* ── Output section ── */}
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  {t("operation.consoleOutput")}
                </h3>
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
    case "test": return t("operation.testConnection") || "Test Connection";
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
