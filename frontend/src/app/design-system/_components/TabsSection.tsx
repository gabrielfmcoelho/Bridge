"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import TabBar from "@/components/ui/TabBar";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import ViewModeToggle from "@/components/atlas/shared/ViewModeToggle";
import PillFilter from "@/components/atlas/shared/PillFilter";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

const SETTINGS_TABS = [
  { key: "enums", label: "Enums" },
  { key: "users", label: "Users" },
  { key: "entidades", label: "Entidades" },
  { key: "offerings", label: "Offerings" },
];

const SHARE_TABS = [
  { key: "secrets", label: "Secrets", count: 4 },
  { key: "api", label: "API Docs", count: 2 },
  { key: "wiki", label: "Wiki", count: 7 },
];

// specimen: app/catalog/_components/CatalogSearch.tsx:151
const chipClass = (active: boolean) =>
  `rounded-full border px-3.5 py-1.5 text-xs transition-[color,background-color,border-color,transform] duration-200 active:scale-[0.97] ${
    active
      ? "border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent)]"
      : "border-[var(--border-default)] text-[var(--text-secondary)] hover:border-[var(--border-strong)] hover:text-[var(--text-primary)]"
  }`;

// specimen: components/vault/VaultPage.tsx:191
function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className={`px-3 py-1 text-xs rounded-[var(--radius-md)] border transition-colors ${
        active
          ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/50"
          : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-subtle)] hover:text-[var(--text-secondary)]"
      }`}
    >
      {children}
    </button>
  );
}

export default function TabsSection() {
  const [tab, setTab] = useState("overview");
  const [settingsTab, setSettingsTab] = useState("enums");
  const [shareTab, setShareTab] = useState("secrets");
  const [sourceTab, setSourceTab] = useState<"upload" | "url">("upload");
  const [view, setView] = useState("cards");
  const [viewMode, setViewMode] = useState<"tree" | "list" | "graph">("tree");
  const [layers, setLayers] = useState<string[]>([]);
  const [layers2, setLayers2] = useState<string[]>(["silver"]);
  const [chip, setChip] = useState("offering");
  const [vaultChip, setVaultChip] = useState("shared");

  return (
    <Section id="tabs" title="Tabs & Toggles">
      <Specimen title="TabBar" source="components/ui/TabBar.tsx" wide>
        <TabBar
          tabs={[
            { key: "overview", label: "Overview", icon: ICON_PATHS.server },
            { key: "issues", label: "Issues", icon: ICON_PATHS.alert, badge: 3 },
            { key: "scans", label: "Scans", icon: ICON_PATHS.scan },
          ]}
          activeTab={tab}
          onChange={setTab}
        />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          The shared one — 8 consumers. Labels are <code>hidden sm:inline</code>, so below <code>sm</code> only
          the icon and badge show. <code>icon</code> is a raw SVG <code>d</code> string, not a component.
        </p>
      </Specimen>

      <Specimen
        title="Settings desktop tab bar"
        source="app/settings/page.tsx:85-100"
        alsoIn={["app/settings/page.tsx:73-119 (mobile tab selector → Drawer list)"]}
        wide
      >
        {/* specimen: app/settings/page.tsx:85-100 */}
        <div className="hidden md:flex gap-1 mb-6 p-1 rounded-[var(--radius-md)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] overflow-x-auto">
          {SETTINGS_TABS.map((t) => (
            <button
              key={t.key}
              onClick={() => setSettingsTab(t.key)}
              className={`px-4 py-2 text-sm font-medium rounded-[var(--radius-sm)] transition-all duration-150 whitespace-nowrap ${
                settingsTab === t.key
                  ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                  : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)]">
          Pixel-identical to <code>TabBar</code> minus icons and badges — and <code>hidden md:flex</code>, so it
          is invisible on a phone where a separate button + <code>Drawer</code> list takes over.
        </p>
      </Specimen>

      <Specimen title="Share bundle tabs" source="app/share/[token]/page.tsx:517-531" wide>
        {/* specimen: app/share/[token]/page.tsx:517-531 */}
        <div className="flex gap-1 border-b border-[var(--border-subtle)]">
          {SHARE_TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => setShareTab(t.key)}
              className={`px-4 py-2 text-sm font-medium -mb-px border-b-2 transition-colors ${
                shareTab === t.key
                  ? "border-[var(--accent)] text-[var(--accent)]"
                  : "border-transparent text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              }`}
            >
              {t.label}
              <span className="ml-1.5 text-[10px] opacity-60">{t.count}</span>
            </button>
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Underline style with a count suffix — the only tab bar in the app that is not the pill/segment look,
          and the only one rendered on the public unauthenticated page.
        </p>
      </Specimen>

      <Specimen title="AddApiModal source tabs" source="components/atlas/apis/AddApiModal.tsx:117-135">
        {/* specimen: components/atlas/apis/AddApiModal.tsx:117-135 */}
        <div className="flex gap-2">
          {(["upload", "url"] as const).map((tk) => (
            <button
              key={tk}
              type="button"
              onClick={() => setSourceTab(tk)}
              className={`px-3 py-1.5 text-sm rounded-[var(--radius-md)] border ${
                sourceTab === tk
                  ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
                  : "border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {tk === "upload" ? "Upload" : "From URL"}
            </button>
          ))}
        </div>
      </Specimen>

      <Specimen title="ViewToggle" source="components/ui/ViewToggle.tsx">
        <ViewToggle
          value={view}
          onChange={setView}
          options={[
            { key: "cards", label: "Cards", icon: VIEW_ICONS.cards },
            { key: "table", label: "Table", icon: VIEW_ICONS.table },
            { key: "kanban", label: "Kanban", icon: VIEW_ICONS.kanban },
          ]}
        />
        <span className="text-xs text-[var(--text-muted)] self-center">
          icon-only; labels live in <code>title</code>. <code>VIEW_ICONS</code> is its own 3-key path map,
          separate from <code>lib/icon-paths.ts</code>.
        </span>
      </Specimen>

      <Specimen
        title="ViewModeToggle"
        source="components/atlas/shared/ViewModeToggle.tsx"
        alsoIn={[
          "app/settings/page.tsx:455-476 (inline cards/table toggle)",
          "app/issues/IssueBoard.tsx:348-360 (inline kanban/list toggle)",
        ]}
        wide
      >
        <ViewModeToggle
          value={viewMode}
          onChange={setViewMode}
          ariaLabel="View mode"
          options={[
            { value: "tree", label: "Tree", icon: <Icon path={ICON_PATHS.folder} className="w-3.5 h-3.5" /> },
            { value: "list", label: "List", icon: <Icon path={ICON_PATHS.document} className="w-3.5 h-3.5" /> },
            { value: "graph", label: "Graph", icon: <Icon path={ICON_PATHS.link} className="w-3.5 h-3.5" /> },
          ]}
        />
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Generic over the value type and takes <code>ReactNode</code> icons instead of path strings. Its own
          docblock says it &quot;mirrors the styling vocabulary of the existing ViewToggle&quot; — which is four
          view-toggle implementations, not one.
        </p>
      </Specimen>

      <Specimen title="PillFilter" source="components/atlas/shared/PillFilter.tsx" wide>
        <div className="space-y-3">
          <PillFilter
            label="Layer"
            value={layers}
            onChange={setLayers}
            renderLead={() => <span className="w-1.5 h-1.5 rounded-full bg-amber-400" />}
            options={[
              { value: "gold", label: "Gold", count: 12 },
              { value: "silver", label: "Silver", count: 34 },
              { value: "bronze", label: "Bronze", count: 7 },
            ]}
          />
          <PillFilter
            label="Layer"
            hideLabel
            value={layers2}
            onChange={setLayers2}
            options={[
              { value: "gold", label: "Gold", count: 12 },
              { value: "silver", label: "Silver", count: 34 },
              { value: "bronze", label: "Bronze", count: 7 },
            ]}
          />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          Multi-select: an empty array means no filter, so the <code>All</code> pill lights up.{" "}
          <code>renderLead</code> is how the lineage layer dots get in.
        </p>
      </Specimen>

      <Specimen title="chipClass()" source="app/catalog/_components/CatalogSearch.tsx:151">
        {(["all", "offering", "asset"] as const).map((k) => (
          <button key={k} type="button" onClick={() => setChip(k)} className={chipClass(chip === k)}>
            {k === "all" ? "All" : k === "offering" ? "Offerings" : "Existing"}
          </button>
        ))}
      </Specimen>

      <Specimen title="VaultPage Chip" source="components/vault/VaultPage.tsx:191">
        {(["personal", "shared"] as const).map((k) => (
          <Chip key={k} active={vaultChip === k} onClick={() => setVaultChip(k)}>
            {k === "personal" ? "Personal" : "Shared"}
          </Chip>
        ))}
      </Specimen>

      <div className="space-y-2">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Cross-references &amp; worklist</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            <code>components/ui/PillButton.tsx</code> — the fourth filter-chip style; rendered under Buttons.
          </li>
          <li>
            <code>components/ui/Toggle.tsx</code> — the on/off switch; rendered under Inputs &amp; Forms.
          </li>
          <li>
            Tab implementations &times;5: <code>TabBar</code>, <code>app/settings/page.tsx</code>,{" "}
            <code>app/share/[token]/page.tsx</code>, <code>components/atlas/apis/AddApiModal.tsx</code>,{" "}
            <code>components/atlas/shared/ViewModeToggle.tsx</code>.
          </li>
          <li>
            View toggles &times;4: <code>ViewToggle</code>, <code>ViewModeToggle</code>,{" "}
            <code>app/settings/page.tsx:455-476</code>, <code>app/issues/IssueBoard.tsx:348-360</code>.
          </li>
          <li>
            Filter chips &times;4: <code>PillButton</code>, <code>PillFilter</code>, <code>chipClass()</code>,{" "}
            <code>VaultPage.Chip</code>.
          </li>
          <li>
            <code>app/hosts/[slug]/_components/SSHConfigDrawer.tsx:40-62</code> also rolls its own two-button
            segmented control.
          </li>
        </ul>
      </div>
    </Section>
  );
}
