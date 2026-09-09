"use client";

import { Section, Specimen } from "./Section";
import Card, { CardIcon } from "@/components/ui/Card";
import StatCard from "@/components/ui/StatCard";
import KpiGrid from "@/components/inventory/KpiGrid";
import { CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator } from "@/components/inventory";
import OfferingCard from "@/app/catalog/_components/OfferingCard";
import TableCard from "@/components/atlas/catalog/TableCard";
import LinkedEntityList from "@/components/ui/LinkedEntityList";
import Badge from "@/components/ui/Badge";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { Offering } from "@/lib/types";
import type { TableRecord } from "@/lib/atlas/types";

const noop = () => {};

const CARD_ACCENTS = ["success", "warning", "danger", "info", "cyan", "purple", "rose", "accent", "muted"] as const;
const STAT_CARD_COLORS = ["cyan", "emerald", "purple", "amber", "red", "sky", "rose"] as const;

const OFFERING_1: Offering = {
  id: 1,
  slug: "vm-standard",
  name: "Standard VM",
  category: "Compute",
  description: "General-purpose virtual machine sized for typical workloads.",
  request_type: "vm",
  use_cases: ["Internal web app", "Batch job runner"],
  form_schema: { fields: [] },
  glpi_mode: "inherit",
  is_active: true,
  sort_order: 1,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const OFFERING_2: Offering = {
  ...OFFERING_1,
  id: 2,
  slug: "dns-record",
  name: "DNS Record",
  category: "Network",
  description: "New A/CNAME record under a managed zone.",
  request_type: "dns",
  use_cases: ["New subdomain", "Service alias"],
  sort_order: 2,
};

const TABLE_RECORD: TableRecord = {
  node: {
    id: "gold.dim_users",
    type: "table",
    label: "dim_users",
    data: { catalog: "prod", schema: "gold", table: "dim_users" },
  },
  namespace: "servidores",
  layer: "gold",
  role: "built",
  columnCount: 14,
  hasWarning: false,
};

export default function CardsSection() {
  return (
    <Section id="cards" title="Cards">
      <Specimen title="Card" source="components/ui/Card.tsx" wide>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
          {CARD_ACCENTS.map((accent) => (
            <Card key={accent} accent={accent}>
              <p className="text-sm font-semibold" style={{ fontFamily: "var(--font-mono)" }}>
                accent={accent}
              </p>
              <p className="text-xs text-[var(--text-muted)] mt-1">Left stripe from the token; legacy hue keys still resolve.</p>
            </Card>
          ))}
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3">
          <Card accent="info" decorator="stripe-top" padding="sm">
            <p className="text-sm font-semibold font-mono">decorator=&quot;stripe-top&quot;</p>
            <p className="text-xs text-[var(--text-muted)] mt-1">Atlas TableCard stripe.</p>
          </Card>
          <Card accent="purple" decorator="tint" padding="sm">
            <p className="text-sm font-semibold font-mono relative">decorator=&quot;tint&quot;</p>
            <p className="text-xs text-[var(--text-muted)] mt-1 relative">Border and wash from the accent (StatCard).</p>
          </Card>
          <Card as="button" accent="cyan" decorator="none" onClick={noop} selected>
            <div className="flex items-center gap-3">
              <CardIcon path={ICON_PATHS.server} />
              <div>
                <p className="text-sm font-semibold font-mono">as=&quot;button&quot; selected</p>
                <p className="text-xs text-[var(--text-muted)]">CardIcon takes the accent on hover.</p>
              </div>
            </div>
          </Card>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3">
          <Card hover={false}>
            <p className="text-sm font-semibold" style={{ fontFamily: "var(--font-mono)" }}>
              hover=false
            </p>
            <p className="text-xs text-[var(--text-muted)] mt-1">No lift on hover.</p>
          </Card>
          <Card clickIndicator="link" onClick={noop}>
            <p className="text-sm font-semibold" style={{ fontFamily: "var(--font-mono)" }}>
              clickIndicator=&quot;link&quot;
            </p>
            <p className="text-xs text-[var(--text-muted)] mt-1">Chevron in the corner.</p>
          </Card>
          <Card clickIndicator="drawer" onClick={noop}>
            <p className="text-sm font-semibold" style={{ fontFamily: "var(--font-mono)" }}>
              clickIndicator=&quot;drawer&quot;
            </p>
            <p className="text-xs text-[var(--text-muted)] mt-1">List icon in the corner.</p>
          </Card>
        </div>
      </Specimen>

      <Specimen title="StatCard" source="components/ui/StatCard.tsx" wide>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {STAT_CARD_COLORS.map((color) => (
            <StatCard key={color} label={color} value={42} icon={ICON_PATHS.server} color={color} />
          ))}
        </div>
      </Specimen>

      <Specimen title="KpiGrid" source="components/inventory/KpiGrid.tsx" wide>
        <KpiGrid
          heading="Indicators"
          kpis={[
            { label: "Hosts", value: 128, color: "cyan", icon: ICON_PATHS.server },
            { label: "DNS records", value: 342, color: "emerald", icon: ICON_PATHS.globe },
            { label: "Services", value: 57, color: "amber", icon: ICON_PATHS.container },
            { label: "Projects", value: 19, color: "purple", icon: ICON_PATHS.folder },
          ]}
        />
      </Specimen>

      <Specimen
        title="Inventory card anatomy"
        source="components/inventory/{CardHeader,CardMetadataGrid,CardTagsSection,CardIndicator,CardIndicatorSeparator}.tsx"
        wide
      >
        <Card accent="cyan" className="max-w-sm flex flex-col">
          <CardHeader
            title="web-01"
            subtitle="web-01.sead.pi.gov.br"
            description="Nginx edge proxy"
            badge={
              <Badge variant="situacao" situacao="active" compact>
                Active
              </Badge>
            }
          />
          <CardMetadataGrid
            items={[
              { label: "IP", value: "10.0.0.4", mono: true },
              { label: "OS", value: "Ubuntu 24.04" },
              { label: "Setor", value: "Infra" },
              { label: "Uptime", value: "41d", mono: true },
            ]}
          />
          <CardTagsSection tags={["prod", "edge", "nginx", "tls", "legacy"]} />
          <div className="flex items-center gap-3 mt-auto pt-4 border-t border-[var(--border-subtle)]">
            <CardIndicator icon={ICON_PATHS.container} count={3} color="cyan" title="Containers" />
            <CardIndicatorSeparator />
            <CardIndicator icon={ICON_PATHS.alert} count={0} color="amber" title="Alerts" />
          </div>
        </Card>
        <p className="text-xs text-[var(--text-muted)] mt-2 w-full">
          5 vertical sections: header (title/subtitle/desc + badge), metadata grid, tags, domain-specific
          (entity-unique, omitted here), indicators.
        </p>
      </Specimen>

      <Specimen title="OfferingCard" source="app/catalog/_components/OfferingCard.tsx" wide>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <OfferingCard offering={OFFERING_1} onRequest={noop} index={0} />
          <OfferingCard offering={OFFERING_2} onRequest={noop} index={1} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>Card as=&quot;button&quot;</code> + <code>CardIcon</code>; keeps its <code>stagger-in</code> reveal.
        </p>
      </Specimen>

      <Specimen title="TableCard" source="components/atlas/catalog/TableCard.tsx" wide>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 max-w-xl">
          <TableCard table={TABLE_RECORD} onClick={noop} />
          <TableCard table={TABLE_RECORD} selected onClick={noop} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>Card as=&quot;button&quot; decorator=&quot;stripe-top&quot; selected</code>; accent from <code>getLayerAccent()</code>.
        </p>
      </Specimen>

      <Specimen title="LinkedEntityList" source="components/ui/LinkedEntityList.tsx" wide>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <LinkedEntityList
            title="Linked services"
            items={[
              { id: 1, href: "#", label: "nginx-edge", badge: <Badge color="emerald">up</Badge> },
              { id: 2, href: "#", label: "postgres-main", badge: <Badge color="emerald">up</Badge> },
            ]}
          />
          <LinkedEntityList title="Linked services" items={[]} emptyMessage="No linked services" />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>Card hover={"{false}"} padding=&quot;sm&quot;</code> around the list.
        </p>
      </Specimen>

      <div className="space-y-1">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Worklist</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            Entity cards {"app/{hosts,dns,services,projects}/_components/*Card.tsx"}, the ssh-keys credential card and the
            settings user card all pass <code>accent</code> now (<code>situacaoAccent()</code> for situacao) — the four inline{" "}
            <code>borderLeftColor</code> sources are gone. Not rendered here (need full entity objects).
          </li>
          <li>
            Still their own box: <code>components/glpi/TicketList.tsx</code> rows, <code>app/wiki/page.tsx:328</code> rows,{" "}
            <code>app/catalog/_components/AiPrefillPanel.tsx</code>, <code>app/issues/IssueBoard.tsx</code> kanban columns.
          </li>
          <li>
            <strong>66 files</strong> carry ad-hoc <code>rounded-* border bg-[var(--bg-surface)]</code> containers
            (<code>grep -rl &quot;bg-[var(--bg-surface)]&quot; src --include=*.tsx | xargs grep -l &quot;rounded-&quot;
            | xargs grep -l &quot;border-[var(--border-&quot;</code>; heaviest: <code>app/issues/IssueBoard.tsx</code>,{" "}
            <code>app/chamados/page.tsx</code>, <code>app/wiki/page.tsx</code>, <code>app/share/[token]/page.tsx</code>,{" "}
            <code>app/settings/IntegrationsTab.tsx</code>, <code>components/glpi/*</code>, <code>components/lineage/*</code>).
          </li>
        </ul>
      </div>
    </Section>
  );
}
