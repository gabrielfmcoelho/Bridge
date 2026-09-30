"use client";

import { useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Icon from "@/components/ui/Icon";
import RowActions, { type RowAction } from "@/components/ui/RowActions";
import { CardHeader, CardMetadataGrid, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import { rowLead, type VaultRow } from "@/lib/vaultGroups";
import { scopeLabel } from "./SecretDetailDrawer";
import { TYPE_ICON, rowName, rowUse } from "./VaultTable";
import type { Secret } from "@/lib/types";

type T = (k: string, v?: Record<string, string>) => string;

/** The type colours the card stripe: host credentials, logins, variables. */
const TYPE_ACCENT: Record<string, string> = { sshkey: "info", password: "info", cred: "warning", app_login: "cyan", env_var: "success" };

/**
 * The vault as inventory cards (header · metadata grid · indicators). A
 * secret opens its drawer; an env-var bundle or a repeated credential is one
 * card that lists its members.
 */
export default function VaultCards({ rows, onOpen, actionsFor, t }: {
  rows: VaultRow[];
  onOpen: (s: Secret) => void;
  actionsFor: (s: Secret) => RowAction[];
  t: T;
}) {
  const { locale } = useLocale();
  const [open, setOpen] = useState<Set<string>>(new Set());
  const toggle = (k: string) => setOpen((e) => { const n = new Set(e); if (n.has(k)) n.delete(k); else n.add(k); return n; });

  const indicators = (s: Secret, members?: number) => (
    <CardIndicatorGrid>
      <CardIndicator icon={ICON_PATHS.server} count={s.linked_hosts ?? 0} color="info" title={t("vault.inHosts", { count: String(s.linked_hosts ?? 0) })} />
      <CardIndicator icon={ICON_PATHS.copy} count={(s.dup_count ?? 0) > 1 ? s.dup_count! : 0} color="warning"
        title={(s.dup_count ?? 0) > 1 ? t("vault.repeatedTitle", { count: String(s.dup_count) }) : t("vault.notRepeated")} />
      {members != null && <CardIndicator icon={ICON_PATHS.code} count={members} color="success" title={t("vault.varsCount", { count: String(members) })} />}
      <CardIndicator icon={s.visibility === "personal" ? ICON_PATHS.user : ICON_PATHS.building} count={1} hideCount
        color={s.visibility === "personal" ? "purple" : "info"} title={t(`vault.visibility.${s.visibility}`)} />
    </CardIndicatorGrid>
  );

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
      {rows.map((r) => {
        const s = rowLead(r);
        const accent = TYPE_ACCENT[s.type] ?? "accent";
        if (r.kind === "secret") {
          return (
            <Card key={r.key} accent={accent} hover onClick={() => onOpen(s)} className="h-full flex flex-col overflow-hidden cursor-pointer">
              <CardHeader
                titleFont="display"
                title={s.name}
                subtitle={s.username || undefined}
                status={<span className="inline-flex items-center gap-1.5 text-xs text-[var(--text-secondary)]">
                  <Icon path={TYPE_ICON[s.type] ?? ICON_PATHS.key} className="w-3.5 h-3.5" />{t(`vault.type.${s.type}`)}
                </span>}
                description={s.description ?? undefined}
                corner={<span onClick={(e) => e.stopPropagation()}><RowActions name={s.name} actions={actionsFor(s)} /></span>}
              />
              <CardMetadataGrid items={[
                { label: t("secretForm.scope"), value: scopeLabel(s, t) },
                { label: t("secretForm.visibility"), value: t(`vault.visibility.${s.visibility}`) },
                { label: t("vault.owner"), value: s.owner_name ?? "" },
                { label: t("vault.updated"), value: getTimeAgo(s.updated_at, locale) },
              ]} />
              {indicators(s)}
            </Card>
          );
        }
        const expanded = open.has(r.key);
        return (
          <Card key={r.key} accent={r.kind === "repeated" ? "warning" : accent} hover onClick={() => toggle(r.key)}
            className="h-full flex flex-col overflow-hidden cursor-pointer">
            <CardHeader
              titleFont={r.kind === "bundle" ? "mono" : "display"}
              title={rowName(r, t)}
              subtitle={r.kind === "bundle" ? scopeLabel(s, t) : t("vault.severalPlaces")}
              subtitleFont="display"
              status={<span className={`text-xs ${r.kind === "repeated" ? "text-[var(--warning)]" : "text-[var(--text-secondary)]"}`}>{rowUse(r, t)}</span>}
              description={r.kind === "repeated" ? t("vault.repeatedBody") : undefined}
            />
            <CardMetadataGrid items={[
              { label: t("secretForm.type"), value: t(`vault.type.${s.type}`) },
              { label: t("secretForm.visibility"), value: t(`vault.visibility.${s.visibility}`) },
              { label: t("vault.owner"), value: s.owner_name ?? "" },
              { label: t("vault.updated"), value: getTimeAgo(r.items.reduce((m, x) => (x.updated_at > m ? x.updated_at : m), s.updated_at), locale) },
            ]} />
            <button type="button" aria-expanded={expanded} onClick={(e) => { e.stopPropagation(); toggle(r.key); }}
              className="mt-3 flex items-center gap-1.5 text-xs text-[var(--accent)] hover:underline self-start">
              <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 transition-transform ${expanded ? "rotate-90" : ""}`} />
              {t(expanded ? "vault.hideMembers" : "vault.showMembers", { count: String(r.items.length) })}
            </button>
            {expanded && (
              <ul className="mt-2 divide-y divide-[var(--border-subtle)] border-t border-[var(--border-subtle)]" onClick={(e) => e.stopPropagation()}>
                {r.items.map((m) => (
                  <li key={m.id}>
                    <button type="button" onClick={() => onOpen(m)} className="w-full flex items-center gap-2 py-2 text-left hover:bg-[var(--bg-elevated)] rounded-[var(--radius-sm)] px-1">
                      <span className={`min-w-0 flex-1 truncate text-sm text-[var(--text-primary)] ${m.type === "env_var" ? "font-mono" : ""}`}>{m.name}</span>
                      <span className="text-xs text-[var(--text-muted)] truncate max-w-[50%]">{r.kind === "repeated" ? scopeLabel(m, t) : m.description || ""}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {indicators(s, r.kind === "bundle" ? r.items.length : undefined)}
          </Card>
        );
      })}
    </div>
  );
}
