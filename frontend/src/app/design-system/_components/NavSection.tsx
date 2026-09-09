"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Button from "@/components/ui/Button";
import PageHeader from "@/components/ui/PageHeader";
import InventoryPageHeader from "@/components/inventory/InventoryPageHeader";
import DetailHeader from "@/components/ui/DetailHeader";
import DetailActions from "@/components/ui/DetailActions";
import ListToolbar from "@/components/ui/ListToolbar";
import SearchBadge from "@/components/ui/SearchBadge";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

const noop = () => {};

export default function NavSection() {
  const [viewMode, setViewMode] = useState<"cards" | "table">("cards");
  const [search, setSearch] = useState("");

  return (
    <Section id="nav" title="Navigation & Toolbars">
      <Specimen
        title="PageHeader"
        source="components/ui/PageHeader.tsx"
        alsoIn={["12 pages hand-roll the <h1> instead; see Typography"]}
        wide
      >
        <PageHeader title="Hosts" addLabel="Host" onAdd={noop} />
      </Specimen>

      <Specimen title="InventoryPageHeader" source="components/inventory/InventoryPageHeader.tsx" wide>
        <InventoryPageHeader title="Hosts" viewMode={viewMode} onViewModeChange={setViewMode} addLabel="Host" onAdd={noop} />
        <p className="text-[11px] text-[var(--text-muted)] mt-1">
          Same markup as <code>PageHeader</code> + a <code>ViewToggle</code> (<code>hidden sm:flex</code>); used by
          exactly the 4 inventory pages.
        </p>
      </Specimen>

      <Specimen
        title="DetailHeader"
        source="components/ui/DetailHeader.tsx"
        alsoIn={["app/hosts/[slug]/HostDetail.tsx:145 (hand-rolled detail header)"]}
        wide
      >
        <DetailHeader
          backHref="/design-system"
          backLabel="Back"
          title="web-01"
          titleFont="mono"
          titleColor="var(--accent)"
          subtitle="Host"
          description="Edge proxy"
          badges={<Badge variant="situacao" situacao="active" compact>Active</Badge>}
          counters={
            <span className="inline-flex items-center gap-1 text-xs font-medium text-purple-400">
              <Icon path={ICON_PATHS.alert} className="w-3.5 h-3.5" /> 2
            </span>
          }
        >
          <DetailActions canEdit isAdmin onEdit={noop} onDelete={noop} deleteConfirmMessage="Delete web-01?" />
        </DetailHeader>
        <p className="text-[11px] text-[var(--text-muted)]">
          <code>DetailActions</code> is <code>hidden md:flex</code>; <code>onDelete</code> goes through native{" "}
          <code>confirm()</code>.
        </p>
      </Specimen>

      <Specimen title="ListToolbar" source="components/ui/ListToolbar.tsx" wide>
        <ListToolbar
          search={search}
          onSearchChange={setSearch}
          onFilterClick={noop}
          activeFilterCount={2}
          actions={<Button size="sm">+ Add</Button>}
          searchAdornment={<Badge color="cyan" compact>3</Badge>}
        />
        <p className="text-[11px] text-[var(--text-muted)]">
          <code>mb-5</code> baked in; 5 consumers; the de-facto search box — 11 other files inline the magnifier SVG
          with their own input markup.
        </p>
      </Specimen>

      <Specimen title="SearchBadge" source="components/ui/SearchBadge.tsx">
        <SearchBadge search="nginx" onClear={noop} />
      </Specimen>

      <p className="text-xs text-[var(--text-muted)]">
        <strong>SectionHeading</strong> / <strong>ToolbarActionButton</strong> — see Typography / Buttons.
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
            <code>components/atlas/shared/AtlasToolbar.tsx</code> — pill filters + counts + ⌘K (needs{" "}
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
