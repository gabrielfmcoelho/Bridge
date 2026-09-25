"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Button from "@/components/ui/Button";
import PageHeader from "@/components/ui/PageHeader";
import Divider from "@/components/ui/Divider";
import SituacaoText from "@/components/ui/SituacaoText";
import CardIndicator from "@/components/inventory/CardIndicator";
import ListToolbar from "@/components/ui/ListToolbar";
import SearchBadge from "@/components/ui/SearchBadge";
import Badge from "@/components/ui/Badge";
import { ICON_PATHS } from "@/lib/icon-paths";

const noop = () => {};

export default function NavSection() {
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [search, setSearch] = useState("");
  const [sideTab, setSideTab] = useState("profile");

  return (
    <Section id="nav" title="Navigation & Toolbars">
      <Specimen
        title="PageHeader"
        source="components/ui/PageHeader.tsx"
        alsoIn={["login, setup and share keep their own h1 (no app shell)"]}
        wide
      >
        <div className="space-y-6">
          <PageHeader title="Contatos" addLabel="Contato" onAdd={noop} />
          <PageHeader
            title="Hosts"
            addLabel="Host"
            onAdd={noop}
            hideAddOnPhone
            controlsKey="ds-demo"
            controlsBadge={2}
            controls={
              <ListToolbar
                search={search}
                onSearchChange={setSearch}
                onFilterClick={noop}
                activeFilterCount={2}
                viewMode={viewMode}
                onViewModeChange={setViewMode}
              />
            }
          />
          <PageHeader
            title="web-01"
            titleFont="mono"
            subtitle="Host"
            description="Edge proxy"
            status={<SituacaoText situacao="active" />}
            indicators={
              <>
                <CardIndicator icon={ICON_PATHS.alert} count={2} color="warning" title="Issues" />
                <CardIndicator icon={ICON_PATHS.lock} count={1} hideCount color="success" title="HTTPS" />
              </>
            }
            onEdit={noop}
            onDelete={noop}
            deleteConfirmMessage="Delete web-01?"
          />
          <PageHeader
            title="Configurações"
            tabs={{
              idBase: "ds-side",
              label: "Configurações",
              variant: "side",
              active: sideTab,
              onChange: setSideTab,
              items: [
                { key: "profile", label: "Profile", group: "Account" },
                { key: "users", label: "Users", group: "Admin" },
                { key: "integrations", label: "Integrations", group: "Admin" },
              ],
            }}
          >
            <p className="text-xs text-[var(--text-secondary)]">Panel for “{sideTab}”.</p>
          </PageHeader>
        </div>
        <p className="text-[11px] text-[var(--text-muted)] mt-1">
          One header for every page, fixed anatomy: title with <code>actions</code>, edit/delete (<code>onEdit</code>,{" "}
          <code>onDelete</code> — it confirms itself) and the one primary (<code>onAdd</code>) last; <code>status</code>{" "}
          (SituacaoText) then <code>indicators</code> (CardIndicator); <code>controls</code> for the list toolbar,
          hidable with <code>controlsKey</code>; <code>tabs</code> (top or side) with the panel as children. Breadcrumbs
          and Voltar live in the app header above it.
        </p>
      </Specimen>

      <Specimen title="ListToolbar" source="components/ui/ListToolbar.tsx" wide>
        <ListToolbar
          search={search}
          onSearchChange={setSearch}
          onFilterClick={noop}
          activeFilterCount={2}
          actions={<Button size="sm">+ Add</Button>}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          searchAdornment={<Badge color="cyan" compact>3</Badge>}
        />
        <p className="text-[11px] text-[var(--text-muted)]">
          No margin of its own: it sits in <code>PageHeader controls</code>. The view switch is a{" "}
          <code>ToolbarSelect</code> at its end. 11 other files still inline the magnifier SVG with their own input markup.
        </p>
      </Specimen>

      <Specimen title="Divider" source="components/ui/Divider.tsx" wide>
        <p className="text-xs text-[var(--text-secondary)]">Above the rule</p>
        <Divider className="my-3" />
        <div className="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
          <span>Chip</span>
          <Divider vertical className="h-4" />
          <span>Chip</span>
          <Divider vertical className="h-4" />
          <span>Chip</span>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Free-standing rules and rails only (Sidebar sections, catalog chip row, atlas list header, share page,{" "}
          <code>CardIndicatorSeparator</code>). Padded section containers keep their own <code>border-t</code>.
        </p>
      </Specimen>

      <Specimen title="SearchBadge" source="components/ui/SearchBadge.tsx">
        <SearchBadge search="nginx" onClear={noop} />
      </Specimen>

      <p className="text-xs text-[var(--text-muted)]">
        <strong>SectionHeading</strong> / <strong>ToolbarActionButton</strong>; see Typography / Buttons.
      </p>

      <p className="text-xs text-[var(--text-muted)]">
        <code>components/layout/Breadcrumbs.tsx</code> takes no props and is live in the Header above;{" "}
        <code>lib/breadcrumbs.ts buildCrumbs(pathname, NAV_ITEMS)</code> is pure and unit-tested; dynamic segments
        resolve names from the react-query cache via <code>useSyncExternalStore</code>.
      </p>

      <div>
        <p className="text-xs text-[var(--text-muted)] mb-1">Describe only:</p>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            <code>components/atlas/shared/AtlasToolbar.tsx</code>; pill filters + counts + ⌘K (needs{" "}
            <code>AtlasIndexes</code>)
          </li>
          <li><code>components/lineage/LineageToolbar.tsx</code></li>
          <li><code>components/layout/MobileBottomNav.tsx</code></li>
          <li><code>components/inventory/InventoryFAB.tsx</code></li>
          <li><code>hooks/useInventoryFilters.ts</code></li>
        </ul>
        <p className="text-[11px] text-[var(--text-faint)] mt-2">
          Worklist: page headers ×3 + 12 inline h1; toolbars ×3 with zero shared code; search inputs ×11; 300ms
          debounce duplicated (<code>app/catalog/_components/CatalogSearch.tsx:59</code>,{" "}
          <code>app/requests/_components/RequestList.tsx:58</code>).
        </p>
      </div>
    </Section>
  );
}
