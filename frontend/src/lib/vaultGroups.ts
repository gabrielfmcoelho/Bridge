import type { Secret } from "./types";

/**
 * Vault list rows: a secret on its own, an env-var bundle (the variables of
 * one environment on one asset), or the same credential repeated in several
 * places (dup_group from the API). Each secret lands in exactly one row;
 * bundles win over repetition since env vars never carry dup_group.
 */
export type VaultRow =
  | { kind: "secret"; key: string; secret: Secret }
  | { kind: "bundle"; key: string; label: string; lead: Secret; items: Secret[] }
  | { kind: "repeated"; key: string; lead: Secret; items: Secret[] };

export function groupVault(secrets: Secret[]): VaultRow[] {
  const rows: VaultRow[] = [];
  const bundles = new Map<string, Extract<VaultRow, { kind: "bundle" }>>();
  const repeated = new Map<string, Extract<VaultRow, { kind: "repeated" }>>();
  for (const s of secrets) {
    if (s.type === "env_var") {
      const key = `b:${s.scope}:${s.parent_id ?? 0}:${s.visibility}:${s.group_label ?? ""}`;
      let row = bundles.get(key);
      if (!row) {
        row = { kind: "bundle", key, label: s.group_label ?? "", lead: s, items: [] };
        bundles.set(key, row);
        rows.push(row);
      }
      row.items.push(s);
    } else if (s.dup_group && (s.dup_count ?? 0) > 1) {
      const key = `r:${s.dup_group}`;
      let row = repeated.get(key);
      if (!row) {
        row = { kind: "repeated", key, lead: s, items: [] };
        repeated.set(key, row);
        rows.push(row);
      }
      row.items.push(s);
      // The shared (avulso) copy names the group: it's the one to keep.
      if (s.scope === "avulso" && row.lead.scope !== "avulso") row.lead = s;
    } else {
      rows.push({ kind: "secret", key: `s:${s.id}`, secret: s });
    }
  }
  // A "group" of one (the others filtered out) is just that secret.
  return rows.map((r) => (r.kind === "repeated" && r.items.length === 1 ? { kind: "secret", key: `s:${r.lead.id}`, secret: r.lead } : r));
}

/** The secret a row stands for when sorting or searching. */
export const rowLead = (r: VaultRow): Secret => (r.kind === "secret" ? r.secret : r.lead);
