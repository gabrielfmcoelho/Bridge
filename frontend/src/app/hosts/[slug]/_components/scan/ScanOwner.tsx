"use client";

import type { ReactNode } from "react";
import SectionCard from "@/components/ui/SectionCard";
import { useLocale } from "@/contexts/LocaleContext";
import type { OwnerClues, VMInfoType } from "@/lib/api";
import { SubBlock, type T } from "./shared";

/**
 * A8 owner clues: leads to who owns or maintains a VM nobody claims — last
 * logins (with year), account full names (GECOS), sudo/wheel members, SSH
 * authorized_keys comments, and FTP uploaders per month. Each block names its
 * source. No client address, SSH key material or filename leaves the host.
 * Background scan only.
 */
export default function ScanOwner({ info, t }: { info: VMInfoType; t: T }) {
  const o = info.deep_facts?.owner;
  const empty = info.deep_facts == null
    ? t("scan.deep.notRun")
    : !o
      ? t("scan.pane.empty.owner")
      : undefined;

  return (
    <SectionCard as="h3" title={t("scan.pane.group.owner")} empty={empty}>
      {o && !empty && (
        <div className="space-y-4">
          <p className="text-2xs text-[var(--text-muted)]">{t("scan.deep.owner.summary")}</p>
          <LastLogins o={o} t={t} />
          {o.gecos && o.gecos.length > 0 && (
            <SubBlock title={t("scan.deep.owner.gecos")}>
              <Pairs items={o.gecos.map((g) => ({ k: g.user, v: g.gecos }))} />
            </SubBlock>
          )}
          {o.sudoers && o.sudoers.length > 0 && (
            <SubBlock title={t("scan.deep.owner.sudoers")}>
              <Chips items={o.sudoers} />
            </SubBlock>
          )}
          {o.auth_keys && o.auth_keys.length > 0 && (
            <SubBlock title={t("scan.deep.owner.authKeys")}>
              <Pairs items={o.auth_keys.map((k) => ({ k: k.user, v: k.comment }))} />
            </SubBlock>
          )}
          {o.ftp && o.ftp.length > 0 && (
            <SubBlock title={t("scan.deep.owner.ftp")}>
              <Pairs items={o.ftp.map((f) => ({ k: f.user, v: `${f.month} · ${t("scan.deep.owner.ftpCount", { count: f.upload.toLocaleString() })}` }))} />
            </SubBlock>
          )}
        </div>
      )}
    </SectionCard>
  );
}

function LastLogins({ o, t }: { o: OwnerClues; t: T }) {
  const { formatDate } = useLocale();
  if (!o.last_logins || o.last_logins.length === 0) return null;
  return (
    <SubBlock title={t("scan.deep.owner.lastLogins")}>
      <Pairs items={o.last_logins.map((l) => ({ k: l.user, v: formatDate(new Date(l.when * 1000)) }))} />
    </SubBlock>
  );
}

/** A compact grid of user → value rows. */
function Pairs({ items }: { items: { k: string; v: ReactNode }[] }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-xs">
      {items.map((it, i) => (
        <div key={i} className="contents">
          <dt className="font-mono text-[var(--text-secondary)]">{it.k}</dt>
          <dd className="min-w-0 break-words text-[var(--text-muted)]">{it.v}</dd>
        </div>
      ))}
    </dl>
  );
}

function Chips({ items }: { items: string[] }) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {items.map((u) => (
        <span key={u} className="rounded px-1.5 py-0.5 text-2xs font-mono bg-[var(--bg-elevated)] text-[var(--text-secondary)]">{u}</span>
      ))}
    </div>
  );
}
