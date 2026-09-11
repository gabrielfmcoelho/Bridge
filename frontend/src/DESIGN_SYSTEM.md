# Bridge Design System

Rules for the parts of the UI a page cannot show. Everything renderable lives
at **`/design-system`** in the app (admin only; `npm run dev:mock`, admin
persona): every primitive with its variants, next to the ad-hoc copies still
waiting to be migrated, with "Also implemented in" notes per block. When this
file and that page disagree, the page is the current state and this file is
the intent.

Counts quoted below are grep tallies as of 2026-09-10; `frontend/scripts/ds-counts.sh`
reprints them.

---

## 1. Rules

1. **Colour comes from tokens.** `--bg-*`, `--border-*`, `--text-*` for surfaces;
   `--success` `--warning` `--danger` `--info` for meaning; `--accent` for the
   one interactive colour (runtime-overridden by `app_color`); `--cyan`
   `--purple` `--rose` for categorical identity. Tints are opacity modifiers:
   `bg-[var(--success)]/15 border-[var(--success)]/30`. No raw palette classes
   (`text-emerald-400`) and no hex in tsx: the light theme only works through
   the tokens. Swept app-wide on 2026-09-10; what is left is `Header.tsx`, `app/secrets`
   and the atlas layer palette in `LayerBadge`.
2. **One registry for icons.** `lib/icon-paths.ts` (`ICON_PATHS`, `NAV_ICONS`)
   through `<Icon path size>`; `size` scales stroke with the box. Adding an
   icon means adding a named path there, never an inline `<svg>`.
3. **One card.** `<Card accent decorator padding selected as>`; children read
   `var(--card-accent)`. Inventory cards compose `CardHeader`, `CardMetadataGrid`,
   `CardTagsSection`, `CardIndicator` inside it. KPI tiles are `StatCard`.
4. **Three headings.** `PageHeader` (title, subtitle, actions, add) for the page;
   `SectionHeading variant="section|label|rule"` inside it. No inline `<h1>`/`<h2>`
   recipes.
5. **One field anatomy.** `FormField` = label, control, hint, error, required
   asterisk. `Input`, `Textarea`, `Select` (searchable), `NativeSelect` (plain)
   render through it; wrap anything else in it. Actions sit in the Modal/Drawer
   `footer` slot: `FormFooter` for Cancel/Confirm, `useMultiStepForm` for wizards.
6. **Buttons are `Button`, `IconButton`, `PillButton`** (chips: `shape`, `count`,
   `lead`), `ViewToggle`, `TabBar`, `ToolbarActionButton`, `CopyButton`. All press
   (`active:scale`); the global `*:focus-visible` ring is the focus indicator.
7. **Motion**: `.stagger-in` with `--i` per item is the only entrance cascade;
   `transition` (Tailwind's default list), never `transition-all`, except an
   element that animates a size, which names it (`transition-[width]`).
8. **Type scale** ends at `text-2xs` (10px) and `text-3xs` (9px); no `text-[Npx]`.
   `font-display` and `font-mono` are utilities; no inline `fontFamily` styles.
9. **Radius** only through `--radius-sm|md|lg|xl`; `rounded-full` for pills and
   dots. **Shadows** through `--shadow-sm|md|lg`.
10. **Small parts have a home**: `StatusDot`, `Avatar`, `Spinner`, `Divider`,
    `DropdownMenu`, `Field` (read-only key/value), `tableClasses` (table skin),
    `useCopy`, `useDebounce`, `formatPhone` in `lib/utils`.
11. **`--text-faint` is for non-text**: icons at rest, hairlines, disabled
    glyphs, empty-cell dashes. If it is a word a human has to read, it gets
    `--text-muted` or above. The three text tiers are contrast-tuned per theme
    (measured in-browser, not eyeballed) and none of them is decorative.
12. **Uppercase is for table column headers only** (`tableClasses.th`), where
    it is a real convention and the row below it is data. Uppercase
    letterspaced 10-12px text is the slowest text on the page to read, so
    nothing else earns it: differentiate labels by weight and colour at the
    same size. Swept 2026-09-11, 149 → 2.
13. **Mono is for machine identifiers** — hostname, domain, IP, ID, path,
    token, version, count, timestamp — where fixed advance width makes
    character-level diffs pop. Prose gets `font-display` (which is the body
    sans, not a third face). `CardHeader titleFont` and `DetailHeader
    titleFont` carry the choice per entity.
14. **Elevation is `--elevate` / `--elevate-hi`**, never `--shadow-*` directly
    on a surface. In dark they are a 1px inner top highlight (a drop shadow
    has nowhere to land on `#080c14`); in light they map to the real shadows.
15. **State is never colour alone** (WCAG 1.4.1). A status dot carries shape
    (filled vs ring) and an accessible name; prefer showing the label.
16. **Render data that exists.** A metadata pair with no value, a zero count,
    a meter with no reading and a tag row with no tags are not rendered at
    all — `CardMetadataGrid`, `CardIndicator` and `CardTagsSection` return
    `null` rather than a `-`. Card grids use `items-start` so a card keeps its
    own height and height becomes information. A boolean flag is the
    exception: pass `hideCount` and the off state stays visible.

---

## 2. Card Anatomy

Every inventory card has five vertical sections inside `<Card accent={situacaoAccent(...)}>`:

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

- Title: mono, sm, semibold. Subtitle: mono, xs, faint. Description: body, xs, muted, truncate.
- Metadata grid `grid-cols-2 gap-x-4 gap-y-3`; labels faint xs, values secondary xs (mono for IDs, hosts, tech).
- Tags: `mt-3 pt-3 border-t border-subtle`, max 4 + `+N`, `min-h-[28px]`.
- Indicators: `mt-auto pt-4 border-t`; icons always visible (faint at count 0), counts only when > 0.
- Card accent: entity cards pass `situacaoAccent(situacao, enumColor)`; services pass
  `danger | cyan | warning` for external-dependency / in-house / vendor.

### Redundancy rules

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

## 3. Entity Colours

| Entity | Token |
|---|---|
| Hosts | `--cyan` |
| DNS | `--success` |
| Services | `--warning` |
| Projects, Issues | `--purple` |
| Containers | `--info` |
| Chamados | `--warning` |

`lib/constants.ts` `ENTITY_INDICATOR_COLORS` is on tokens as of the 2026-09-10 sweep.

---

## 4. Spacing

| Element | Spacing |
|---|---|
| Page header margin | `mb-6` (inside `PageHeader`) |
| KPI section | `mb-5` (inside `KpiGrid`) |
| Listing label | `mb-3` (inside `SectionHeading`) |
| Toolbar | `mb-5` (inside `ListToolbar`) |
| Card grid | `gap-4` |
| Card padding | `p-3.5 md:p-5` (`Card padding="md"`), `p-4` (`"sm"`) |
| Modal / Drawer body and footer | `p-4 md:p-5` |
| Metadata grid | `gap-x-4 gap-y-3` |

---

## 5. Badges

| Usage | Props |
|---|---|
| Entity status | `<Badge variant="situacao" situacao compact>` |
| Meaning | `color="success | warning | danger | info"` |
| Category | `color="cyan | purple | rose"` (hue keys `emerald`, `amber`, `red`, `sky` still resolve) |
| Tags | default |

Compact badges are a dot that expands with its label on hover.

---

## 6. Card Navigation

- All cards wrap in `<Link>` (semantic HTML, SSR-friendly, right-click works) with `clickIndicator="link"`.
- Whole-card controls that are not navigation (catalog offerings, atlas tables) are `<Card as="button">`.
- Never `onClick` + `router.push()` for navigation.

---

## 7. List Page Pattern

```tsx
<PageShell>
  <InventoryPageHeader title viewMode onViewModeChange addLabel onAdd />   {/* PageHeader + ViewToggle */}
  <KpiGrid heading={t("common.indicators")} kpis={...} />
  <SearchBadge search onClear />                                            {/* when searching */}
  {hasItems && <SectionHeading>{t("x.listing")}</SectionHeading>}
  <ListToolbar search onSearchChange onFilterClick activeFilterCount actions />
  <InventoryContent ... />          {/* loading / empty / cards / table; cards use stagger-in */}
  <Drawer footer={formFooter}> <EntityForm onFooterChange={setFormFooter} /> </Drawer>
  <InventoryFilterDrawer ... />
  <InventoryFAB ... />                                                       {/* phones */}
</PageShell>
```

Content grid: `grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4`, items
`className="stagger-in" style={{ "--i": i }}`. Skeleton: six `<SkeletonCard />`.

---

## 8. Adding a New Inventory Page

1. `app/{entity}/page.tsx` following section 7.
2. `app/{entity}/_components/EntityCard.tsx` from `Card` + inventory parts (section 2).
3. `app/{entity}/_components/EntityTableView.tsx` from `SortableTable` (controlled) + `Pagination`.
4. `app/{entity}/FilterDrawer.tsx` from `Drawer` + `DrawerSection` + `PillButton`.
5. `app/{entity}/EntityForm.tsx` following section 10 (footer slot, `FormField`).
6. `app/{entity}/[id]/` detail page following section 9.
7. Icons: add named paths to `lib/icon-paths.ts`.
8. Strings: both `messages/en.json` and `messages/pt-BR.json`, exact parity.
9. Scoped asset? Follow the entidades contract in the root CLAUDE.md.

---

## 9. Detail Page Pattern

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

## 10. Form Pattern

Every entity form is a multi-step wizard on create and the same component with
`initial` on edit. The form owns fields; the container owns actions.

```tsx
<Drawer title subHeader={formSubHeader} footer={formFooter}>
  <EntityForm initial onSuccess onSubHeaderChange={setFormSubHeader} onFooterChange={setFormFooter} />
</Drawer>
```

- `useMultiStepFormEffects({ step, setStep, totalSteps, stepLabels, onSubmit, canProceed, isPending, onFooterChange, onSubHeaderChange })`
  pushes `StepIndicator` into `subHeader` and Back / Next / Save into `footer`. Forms render no action bars of their own.
- Fields: `Input`, `Textarea`, `Select`, `NativeSelect`, `Checkbox`, `CheckboxList`, `RadioGroup`, `Toggle`,
  `DateTimeInput`, `TagInput`, `AsyncPicker`, `ContactInput`, `MarkdownEditor`; anything else inside `<FormField label required hint error>`.
- `required` draws the asterisk; `hint` is the helper line; per-field `error` under the control; form-level `<FormError>` at the top.
- Scope: `<EntidadeScopeFields value onChange compact>` with `defaultGrants(user)`; spread `...grants` into the payload.
- Cancel/Confirm dialogs (not wizards): `<ResponsiveModal footer={<FormFooter onCancel submitLabel onSubmit loading variant />}>`;
  a submit button outside its `<form>` uses `submitType="submit" form="<id>"`.
- Submit: `useState` per field, `useMutation`, `loading={mutation.isPending}`. No react-hook-form, no zod.

---

## 11. Responsaveis Pattern

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
- `Sync{Entity}Responsaveis()`: transactional replace pattern
- `Get{Entity}MainResponsavelNamesBulk()`: single query for list enrichment
- `List{Entity}Responsaveis()`: joined with contacts for detail pages

---

---

## 12. Acontecimentos Tab Pattern

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

---

## 13. Table View Pattern

- `SortableTable` (controlled `sortKey/sortDir/onSortChange` for server sort) + `Pagination`, 20 rows per page.
- Hand-written tables use `tableClasses` from `components/ui/Table` (`wrapper`, `table`, `headRow`, `th`, `row`, `rowAlt`, `td`).
- No inline edit/delete in inventory tables; actions live on the detail page. Row click navigates.
- Mono for IDs, hostnames, domains, tech values. Tags column shows 3 + `+N`.

---

## 14. Component Reference

`components/ui/`: Avatar, AsyncPicker, Badge, Button, Card (+CardIcon), Checkbox, CheckboxList, ContactInput,
CopyButton, DateTimeInput, DetailActions, DetailHeader, Divider, Drawer, DrawerSection, DropdownMenu (+Item),
EmptyState, Field, FloatingActionButton, FormError, FormField, FormFooter, Icon, IconButton, Input,
LinkedEntityList, ListToolbar, MarkdownEditor, Modal, NativeSelect, OperationOutput, PageHeader, Pagination,
PillButton, RadioGroup, ResponsiveModal, SearchBadge, SectionHeading, Select, Skeleton (+Card/Table/Stats),
SortDropdown, SortableTable, Spinner, StatCard, StatusAlert, StatusDot, StepIndicator, TabBar, Table
(`tableClasses`), TagInput, Textarea, Toggle, ToolbarActionButton, Tooltip, ViewToggle.

`components/inventory/`: CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator,
InventoryPageHeader, InventoryContent, InventoryFilterDrawer, InventoryFAB, KpiGrid, ResponsavelList,
ResponsaveisSection.

`hooks/`: useMultiStepForm, useCopy, useDebounce, useInventoryFilters, useSecretReveal, useMediaQuery.
`lib/`: icon-paths (`ICON_PATHS`, `NAV_ICONS`, `REQUEST_TYPE_ICON`), constants (`SITUACAO_*`, `situacaoAccent`,
`NAV_SECTIONS`), utils (`formatPhone`, `getTimeAgo`, ...), requests (`transitionVariant`, ...).

Retired: `ListingLabel`, the catalog's `SectionLabel`/`StepHeading`/`chipClass`, `VaultPage.Chip`, the
`.stagger-1..9` classes, `--shadow-glow`, `.animate-shimmer`, `.animate-slide-right`.
