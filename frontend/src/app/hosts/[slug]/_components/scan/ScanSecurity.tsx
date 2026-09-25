"use client";

import type { ReactNode } from "react";
import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import type { VMInfoType } from "@/lib/api";
import { SubBlock, type T } from "./shared";

type YN = "yes" | "no" | "unknown";
const yn = (v?: string): YN => (v === "yes" ? "yes" : v === "no" ? "no" : "unknown");

/** Firewall status and the SSH auth policy ("does this host accept passwords, and for whom?"). */
export default function ScanSecurity({ info, t }: { info: VMInfoType; t: T }) {
  const empty = !info.firewall_status && !info.ssh_auth_policy;
  return (
    <SectionCard as="h3" title={t("scan.pane.group.security")} empty={empty ? t("scan.pane.empty.security") : undefined}>
      {info.ssh_auth_policy && <SSHAuthPolicy policy={info.ssh_auth_policy} users={info.remote_users ?? []} t={t} />}
      {info.firewall_status && (
        <SubBlock title={t("scan.firewallStatus")}>
          <pre className="text-xs text-[var(--text-secondary)] whitespace-pre-wrap break-all font-mono">{info.firewall_status}</pre>
        </SubBlock>
      )}
    </SectionCard>
  );
}

function SSHAuthPolicy({ policy, users, t }: {
  policy: NonNullable<VMInfoType["ssh_auth_policy"]>;
  users: NonNullable<VMInfoType["remote_users"]>;
  t: T;
}) {
  const passwordAllowed = yn(policy.password_auth);
  const kbdAllowed = yn(policy.kbd_interactive_auth);
  // Either directive may unlock password-style auth.
  const effective: YN =
    passwordAllowed === "yes" || kbdAllowed === "yes" ? "yes"
      : passwordAllowed === "no" && kbdAllowed === "no" ? "no"
      : "unknown";

  // Users who can actually log in with a password: AllowUsers/DenyUsers
  // crossed with each account's password status.
  const allow = new Set((policy.allow_users ?? []).map((s) => s.toLowerCase()));
  const deny = new Set((policy.deny_users ?? []).map((s) => s.toLowerCase()));
  const passwordUsers = effective !== "yes" ? [] : users.filter((u) => {
    const name = u.name.toLowerCase();
    return u.password_status === "P" && u.has_login && !deny.has(name) && (allow.size === 0 || allow.has(name));
  });

  const sources = policy.directive_sources ?? {};
  const rows: { key: string; label: string; value: ReactNode }[] = [
    { key: "passwordauthentication", label: t("scan.policyPasswordAuth"), value: <YesNo value={passwordAllowed} t={t} /> },
    { key: "kbdinteractiveauthentication", label: t("scan.policyKbdInteractive"), value: <YesNo value={kbdAllowed} t={t} /> },
    { key: "pubkeyauthentication", label: t("scan.policyPubkeyAuth"), value: <YesNo value={yn(policy.pubkey_auth)} t={t} /> },
    { key: "usepam", label: t("scan.policyUsePAM"), value: <YesNo value={yn(policy.use_pam)} t={t} /> },
    { key: "permitrootlogin", label: t("scan.policyPermitRoot"), value: <span className="text-xs text-[var(--text-primary)] font-mono">{policy.permit_root_login || "–"}</span> },
  ];
  if (policy.authentication_methods && policy.authentication_methods !== "any") {
    rows.push({ key: "authenticationmethods", label: t("scan.policyAuthMethods"), value: <span className="text-xs text-[var(--text-primary)] font-mono">{policy.authentication_methods}</span> });
  }
  const lists: [string, string[] | undefined][] = [
    [t("scan.policyAllowUsers"), policy.allow_users],
    [t("scan.policyAllowGroups"), policy.allow_groups],
    [t("scan.policyDenyUsers"), policy.deny_users],
    [t("scan.policyDenyGroups"), policy.deny_groups],
  ];

  return (
    <SubBlock title={t("scan.sshAuthPolicy")} aside={policy.source}>
      <div className="mb-3">
        <Badge color={effective === "yes" ? "warning" : effective === "no" ? "success" : "default"} dot>
          {effective === "yes" ? t("scan.policyPasswordAllowed") : effective === "no" ? t("scan.policyPasswordDenied") : t("scan.policyPasswordUnknown")}
        </Badge>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-6">
        {rows.map(({ key, label, value }) => (
          <div key={key} className="py-1.5 border-b border-[var(--border-subtle)]/50">
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs text-[var(--text-muted)]">{label}</span>
              {value}
            </div>
            {(sources[key] ?? []).map((h, i) => (
              <div key={i} className="mt-0.5 text-2xs text-[var(--text-faint)] truncate font-mono" title={h.value ? `${h.file}:${h.line} → ${h.value}` : `${h.file}:${h.line}`}>
                <span className="opacity-60">↳</span> {h.file}:{h.line}
                {h.value && <span className="ml-1 opacity-70">{h.value}</span>}
              </div>
            ))}
          </div>
        ))}
      </div>

      {lists.some(([, items]) => items?.length) && (
        <div className="mt-3">
          {lists.filter(([, items]) => items?.length).map(([label, items]) => (
            <div key={label} className="flex flex-wrap items-center gap-2 py-1.5">
              <span className="text-xs text-[var(--text-muted)] shrink-0">{label}</span>
              {items!.map((u) => <Badge key={u} className="font-mono">{u}</Badge>)}
            </div>
          ))}
        </div>
      )}

      {/* Nothing to list when password auth is off. */}
      {effective !== "no" && (
        <div className="mt-3">
          <span className="text-xs text-[var(--text-muted)] block mb-2">{t("scan.policyPasswordUsers")}</span>
          {passwordUsers.length === 0 ? (
            <p className="text-xs text-[var(--text-muted)]">
              {effective === "yes" ? t("scan.policyPasswordUsersNone") : t("scan.policyPasswordUsersUnknown")}
            </p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {passwordUsers.map((u) => (
                <span key={u.name} title={`uid ${u.uid} · ${u.shell || ""}`}><Badge color="warning" className="font-mono">{u.name}</Badge></span>
              ))}
            </div>
          )}
        </div>
      )}
    </SubBlock>
  );
}

function YesNo({ value, t }: { value: YN; t: T }) {
  if (value === "yes") return <Badge color="success">{t("common.yes")}</Badge>;
  if (value === "no") return <Badge color="rose">{t("common.no")}</Badge>;
  return <span className="text-xs text-[var(--text-muted)]">–</span>;
}
