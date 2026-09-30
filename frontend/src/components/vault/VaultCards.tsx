"use client";

import { useState } from "react";
import Card from "@/components/ui/Card";
import Icon from "@/components/ui/Icon";
import RowActions, { type RowAction } from "@/components/ui/RowActions";
import { ICON_PATHS } from "@/lib/icon-paths";
import { rowLead, type VaultRow } from "@/lib/vaultGroups";
import { scopeLabel } from "./SecretDetailDrawer";
import { TYPE_ICON, rowName, rowUse } from "./VaultTable";
import type { Secret } from "@/lib/types";

type T = (k: string, v?: Record<string, string>) => string;

/** The vault on a phone: the table's rows as cards, groups expanding in place. */
export default function VaultCards({ rows, onOpen, actionsFor, t }: {
  rows: VaultRow[];
  onOpen: (s: Secret) => void;
  actionsFor: (s: Secret) => RowAction[];
  t: T;
}) {
  const [open, setOpen] = useState<Set<string>>(new Set());
  const toggle = (k: string) => setOpen((e) => { const n = new Set(e); if (n.has(k)) n.delete(k); else n.add(k); return n; });

  const line = (s: Secret, title: string, meta: string, onClick: () => void, trailing?: React.ReactNode, lead?: React.ReactNode) => (
    <div className="flex items-center gap-3">
      <button type="button" onClick={onClick} className="flex items-center gap-3 min-w-0 flex-1 text-left">
        {lead}
        <Icon path={TYPE_ICON[s.type] ?? ICON_PATHS.key} className="w-4 h-4 shrink-0 text-[var(--text-muted)]" />
        <span className="min-w-0">
          <span className={`block text-sm truncate text-[var(--text-primary)] ${s.type === "env_var" ? "font-mono" : "font-medium"}`}>{title}</span>
          <span className="block text-xs text-[var(--text-muted)] truncate">{meta}</span>
        </span>
      </button>
      {trailing}
    </div>
  );

  return (
    <div className="space-y-2">
      {rows.map((r) => {
        const s = rowLead(r);
        const use = rowUse(r, t);
        if (r.kind === "secret") {
          return (
            <Card key={r.key} className="!py-3">
              {line(s, s.name, [t(`vault.type.${s.type}`), scopeLabel(s, t), use].filter(Boolean).join(" · "), () => onOpen(s),
                <RowActions name={s.name} actions={actionsFor(s)} />)}
            </Card>
          );
        }
        const expanded = open.has(r.key);
        return (
          <Card key={r.key} className="!py-3">
            {line(s, rowName(r, t), [r.kind === "bundle" ? scopeLabel(s, t) : t("vault.severalPlaces"), use].join(" · "), () => toggle(r.key), undefined,
              <Icon path={ICON_PATHS.chevronRight} className={`w-3.5 h-3.5 shrink-0 text-[var(--text-muted)] transition-transform ${expanded ? "rotate-90" : ""}`} />)}
            {expanded && (
              <div className="mt-3 pt-2 border-t border-[var(--border-subtle)] divide-y divide-[var(--border-subtle)]">
                {r.items.map((m) => (
                  <div key={m.id} className="py-2 pl-6">
                    {line(m, m.name, scopeLabel(m, t), () => onOpen(m))}
                  </div>
                ))}
              </div>
            )}
          </Card>
        );
      })}
    </div>
  );
}
