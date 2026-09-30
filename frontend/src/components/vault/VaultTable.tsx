"use client";

import { Fragment, useState } from "react";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import { tableClasses } from "@/components/ui/Table";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import RowActions, { type RowAction } from "@/components/ui/RowActions";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import { rowLead, type VaultRow } from "@/lib/vaultGroups";
import { scopeLabel } from "./SecretDetailDrawer";
import type { Secret } from "@/lib/types";

type Col = "name" | "type" | "scope" | "visibility" | "use" | "updated" | "actions";
type T = (k: string, v?: Record<string, string>) => string;

export const TYPE_ICON: Record<string, string> = {
  sshkey: ICON_PATHS.keyOutline, password: ICON_PATHS.lockOutline, cred: ICON_PATHS.user,
  app_login: ICON_PATHS.globe, env_var: ICON_PATHS.code, api_key: ICON_PATHS.keyOutline,
};

/** What a row says in the "Uso" column. */
export function rowUse(r: VaultRow, t: T): string {
  if (r.kind === "bundle") return t("vault.varsCount", { count: String(r.items.length) });
  if (r.kind === "repeated") return t("vault.repeatedIn", { count: String(r.items.length) });
  const n = r.secret.linked_hosts ?? 0;
  return n > 0 ? t("vault.inHosts", { count: String(n) }) : "";
}

export function rowName(r: VaultRow, t: T): string {
  return r.kind === "bundle" ? t("vault.bundleName", { group: r.label }) : rowLead(r).name;
}

/**
 * The vault as a table: one line per secret, env-var bundle or repeated
 * credential; bundles and repetitions expand in place to their members.
 * Clicking a secret opens its detail drawer; "…" holds the rest.
 */
export default function VaultTable({ rows, onOpen, actionsFor, t, locale }: {
  rows: VaultRow[];
  onOpen: (s: Secret) => void;
  actionsFor: (s: Secret) => RowAction[];
  t: T;
  locale: string;
}) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const toggle = (k: string) => setExpanded((e) => { const n = new Set(e); if (n.has(k)) n.delete(k); else n.add(k); return n; });
  const cmp = (f: (s: Secret) => string) => (a: VaultRow, b: VaultRow) => f(rowLead(a)).localeCompare(f(rowLead(b)), locale);

  const cells = (s: Secret, r: VaultRow | null, child: boolean) => (
    <>
      <td className={`${tableClasses.td} ${child ? "pl-10" : ""}`}>
        <div className="flex items-center gap-2 min-w-0">
          <Icon path={TYPE_ICON[s.type] ?? ICON_PATHS.key} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)]" />
          <span className="min-w-0">
            <span className={`block truncate text-[var(--text-primary)] ${s.type === "env_var" ? "font-mono" : "font-medium"}`}>
              {r ? rowName(r, t) : s.name}
            </span>
            {s.username && <span className="block text-2xs font-mono text-[var(--text-muted)] truncate">{s.username}</span>}
          </span>
        </div>
      </td>
      <td className={`${tableClasses.td} text-[var(--text-secondary)] whitespace-nowrap`}>{t(`vault.type.${s.type}`)}</td>
      <td className={`${tableClasses.td} text-[var(--text-secondary)] max-w-[16rem] truncate`}>{scopeLabel(s, t)}</td>
      <td className={tableClasses.td}>
        <Badge color={s.visibility === "personal" ? "purple" : undefined}>{t(`vault.visibility.${s.visibility}`)}</Badge>
      </td>
      <td className={`${tableClasses.td} text-[var(--text-secondary)] whitespace-nowrap`}>
        {r ? rowUse(r, t) : (s.linked_hosts ? t("vault.inHosts", { count: String(s.linked_hosts) }) : "")}
      </td>
      <td className={`${tableClasses.td} text-[var(--text-muted)] whitespace-nowrap tabular-nums`}>{getTimeAgo(s.updated_at, locale)}</td>
      <td className={`${tableClasses.td} text-right`} onClick={(e) => e.stopPropagation()}>
        {(!r || r.kind === "secret") && <RowActions name={s.name} actions={actionsFor(s)} />}
      </td>
    </>
  );

  return (
    <SortableTable<Col>
      columns={[
        { key: "name", label: t("common.name") },
        { key: "type", label: t("secretForm.type") },
        { key: "scope", label: t("secretForm.scope") },
        { key: "visibility", label: t("secretForm.visibility") },
        { key: "use", label: t("vault.use"), sortable: false },
        { key: "updated", label: t("vault.updated") },
        { key: "actions", label: "", sortable: false },
      ]}
      defaultSort="name"
    >
      {(key, dir) => sortRows(rows, key, dir, {
        name: (a, b) => rowName(a, t).localeCompare(rowName(b, t), locale),
        type: cmp((s) => t(`vault.type.${s.type}`)),
        scope: cmp((s) => scopeLabel(s, t)),
        visibility: cmp((s) => s.visibility),
        use: () => 0,
        updated: cmp((s) => s.updated_at),
        actions: () => 0,
      }).map((r) => {
        const lead = rowLead(r);
        const group = r.kind !== "secret";
        const open = expanded.has(r.key);
        return (
          <Fragment key={r.key}>
            <tr className={`${tableClasses.row} border-t border-[var(--border-subtle)] cursor-pointer`}
              onClick={() => (group ? toggle(r.key) : onOpen(lead))}
              aria-expanded={group ? open : undefined}>
              {group ? (
                <>
                  <td className={tableClasses.td}>
                    <div className="flex items-center gap-2 min-w-0">
                      <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 shrink-0 text-[var(--text-muted)] transition-transform ${open ? "rotate-90" : ""}`} />
                      <Icon path={TYPE_ICON[lead.type] ?? ICON_PATHS.key} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)]" />
                      <span className="font-medium text-[var(--text-primary)] truncate">{rowName(r, t)}</span>
                    </div>
                  </td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)] whitespace-nowrap`}>{t(`vault.type.${lead.type}`)}</td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)] max-w-[16rem] truncate`}>
                    {r.kind === "bundle" ? scopeLabel(lead, t) : t("vault.severalPlaces")}
                  </td>
                  <td className={tableClasses.td}>
                    <Badge color={lead.visibility === "personal" ? "purple" : undefined}>{t(`vault.visibility.${lead.visibility}`)}</Badge>
                  </td>
                  <td className={`${tableClasses.td} whitespace-nowrap ${r.kind === "repeated" ? "text-[var(--warning)]" : "text-[var(--text-secondary)]"}`}>{rowUse(r, t)}</td>
                  <td className={`${tableClasses.td} text-[var(--text-muted)] whitespace-nowrap tabular-nums`}>{getTimeAgo(lead.updated_at, locale)}</td>
                  <td className={tableClasses.td} />
                </>
              ) : cells(lead, r, false)}
            </tr>
            {group && open && r.items.map((s) => (
              <tr key={s.id} className={`${tableClasses.row} border-t border-[var(--border-subtle)] cursor-pointer bg-[var(--bg-elevated)]/40`} onClick={() => onOpen(s)}>
                {cells(s, null, true)}
              </tr>
            ))}
          </Fragment>
        );
      })}
    </SortableTable>
  );
}
