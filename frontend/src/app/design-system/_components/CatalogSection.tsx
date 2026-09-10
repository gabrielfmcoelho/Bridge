"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import StepIndicator from "@/components/ui/StepIndicator";
import DynamicField from "@/components/requests/DynamicField";
import RequestAnswers from "@/components/requests/RequestAnswers";
import TemplatePicker from "@/app/catalog/_components/TemplatePicker";
import CatalogAssetsTable, { type AssetSortKey } from "@/app/catalog/_components/CatalogAssetsTable";
import AiPrefillPanel from "@/app/catalog/_components/AiPrefillPanel";
import { REQUEST_TYPE_ICON } from "@/lib/icon-paths";
import { TRANSITIONS, TRANSITION_ACTION_KEY, transitionVariant } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { FormField, FormSchema, OfferingTemplate, CatalogHit } from "@/lib/types";

const noop = () => {};

// One field of each type the schema allows (asset_ref left out: it fetches the asset list).
const FIELDS: FormField[] = [
  { key: "hostname", type: "text", label_en: "Hostname", label_pt: "Hostname", required: true, pattern: "^[a-z0-9-]+$", pattern_hint_en: "lowercase, digits, dashes", pattern_hint_pt: "minúsculas, dígitos, hífens", placeholder_en: "web-01", placeholder_pt: "web-01" },
  { key: "vcpu", type: "number", label_en: "vCPU", label_pt: "vCPU", required: true, min: 1, max: 32, help_en: "1 to 32", help_pt: "1 a 32" },
  { key: "os", type: "select", label_en: "Operating system", label_pt: "Sistema operacional", required: true, options: ["Ubuntu 24.04", "Debian 12", "Rocky 9"] },
  { key: "backup", type: "checkbox", label_en: "Nightly backup", label_pt: "Backup noturno", required: false },
  { key: "needed_by", type: "date", label_en: "Needed by", label_pt: "Necessário até", required: false },
  { key: "tags", type: "tags", label_en: "Tags", label_pt: "Tags", required: false },
  { key: "notes", type: "textarea", label_en: "Notes", label_pt: "Observações", required: false, max_length: 500 },
];
const SCHEMA: FormSchema = { fields: FIELDS };

const TEMPLATES: OfferingTemplate[] = [
  { key: "small", name_en: "Small", name_pt: "Pequena", summary_en: "Dev and staging boxes", summary_pt: "Máquinas de dev e homologação", values: { vcpu: 2, os: "Ubuntu 24.04", backup: false } },
  { key: "standard", name_en: "Standard", name_pt: "Padrão", summary_en: "Most internal apps", summary_pt: "A maioria dos sistemas internos", values: { vcpu: 4, os: "Ubuntu 24.04", backup: true } },
];

const HITS: CatalogHit[] = [
  { kind: "asset", asset_type: "host", id: 1, name: "vm-postgres-02", description: "PostgreSQL de homologação", detail: "vm-postgres-02.sead.pi.gov.br", href: "#catalog", entidade_name: "SGA" },
  { kind: "asset", asset_type: "service", id: 2, name: "api-gateway", description: "Edge proxy", detail: "nginx · :8080", href: "#catalog", entidade_name: "SGA" },
  { kind: "asset", asset_type: "dns", id: 3, name: "portal.pi.gov.br", description: "Portal do Servidor", detail: "A · 10.0.0.4", href: "#catalog", entidade_name: "SEAD" },
];

const ANSWERS: Record<string, unknown> = { hostname: "web-01", vcpu: 4, os: "Ubuntu 24.04", backup: true, needed_by: "2026-10-01", tags: ["prod", "edge"], notes: "Nginx in front of the portal." };

export default function CatalogSection() {
  const { t } = useLocale();
  const [values, setValues] = useState<Record<string, unknown>>({ os: "Ubuntu 24.04" });
  const [template, setTemplate] = useState<string | null>("standard");
  const [sort, setSort] = useState<{ key: AssetSortKey; dir: "asc" | "desc" }>({ key: "name", dir: "asc" });
  const moves = TRANSITIONS.submitted;

  return (
    <Section id="catalog" title="Catálogo & Requests">
      <p className="text-xs text-[var(--text-muted)]">
        The blocks that make up /catalog, the request form and /requests/[id], rendered with static fixtures. Offerings
        admin (settings → Offerings) is described at the end.
      </p>

      <Specimen title="DynamicField" source="components/requests/DynamicField.tsx" wide>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {FIELDS.map((f) => (
            <DynamicField
              key={f.key}
              field={f}
              value={values[f.key]}
              onChange={(v) => setValues((s) => ({ ...s, [f.key]: v }))}
              error={f.key === "vcpu" && Number(values.vcpu) > 32 ? "Max 32" : undefined}
            />
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          One descriptor per field type (<code>text</code> with pattern hint, <code>number</code> with bounds,{" "}
          <code>select</code>, <code>checkbox</code>, <code>date</code>, <code>tags</code>, <code>textarea</code>);{" "}
          <code>asset_ref</code> is left out because it fetches the asset list. Renders through <code>FormField</code>.
        </p>
      </Specimen>

      <Specimen title="TemplatePicker" source="app/catalog/_components/TemplatePicker.tsx" wide>
        <TemplatePicker templates={TEMPLATES} fields={FIELDS} selected={template} onSelect={setTemplate} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Presets over an offering&apos;s form; <code>null</code> is the custom path. Selected: <code>{String(template)}</code>.
        </p>
      </Specimen>

      <Specimen title="AiPrefillPanel" source="app/catalog/_components/AiPrefillPanel.tsx" wide>
        <AiPrefillPanel offeringId={1} onDraft={noop} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Queries <code>aiAPI.status()</code>; under <code>dev:mock</code> it shows its unavailable state.
        </p>
      </Specimen>

      <Specimen title="Offering summary tile (request form subHeader)" source="app/catalog/_components/RequestFormModal.tsx:222-233" wide>
        {/* specimen: app/catalog/_components/RequestFormModal.tsx:222-233 */}
        <div className="space-y-3 max-w-xl">
          <div className="flex items-center gap-2.5">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--accent)]">
              <Icon path={REQUEST_TYPE_ICON.vm} className="h-4 w-4" strokeWidth={1.5} />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium text-[var(--text-primary)]">Standard VM</span>
              <span className="block truncate text-xs text-[var(--text-muted)]">Compute</span>
            </span>
            <Badge color="gray">{t("catalog.requestType.vm")}</Badge>
          </div>
          <StepIndicator steps={["Requester", "Start", "Details", "Review"]} current={3} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Lives in the modal&apos;s pinned <code>subHeader</code> with the <code>StepIndicator</code>, so &quot;where am I&quot; survives scrolling.
        </p>
      </Specimen>

      <Specimen title="RequestAnswers" source="components/requests/RequestAnswers.tsx" wide>
        <RequestAnswers schema={SCHEMA} data={ANSWERS} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Read-only mirror of <code>DynamicField</code> (review step and request detail): checkbox as yes/no, tags as badges,
          dates through <code>formatDate</code>, empty as a dash.
        </p>
      </Specimen>

      <Specimen title="CatalogAssetsTable" source="app/catalog/_components/CatalogAssetsTable.tsx" wide>
        <CatalogAssetsTable hits={HITS} sortKey={sort.key} sortDir={sort.dir} onSortChange={(key, dir) => setSort({ key, dir })} />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          One polymorphic row shape for six asset types; <code>detail</code> carries the one distinguishing fact. Controlled
          sort (<code>SortableTable</code>), so the header drives a refetch on the real page.
        </p>
      </Specimen>

      <Specimen title="Request action bar" source="app/requests/[id]/RequestDetail.tsx:184-195">
        {/* specimen: app/requests/[id]/RequestDetail.tsx:184-195 */}
        <div className="flex flex-wrap gap-2">
          {moves.map((to) => (
            <Button key={to} size="sm" variant={transitionVariant(to)} onClick={noop}>
              {t(TRANSITION_ACTION_KEY[to])}
            </Button>
          ))}
          <Button size="sm" variant="secondary" onClick={noop}>
            {t("common.edit")}
          </Button>
        </div>
        <p className="text-xs text-[var(--text-muted)] w-full mt-2">
          Transitions available from <code>submitted</code>, each in the variant <code>transitionVariant()</code> assigns
          (danger for reject/cancel). Confirmation goes through <code>TransitionModal</code> + <code>FormFooter</code>.
        </p>
      </Specimen>

      <div className="space-y-2">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Described only</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            <code>app/settings/OfferingsTab.tsx</code>: Card + <code>SortableTable</code> of offerings with edit/delete
            actions; <code>app/settings/OfferingFormModal.tsx</code>: <code>ResponsiveModal</code> with a{" "}
            <code>form id=&quot;offering-form&quot;</code>, JSON schema/templates editors, live preview through{" "}
            <code>TemplatePicker</code> + <code>DynamicField</code>, and <code>FormFooter</code> in the slot. Both need
            react-query data.
          </li>
          <li>
            <code>app/catalog/_components/RequestFormModal.tsx</code>: the 3-4 step wizard (requester → start → details →
            review) with the documented Cancel | Back | Next footer grid; <code>app/requests/_components/RequestList.tsx</code>{" "}
            (TabBar + ListToolbar + SortableTable + Pagination) and <code>RequestFilterDrawer.tsx</code>.
          </li>
          <li>
            Catalog-shaped loading and empty states: <code>CatalogSearch.tsx</code> renders six <code>h-[280px]</code>{" "}
            skeleton cards and an <code>EmptyState action=&quot;avulsa&quot;</code>; <code>RequestDetail.tsx</code> a 2-col
            skeleton. Generic versions are under Feedback.
          </li>
        </ul>
      </div>
    </Section>
  );
}
