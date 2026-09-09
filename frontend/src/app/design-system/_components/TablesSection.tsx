"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import SortableTable, { sortRows, type SortableColumn } from "@/components/ui/SortableTable";
import Badge from "@/components/ui/Badge";
import Pagination from "@/components/ui/Pagination";
import Field from "@/components/ui/Field";
import Accordion from "@/components/ui/Accordion";
import DrawerSection from "@/components/ui/DrawerSection";
import RequestTimeline from "@/app/requests/[id]/_components/RequestTimeline";
import type { RequestEvent } from "@/lib/types";

type RowKey = "name" | "cpu" | "situacao";
type Row = { id: number; name: string; cpu: number; situacao: string };

const ROWS: Row[] = [
  { id: 1, name: "web-01", cpu: 42, situacao: "active" },
  { id: 2, name: "db-01", cpu: 87, situacao: "maintenance" },
  { id: 3, name: "cache-01", cpu: 12, situacao: "inactive" },
];

const TABLE_COLUMNS: SortableColumn<RowKey>[] = [
  { key: "name", label: "Host" },
  { key: "cpu", label: "CPU %", align: "right" },
  { key: "situacao", label: "Situação", sortable: false },
];

// roleColors: copied verbatim from app/settings/page.tsx:30-34 (components/layout/Header.tsx
// duplicates the same map).
const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-purple-500/10 text-purple-400/70 border-purple-500/15",
  viewer: "bg-[var(--bg-overlay)] text-[var(--text-faint)] border-[var(--border-subtle)]",
};

type SpecimenUser = {
  id: number;
  display_name: string;
  username: string;
  role: string;
  entidades: { id: number; name: string; is_primary: boolean }[];
};

const SPECIMEN_USERS: SpecimenUser[] = [
  { id: 1, display_name: "Ana Souza", username: "ana", role: "admin", entidades: [{ id: 1, name: "SEAD-PI", is_primary: true }] },
  { id: 2, display_name: "Carlos Lima", username: "carlos", role: "editor", entidades: [{ id: 2, name: "TI", is_primary: false }] },
  { id: 3, display_name: "Bia Rocha", username: "bia", role: "viewer", entidades: [] },
];

const ACCORDION_SECTIONS = [
  { id: "basics", title: "Basics", content: <p className="text-sm text-[var(--text-secondary)]">Hostname, IP, and situação.</p> },
  { id: "scope", title: "Scope", content: <p className="text-sm text-[var(--text-secondary)]">Entidade grants for this asset.</p> },
];

const REQUEST_EVENTS: RequestEvent[] = [
  { id: 1, request_id: 1, user_id: 1, user_name: "Ana Souza", kind: "status", body: "", from_status: "", to_status: "submitted", created_at: "2026-09-01T10:00:00Z" },
  { id: 2, request_id: 1, user_id: 2, user_name: "Carlos Lima", kind: "comment", body: "Can you clarify the requested capacity?", from_status: "", to_status: "", created_at: "2026-09-02T09:15:00Z" },
  { id: 3, request_id: 1, user_id: 1, user_name: "Ana Souza", kind: "status", body: "", from_status: "submitted", to_status: "approved", created_at: "2026-09-03T14:30:00Z" },
  { id: 4, request_id: 1, user_id: 1, user_name: "Ana Souza", kind: "delivery", body: "Provisioned successfully.", from_status: "", to_status: "", created_at: "2026-09-04T11:00:00Z" },
];

export default function TablesSection() {
  const [page, setPage] = useState(2);
  const [drawerSectionOpen, setDrawerSectionOpen] = useState(true);

  return (
    <Section id="tables" title="Tables & Lists">
      <Specimen title="SortableTable" source="components/ui/SortableTable.tsx" wide>
        <SortableTable columns={TABLE_COLUMNS} defaultSort="name">
          {(sortKey, sortDir) => {
            const sorted = sortRows(ROWS, sortKey, sortDir, {
              name: (a, b) => a.name.localeCompare(b.name),
              cpu: (a, b) => a.cpu - b.cpu,
              situacao: () => 0,
            });
            return sorted.map((r) => (
              <tr key={r.id} className="border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)]">
                <td className="px-4 py-2.5 text-xs">{r.name}</td>
                <td className="px-4 py-2.5 text-xs text-right">{r.cpu}</td>
                <td className="px-4 py-2.5 text-xs">
                  <Badge variant="situacao" situacao={r.situacao} compact>{r.situacao}</Badge>
                </td>
              </tr>
            ));
          }}
        </SortableTable>
        <p className="text-[11px] text-[var(--text-muted)] mt-2">
          Controlled mode (<code>sortKey</code>/<code>sortDir</code>/<code>onSortChange</code>) is what the
          inventory table views use for server-side sort; no sticky header, no selection, no built-in empty
          state (parents render <code>EmptyState</code>).
        </p>
      </Specimen>

      <Specimen
        title="Hand-rolled table"
        source="app/settings/page.tsx:482-532"
        alsoIn={[
          "app/contacts/page.tsx:90",
          "app/issues/IssueBoard.tsx:750 (own SortIcon)",
          "app/settings/EntidadesTab.tsx:61 (tree-as-table)",
          "app/settings/PermissionsTab.tsx:94 (matrix)",
          "app/settings/RoleMappingsTab.tsx:66",
        ]}
        wide
      >
        {/* specimen: app/settings/page.tsx:482-532 */}
        <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto animate-fade-in">
          {/* ponytail: nested overflow-x-auto so the min-w-[600px] table clips here, not at <main> */}
          <div className="overflow-x-auto">
            <table className="w-full text-sm min-w-[600px]">
              <thead>
                <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
                  <th className="text-left px-4 py-3 font-semibold">Username</th>
                  <th className="text-left px-4 py-3 font-semibold">Role</th>
                  <th className="text-left px-4 py-3 font-semibold">Entidades</th>
                </tr>
              </thead>
              <tbody>
                {SPECIMEN_USERS.map((u, i) => (
                  <tr
                    key={u.id}
                    className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
                  >
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-2.5">
                        <div className="w-8 h-8 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-xs font-semibold text-[var(--text-secondary)] shrink-0">
                          {(u.display_name || u.username).charAt(0).toUpperCase()}
                        </div>
                        <div className="min-w-0">
                          <p className="text-sm font-medium text-[var(--text-primary)] truncate">{u.display_name || u.username}</p>
                          <p className="text-[11px] text-[var(--text-faint)]">@{u.username}</p>
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-2.5">
                      <span className={`text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[u.role] || roleColors.viewer}`}>
                        {u.role}
                      </span>
                    </td>
                    <td className="px-4 py-2.5 text-xs">
                      <div className="flex flex-wrap gap-1">
                        {u.entidades.map((e) => (
                          <span
                            key={e.id}
                            className={`px-1.5 py-0.5 rounded border text-[10px] ${e.is_primary ? "border-[var(--accent)]/40 text-[var(--accent)] bg-[var(--accent-muted)]" : "border-[var(--border-subtle)] text-[var(--text-muted)]"}`}
                            title={e.is_primary ? "Primary" : undefined}
                          >
                            {e.name}
                          </span>
                        ))}
                        {u.entidades.length === 0 && <span className="text-[var(--text-faint)]">-</span>}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
        <p className="text-[11px] text-[var(--text-muted)] mt-2">
          6 hand-rolled tables re-implement the same thead/zebra recipe; no <code>TableRow</code>/<code>TableCell</code>.
        </p>
      </Specimen>

      <Specimen title="Pagination" source="components/ui/Pagination.tsx">
        <Pagination page={page} totalPages={5} total={92} perPage={20} onChange={setPage} />
      </Specimen>

      <Specimen title="Field" source="components/ui/Field.tsx">
        <Field label="Hostname" value="web-01" mono />
        <Field label="URL" link href="#tables" value="https://bridge.gov.pi/hosts/web-01" />
        <Field label="Empty" value="" />
        <p className="text-[11px] text-[var(--text-muted)] w-full">
          Read-only label/value display — not a form field (name collision with the local <code>Field</code> in{" "}
          <code>app/ssh-keys/page.tsx:469</code>).
        </p>
      </Specimen>

      <Specimen
        title="Local KV helper"
        source="app/ssh-keys/page.tsx:469-478"
        alsoIn={[
          "app/hosts/_components/BatchDockerLogsModal.tsx:290 Stat",
          "components/atlas/pipeline/PipelineNodeDetail.tsx:221 KV",
          "components/atlas/catalog/TableDetailPanel.tsx:135 Stat",
          "components/lineage/TablesPanel.tsx:154 Row",
          "components/vault/VaultPage.tsx:182 FilterRow",
          "app/hosts/FilterDrawer.tsx:164 FieldLabel",
          "<dl> in components/requests/RequestAnswers.tsx, app/requests/[id]/RequestDetail.tsx:241, app/catalog/_components/RequestFormModal.tsx:54",
        ]}
      >
        {/* specimen: app/ssh-keys/page.tsx:469-478 */}
        <div>
          <span className="text-xs text-[var(--text-muted)]">Key type</span>
          <p className="text-sm text-[var(--text-secondary)] " style={{ fontFamily: "var(--font-mono)" }}>
            ed25519
          </p>
        </div>
        <div>
          <span className="text-xs text-[var(--text-muted)]">Comment</span>
          <p className="text-sm text-[var(--text-secondary)] ">admin@bridge</p>
        </div>
      </Specimen>

      <Specimen
        title="RequestTimeline"
        source="app/requests/[id]/_components/RequestTimeline.tsx"
        alsoIn={["app/secrets/_components/HistoryDrawer.tsx (guardrailed path)", "components/glpi/TicketDetailDrawer.tsx", "app/releases/page.tsx"]}
        wide
      >
        <RequestTimeline events={REQUEST_EVENTS} />
        <p className="text-[11px] text-[var(--text-faint)] mt-6 mb-2">Empty state:</p>
        <RequestTimeline events={[]} />
      </Specimen>

      <Specimen title="Accordion" source="components/ui/Accordion.tsx">
        <Accordion sections={ACCORDION_SECTIONS} />
        <p className="text-[11px] text-[var(--text-muted)] mt-2 w-full">Zero consumers in the app — dead code.</p>
      </Specimen>

      <Specimen title="DrawerSection" source="components/ui/DrawerSection.tsx">
        <DrawerSection title="Filters" open={drawerSectionOpen} onToggle={() => setDrawerSectionOpen((o) => !o)} active>
          <p className="text-sm text-[var(--text-secondary)]">Situação: Active</p>
        </DrawerSection>
        <p className="text-[11px] text-[var(--text-muted)] mt-2 w-full">The collapsible actually used (filter drawers).</p>
      </Specimen>

      <div>
        <p className="text-xs text-[var(--text-muted)] mb-1">Describe only:</p>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            Trees ×4 — <code>components/wiki/WikiTree.tsx</code> (role=tree), <code>components/atlas/catalog/DomainTree.tsx</code>,{" "}
            <code>app/settings/EntidadesTab.tsx</code> (flattened via <code>lib/entidades.ts withDepth</code>),{" "}
            <code>components/atlas/apis/ShareBundleModal.tsx</code>
          </li>
          <li>
            List renderers with private formatters — <code>components/glpi/TicketList.tsx</code> (own{" "}
            <code>formatDate</code> + <code>statusColor</code>), <code>components/inventory/ResponsavelList.tsx</code> (own{" "}
            <code>formatPhone</code>, duplicated in <code>app/contacts/page.tsx</code>)
          </li>
          <li>
            <code>components/inventory/InventoryContent.tsx</code> — loading/empty/cards/table state machine + infinite scroll
          </li>
          <li>No bytes/number formatter and no truncate helper in <code>src/lib</code></li>
        </ul>
      </div>
    </Section>
  );
}
