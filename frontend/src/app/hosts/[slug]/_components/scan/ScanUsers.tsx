"use client";

import SectionCard from "@/components/ui/SectionCard";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { parseLoginEntry, formatLoginDate } from "@/lib/utils";
import type { VMInfoType } from "@/lib/api";
import { Rows, Row, SubBlock, type T } from "./shared";

type Key = NonNullable<VMInfoType["ssh_keys"]>[number];

// Login "users" that aren't accounts — wtmp rollover and reboot bookkeeping.
const NON_USER_LOGIN_TOKENS = new Set(["reboot", "shutdown", "wtmp", "runlevel", ""]);

/**
 * Remote accounts as compact rows (name, login/password state, key count, uid);
 * a row expands to its home, SSH keys and last logins. Scans that predate the
 * per-user discovery only carry a flat key list, shown as-is.
 */
export default function ScanUsers({ info, locale, t }: { info: VMInfoType; locale: string; t: T }) {
  const users = info.remote_users ?? [];
  const allKeys = info.ssh_keys ?? [];

  if (users.length === 0) {
    return (
      <SectionCard as="h3" title={t("scan.pane.group.users")} empty={allKeys.length === 0 ? t("scan.pane.empty.users") : undefined}>
        <KeyGroups keys={allKeys} t={t} />
      </SectionCard>
    );
  }

  const keysByUser = new Map<string, Key[]>();
  for (const k of allKeys) if (k.user) keysByUser.set(k.user, [...(keysByUser.get(k.user) ?? []), k]);
  // Keys the scan found without a user tag — shown last so nothing is dropped.
  const orphanKeys = allKeys.filter((k) => !k.user);

  // Modern scans carry last logins per user; legacy ones only the flat list,
  // grouped here best-effort (the legacy capture window is narrower).
  const hasStructuredLogins = users.some((u) => u.last_logins && u.last_logins.length > 0);
  const loginsByUser = new Map<string, { from: string; when: string }[]>();
  if (!hasStructuredLogins) {
    for (const raw of info.last_logins ?? []) {
      const { user, from, when } = parseLoginEntry(raw);
      if (NON_USER_LOGIN_TOKENS.has(user)) continue;
      const list = loginsByUser.get(user) ?? [];
      if (list.length < 5) list.push({ from, when });
      loginsByUser.set(user, list);
    }
  }

  return (
    <SectionCard as="h3" title={t("scan.pane.group.users")} count={users.length} body="flush">
      <Rows>
        {users.map((u) => {
          const userKeys = keysByUser.get(u.name) ?? [];
          const logins = u.last_logins?.length ? u.last_logins : (loginsByUser.get(u.name) ?? []);
          return (
            <Row
              key={u.name}
              summary={
                <>
                  <Icon path={ICON_PATHS.user} className={`w-3.5 h-3.5 shrink-0 ${u.is_current ? "text-[var(--cyan)]" : "text-[var(--text-faint)]"}`} aria-label={u.is_current ? t("scan.userCurrent") : undefined} />
                  <span className="font-mono font-medium text-[var(--text-primary)] truncate">{u.name}</span>
                  {!u.has_login && <Badge>{t("scan.userNoLogin")}</Badge>}
                  {u.password_status === "P" && <span title={t("scan.userPasswordSetTooltip")}><Badge color="warning">{t("scan.userPasswordSet")}</Badge></span>}
                  {u.password_status === "L" && <span title={t("scan.userPasswordLockedTooltip")}><Badge>{t("scan.userPasswordLocked")}</Badge></span>}
                  {u.password_status === "NP" && <span title={t("scan.userPasswordNoneTooltip")}><Badge color="success">{t("scan.userPasswordNone")}</Badge></span>}
                  <span className="ml-auto flex items-center gap-3 shrink-0 text-[var(--text-muted)] font-mono">
                    <span className="flex items-center gap-1" title={t("scan.sshKeys")}>
                      <Icon path={ICON_PATHS.key} className="w-3 h-3" />{userKeys.length}
                    </span>
                    <span>{t("vm.uidBadge", { uid: String(u.uid) })}</span>
                  </span>
                </>
              }
            >
              {u.home && <p className="font-mono text-[var(--text-muted)] break-all">{u.home}</p>}
              {userKeys.length === 0 ? (
                <p className="text-[var(--text-muted)]">{t("scan.userNoKeys")}</p>
              ) : (
                <KeyGroups keys={userKeys} t={t} />
              )}
              {logins.length > 0 && (
                <SubBlock title={t("scan.userLastLogins")} aside={logins.length}>
                  <div className="space-y-0.5">
                    {logins.map((l, i) => (
                      <div key={i} className="flex items-baseline gap-2 font-mono">
                        <span className="truncate text-[var(--text-primary)]" title={l.from}>{l.from || "–"}</span>
                        <span className="ml-auto shrink-0 text-[var(--text-muted)]">{formatLoginDate(l.when, locale)}</span>
                      </div>
                    ))}
                  </div>
                </SubBlock>
              )}
            </Row>
          );
        })}
        {orphanKeys.length > 0 && (
          <Row summary={<span className="font-mono text-[var(--text-muted)]">{t("scan.userOrphanKeys")} <span className="text-[var(--text-faint)]">{orphanKeys.length}</span></span>}>
            <KeyGroups keys={orphanKeys} t={t} />
          </Row>
        )}
      </Rows>
    </SectionCard>
  );
}

/** authorized_keys and private keys as two labelled lists. */
function KeyGroups({ keys, t }: { keys: Key[]; t: T }) {
  const auth = keys.filter((k) => k.source === "authorized_keys");
  const priv = keys.filter((k) => k.source !== "authorized_keys");
  return (
    <>
      {auth.length > 0 && (
        <SubBlock title={t("scan.authorizedKeys")} aside={auth.length}>
          <div className="space-y-1.5">{auth.map((k, i) => <KeyLine key={i} k={k} t={t} />)}</div>
        </SubBlock>
      )}
      {priv.length > 0 && (
        <SubBlock title={t("scan.sshKeys")} aside={priv.length}>
          <div className="space-y-1.5">{priv.map((k, i) => <KeyLine key={i} k={k} t={t} />)}</div>
        </SubBlock>
      )}
    </>
  );
}

// Key icon colour encodes managed status (success = known to Bridge); the chip
// appears only when it adds the managed key's name.
function KeyLine({ k, t }: { k: Key; t: T }) {
  return (
    <div className="flex gap-2 text-xs min-w-0">
      <Icon path={ICON_PATHS.key} className={`w-3.5 h-3.5 shrink-0 mt-0.5 ${k.managed ? "text-[var(--success)]" : "text-[var(--text-faint)]"}`} aria-label={k.managed ? t("scan.managed") : t("scan.unmanaged")} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="text-[var(--text-primary)] truncate font-mono">{k.name || "–"}</span>
          <span className="text-2xs text-[var(--text-faint)]">{k.type}</span>
          {k.managed && k.managed_name && <Badge color="success">{k.managed_name}</Badge>}
        </div>
        <div className="text-2xs text-[var(--text-muted)] break-all leading-snug font-mono">{k.fingerprint}</div>
      </div>
    </div>
  );
}
