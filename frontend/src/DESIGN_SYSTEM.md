# SSHCM Design System

Reference guide for maintaining visual and code consistency across all inventory pages. **Host pages are the canonical reference** — DNS, Services, and Projects follow the same patterns.

---

## 1. Card Anatomy

Every inventory card has **5 vertical sections**, top to bottom:

```
+------------------------------------------+
| HEADER: title + subtitle + desc | BADGE  |  <- CardHeader
+------------------------------------------+
| METADATA: 2x2 label+value grid          |  <- CardMetadataGrid
+------------------------------------------+
| TAGS: up to 4 badges, +N overflow       |  <- CardTagsSection
+------------------------------------------+
| DOMAIN-SPECIFIC: resources, spacer, etc  |  <- unique per entity
+------------------------------------------+
| INDICATORS: icons + counts              |  <- CardIndicator row
+------------------------------------------+
```

### Header (3 lines)
- **Line 1** — Title: `font-mono, text-sm, font-semibold, text-primary` (nickname, domain, name)
- **Line 2** — Subtitle: `font-mono, text-xs, text-faint` (slug, service type, setor)
- **Line 3** — Description: `font-body, text-xs, text-muted, truncate`
- **Badge** — Right-aligned, `compact` situacao badge or categorical badge

### Metadata Grid
- 2x2 grid: `grid-cols-2 gap-x-4 gap-y-3`
- Labels: `text-xs text-faint`
- Values: `text-xs text-secondary truncate` (add `font-mono` for IDs, hostnames, tech values)
- **No redundancy** — if data appears in header or indicators, don't repeat it here

### Tags Section
- Border-top separator: `mt-3 pt-3 border-t border-subtle`
- Max 4 visible, `+N` overflow pill
- Empty: show `-`
- Min height: `min-h-[28px]` (prevents layout shift)

### Bottom Indicators
- `mt-auto pt-4 border-t border-subtle mt-4`
- Icons **always visible** (active color when count > 0, `text-faint` when 0)
- Counts shown **only when > 0**, mono font, active color
- Vertical separators between semantic groups

---

## 2. Color System

### Entity Colors (for cross-entity indicators)

| Entity       | Color   | Tailwind     |
|-------------|---------|--------------|
| Hosts       | cyan    | cyan-400     |
| DNS         | emerald | emerald-400  |
| Services    | amber   | amber-400    |
| Projects    | violet  | violet-400   |
| Containers  | sky     | sky-400      |
| Processes   | violet  | violet-400   |
| Dependencies| amber   | amber-400    |
| Alerts      | amber   | amber-400    |
| Issues      | purple  | purple-400   |
| Chamados    | orange  | orange-400   |

Defined in `lib/constants.ts` as `ENTITY_INDICATOR_COLORS`.

### Situacao Colors
Dynamic from backend enums. Fallbacks:
- `active` = `#10b981` (emerald)
- `maintenance` = `#f59e0b` (amber)
- everything else = `#6b7280` (gray)

### Card Border-Left Color
- **Hosts/DNS/Projects**: situacao-based (from enum or fallback)
- **Services**: dependency-based (red=external dep, cyan=internal, amber=external)

### Semantic Colors (status)
- Success/Active: emerald (`#10b981`)
- Warning/Caution: amber (`#f59e0b`)
- Danger/Critical: red (`#ef4444`)
- Info: sky (`#0ea5e9`)
- External: amber or red (for dependencies)

---

## 3. Typography

| Context              | Font          | Size   | Weight     |
|---------------------|---------------|--------|------------|
| Page titles         | `--font-display` (JetBrains Mono) | 2xl | bold |
| Card titles         | `--font-mono`  | sm     | semibold   |
| Card subtitles      | `--font-mono`  | xs     | normal     |
| Card descriptions   | `--font-body`  | xs     | normal     |
| Labels              | `--font-body`  | xs     | normal     |
| Values (IDs, tech)  | `--font-mono`  | xs     | normal     |
| Values (text)       | `--font-body`  | xs     | normal     |
| Indicator counts    | `--font-mono`  | xs     | semibold   |
| Section headings    | `--font-body`  | xs     | semibold, uppercase, tracking-wider |

### When to use mono font
- Nicknames, slugs, hostnames, domains
- Technology stacks, versions, ports
- IDs, timestamps, IP addresses
- Indicator counts
- Code/commands

---

## 4. Spacing Tokens

| Element                  | Spacing                     |
|-------------------------|-----------------------------|
| Page header margin      | `mb-6`                      |
| KPI section             | `mb-5` wrapper              |
| KPI grid                | `gap-3`                     |
| Listing label           | `mb-3`                      |
| Toolbar                 | `mb-5` (built into ListToolbar) |
| Card grid               | `gap-4`                     |
| Card header to metadata | `mb-3`                      |
| Metadata grid           | `gap-x-4 gap-y-3`          |
| Tags separator          | `mt-3 pt-3 border-t`       |
| Indicators separator    | `mt-auto pt-4 border-t mt-4` |
| Indicator items         | `gap-3`                     |

---

## 5. Icon Rules

- Size: `w-3.5 h-3.5` for card indicators, `w-4 h-4` for toolbar buttons
- Style: stroke-based, `stroke="currentColor"`, `strokeWidth={2}`
- ViewBox: `0 0 24 24`
- Color: contextual (`text-{color}-400` when active, `text-[var(--text-faint)]` when inactive)
- **Icons always visible** even when the associated count is 0 (they appear in faint color)
- SVG paths centralized in `lib/icon-paths.ts`

---

## 6. Badge Rules

| Usage                | Variant                | Props                    |
|---------------------|------------------------|--------------------------|
| Entity status       | `variant="situacao"`   | `situacao={value} compact` |
| External dependency | `color="red"`          | `compact`                |
| Internal/External   | `color="cyan"/"amber"` | `compact`                |
| Tags                | default (no props)     |                          |

Compact badges: dot only, expand with label on hover.

---

## 7. Card Navigation

- **All cards use `<Link>`** wrapping (semantic HTML, SSR-friendly, right-click works)
- Cards have `clickIndicator="link"` on the `<Card>` component
- **Never use `onClick` + `useRouter.push()`** for card navigation

---

## 8. List Page Pattern

Every inventory list page follows this canonical structure:

```tsx
<PageShell>
  {/* 1. Header: title + ViewToggle + Add button */}
  <div className="flex items-center justify-between gap-2 mb-6">
    <h1 style={{ fontFamily: "var(--font-display)" }}>{title}</h1>
    <div className="flex items-center gap-1.5">
      <ViewToggle />        {/* hidden sm:flex */}
      <Button>+ Add</Button> {/* hidden sm:block, canEdit */}
    </div>
  </div>

  {/* 2. KPI Section (if data) */}
  <KpiSection />

  {/* 3. Active Search Display (if searching) */}
  <SearchDisplay />

  {/* 4. Listing Label (if data) */}
  <h2 className="text-xs font-semibold text-faint uppercase tracking-wider mb-3">
    {listingLabel}
  </h2>

  {/* 5. ListToolbar: search + filter + export + domain actions */}
  <ListToolbar />

  {/* 6. Content: cards grid | table | skeleton | empty state */}
  <Content />

  {/* 7. Overlays: Form drawer/modal + FilterDrawer */}
  {/* 8. Mobile FAB */}
</PageShell>
```

### KPI Section
- Wrapped in `<div className="mb-5">`
- Label: `<h2>` with `common.indicators` translation key
- Grid: `grid-cols-2 sm:grid-cols-4 gap-3` (or `sm:grid-cols-3 lg:grid-cols-5` for 5 KPIs)
- Uses `<StatCard>` components

### Content Grid
- Cards: `grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4`
- Stagger animation: `animate-slide-up stagger-{min(i+1, 9)}`
- Skeleton: 6 `<SkeletonCard />` during loading

---

## 9. Redundancy Rules

Each piece of data should appear in **exactly one** card section:

| Data Type              | Appears In      | NOT In            |
|-----------------------|-----------------|-------------------|
| Primary name/ID       | Header (title)  | Grid, indicators  |
| Secondary identifier  | Header (subtitle)| Grid              |
| Status/situacao       | Header (badge)  | Grid, indicators  |
| Entity link counts    | Indicators      | Grid              |
| Boolean flags         | Indicators      | Grid              |
| Descriptive text      | Header (desc)   | Grid              |
| Categorical metadata  | Grid            | Header, indicators|
| Tags                  | Tags section    | Nowhere else      |

---

## 10. Component Reference

### Shared (from `components/inventory/`)
- `CardHeader` — 3-line header with badge
- `CardMetadataGrid` — 2x2 label+value grid
- `CardTagsSection` — Tags row with overflow
- `CardIndicator` — Icon + count atomic unit
- `CardIndicatorSeparator` — Vertical divider

### UI Primitives (from `components/ui/`)
- `Card` — Surface container with accent border
- `Badge` — Status/categorical badges
- `StatCard` — KPI stat display
- `ListToolbar` — Search + filter + actions toolbar
- `ViewToggle` — Cards/table view switcher
- `EmptyState` — No-data placeholder
- `Skeleton` / `SkeletonCard` — Loading states
- `TabBar` — Detail page tab navigation
- `DetailHeader` / `DetailActions` — Detail page header
- `Drawer` / `ResponsiveModal` — Form containers
- `FloatingActionButton` — Mobile FAB

### Constants
- `lib/icon-paths.ts` — All SVG path `d` attributes
- `lib/constants.ts` — `ENTITY_INDICATOR_COLORS`, `SITUACAO_COLORS`, navigation

---

## 11. Adding a New Inventory Page

1. Create `app/{entity}/page.tsx` following the List Page Pattern (section 8)
2. Create `app/{entity}/_components/EntityCard.tsx` using shared card components
3. Create `app/{entity}/_components/KpiSection.tsx` with `common.indicators` label
4. Create `app/{entity}/_components/EntityTableView.tsx` with `SortableTable` + `Pagination`
5. Create `app/{entity}/_components/EntityFAB.tsx` for mobile
6. Create `app/{entity}/FilterDrawer.tsx` with collapsible sections
7. Create `app/{entity}/EntityForm.tsx` (multi-step in Drawer, see section 14)
8. Create `app/{entity}/[id]/` detail page with DetailHeader + TabBar + tabs (see section 12)
9. Add SVG paths to `lib/icon-paths.ts` if new icons needed
10. Add translations to both locale files

---

## 12. Detail Page Pattern

Every inventory detail page follows this canonical structure:

```tsx
<PageShell>
  {/* 1. DetailHeader: back link + title (accent color) + subtitle + badges + counters */}
  <DetailHeader
    backHref="/entity"
    backLabel={t("common.back")}
    title={entity.name}
    titleFont="mono"          // mono for slugs/domains, display for names
    titleColor="var(--accent)" // accent color for consistency
    subtitle="Entity Type"
    badges={<Badge variant="situacao" ... />}
    counters={/* inline issue/alert counts with colored icons */}
  >
    <DetailActions canEdit onEdit onDelete />
  </DetailHeader>

  {/* 2. TabBar with icons + badge counts */}
  <TabBar
    tabs={[
      { key: "overview", label: "Overview", icon: "M3 12l2-2m0..." },
      { key: "issues", label: "Acontecimentos", icon: "M12 9v2...", badge: issueCount },
      { key: "topology", label: "Topology", icon: "M13 10V3..." },
    ]}
    activeTab={activeTab}
    onChange={setActiveTab}
  />

  {/* 3. Tab content */}
  {activeTab === "overview" && <OverviewTab />}
  {activeTab === "issues" && <IssuesTab />}
  {activeTab === "topology" && <TopologyTab />}

  {/* 4. Edit Drawer with Form component */}
  <Drawer open={showEditDrawer} title="Edit" subHeader={formSubHeader}>
    <EntityForm initial={entity} onSuccess={...} onSubHeaderChange={...} />
  </Drawer>

  {/* 5. Mobile FAB */}
  <FloatingActionButton actions={[{ label: "Edit", icon: "...", onClick }]} />
</PageShell>
```

### Tab Icon Assignments
| Tab | Icon (SVG d path) |
|-----|-------------------|
| Overview | `M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6` |
| Acontecimentos | `M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z` |
| Topology | `M13 10V3L4 14h7v7l9-11h-7z` |
| Connections | `M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101...` |
| Credentials | `M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z` |

### Inline Counters
Show counts next to badges in the header for cross-cutting entities:
```tsx
counters={
  issueCount > 0 ? (
    <span className="inline-flex items-center gap-1 text-xs font-medium text-purple-400">
      <svg ...>{/* warning icon */}</svg> {issueCount}
    </span>
  ) : undefined
}
```

---

## 13. Responsaveis Pattern

Every entity supports **N responsaveis** (NOT a single `empresa` or `responsavel` text field). The pattern is uniform across hosts, DNS, services, and projects.

### Data Model
```ts
interface EntityResponsavel {
  id?: number;
  contact_id?: number;
  is_main: boolean;      // ONE per entity is main
  is_externo: boolean;    // External person flag
  name: string;          // Contact autocomplete
  phone: string;         // Raw digits, formatted on display
  role: string;          // Job title / role
  entity: string;        // Organization (enum: entidade_responsavel)
}
```

### Form Input
Use `<ResponsavelList>` from `components/inventory/ResponsavelList.tsx`:
- Contact autocomplete (searches existing contacts by name)
- Phone formatting: `(XX) XX 9 XXXX-XXXX`
- `is_main` toggle (pill button, one per entity)
- `is_externo` checkbox
- Entity dropdown (from enums)
- Add/remove responsaveis

### Display (Overview Tab)
Use `<ResponsaveisSection>` from `components/inventory/ResponsaveisSection.tsx`:
- **Cards view:** Avatar initials (cyan=main, gray=secondary), star icon for main, name+role, WhatsApp button, external badge, phone+entity grid
- **Table view:** Sortable columns (name, phone, role, entity, type), WhatsApp action
- ViewToggle between cards and table

### Backend Pattern
- Junction table per entity: `{entity}_responsaveis` (links to `contacts`)
- `Sync{Entity}Responsaveis()` — transactional replace pattern
- `Get{Entity}MainResponsavelNamesBulk()` — single query for list enrichment
- `List{Entity}Responsaveis()` — joined with contacts for detail pages

---

## 14. Form Pattern

All entity forms follow the same architecture:

### Create Mode: Multi-step wizard
```tsx
<StepIndicator steps={["Basic Info", "Responsaveis", "Links & Tags"]} current={step} />
```
- Step 1: Basic identity fields
- Step 2: Responsaveis (using `<ResponsavelList>`)
- Step 3: Links & Tags (linked entities, tag input)

### Edit Mode
Same form component with `initial` prop. Steps still available for navigation.

### Container
- **Always in Drawer** (NOT `ResponsiveModal` or inline)
- Footer managed via `onFooterChange` callback for Drawer integration
- SubHeader via `onSubHeaderChange` for StepIndicator

### Key Props
```ts
interface EntityFormProps {
  initial?: Entity | null;
  initialTags?: string[];
  initialResponsaveis?: EntityResponsavel[];
  onSuccess: () => void;
  onFooterChange?: (footer: React.ReactNode) => void;
  onSubHeaderChange?: (subHeader: React.ReactNode) => void;
}
```

---

## 15. Acontecimentos Tab Pattern

Detail pages should include an "Acontecimentos" (Issues/Tracking) tab:

### Minimal Version (DNS)
- Issues list (open/closed sections)
- Create issue drawer
- Uses `globalIssuesAPI.list({ entity_type, entity_id })`

### Full Version (Hosts)
- Three sections: Alerts, Issues (kanban + table), Chamados
- Each section has its own create/edit drawer
- Alert auto-detection from scans

### Tab Configuration
- Label: `t("host.acontecimentos") || "Acontecimentos"`
- Icon: warning triangle path
- Badge: total issue count (when > 0)

---

## 16. Table View Pattern

All inventory table views follow this structure:

### Components
- `SortableTable` — Header with sort toggles, generic column types
- `Pagination` — Page controls (previous/next, current/total)

### Rules
- **No inline edit/delete buttons** — actions belong in the detail page only
- Row click navigates to detail page
- 20 rows per page (configurable)
- Mono font for IDs, hostnames, domains, tech values
- Tags column shows max 3 with `+N` overflow
- Consistent columns: entity-specific fields + status + tags

### Usage
```tsx
<SortableTable columns={[...]} defaultSort="name">
  {(sortKey, sortDir) => {
    const sorted = sortRows(items, sortKey, sortDir, sortFns);
    const paged = sorted.slice((page - 1) * PER_PAGE, page * PER_PAGE);
    return paged.map((item) => <tr key={item.id}>...</tr>);
  }}
</SortableTable>
<Pagination page={page} totalPages={totalPages} total={total} perPage={PER_PAGE} onChange={setPage} />
```
