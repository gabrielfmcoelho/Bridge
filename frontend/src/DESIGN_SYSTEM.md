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
   `--rose` for categorical identity (see §3 — there is no `--purple`). Tints are opacity modifiers:
   `bg-[var(--success)]/15 border-[var(--success)]/30`. No raw palette classes
   (`text-emerald-400`) and no hex in tsx: the light theme only works through
   the tokens. Swept app-wide on 2026-09-10 (`Header.tsx` on 2026-09-24); what is left is
   `app/secrets` and the atlas layer palette in `LayerBadge`.
2. **One registry for icons.** `lib/icon-paths.ts` (`ICON_PATHS`, `NAV_ICONS`)
   through `<Icon path size>`; `size` scales stroke with the box. Adding an
   icon means adding a named path there, never an inline `<svg>`.
3. **One card.** `<Card accent decorator padding selected as>`; children read
   `var(--card-accent)`. Inventory cards compose `CardHeader`, `CardMetadataGrid`,
   `CardTagsSection`, `CardIndicator` inside it. KPI tiles are `StatCard`: the
   same left stripe, value on `--text-primary`, one recipe in both themes (the
   old `tint` decorator, a wash in dark and a top rule in light, is gone).
   **Sections are `SectionCard`.** Any titled block of content — key-value
   info, a table, a list, a chart, a form — is `<SectionCard title description
   controls footer body variant>`: header (title, optional icon/count, minimal
   h-8 controls on the right; one muted description line) → hairline → body
   (`padded` p-5, or `flush` for tables and row lists) → optional footer bar
   (save bar, pagination, comment box). `variant="plain"` keeps the same header
   without card chrome when the body is itself cards; `collapsible` folds it
   (no controls in a collapsible header). A `SectionHeading` above a `Card`, or
   an `<h2>/<h3>` row hand-built inside one, is the pattern this replaces;
   `SectionHeading` stays for page-level list headings and inside drawers/modals.
   **Empty states have three levels**, nothing else: a missing *value* is a
   muted "–" (`Field`, `CardMetadataGrid`, table cells — never "-", "--" or
   "—"); a *section* with nothing keeps its place and says so in one muted line
   (`SectionCard empty="…" emptyAction`); a whole *pane or page* with nothing is
   `EmptyState compact` with the action that fills it. Sections are hidden only
   when the user may not see them or the feature is off; failures are a
   `StatusAlert`, never a blank area.
   **Detail pages** read left to right: what was declared (the entity's profile,
   1–2 `SectionCard`s, sticky) beside what was observed (scans, activity), which
   scrolls — the host detail "Visão geral" is the reference.
4. **Three headings.** `PageHeader` (title, subtitle, actions, add) for the page;
   `SectionHeading variant="section|label|rule"` inside it; `Heading as size`
   anywhere else. Sizes come from the heading scale in `globals.css`
   (`text-heading-xl|lg|md|sm|xs|xxs`, 28/32 → 12/16, after ADS
   `font.heading.*`); the page title is `sm` = 16px, section titles
   (`SectionCard`) `xs` = 14px; header chrome (breadcrumbs, Voltar, search) is 12px. `as` is the outline level, `size`
   the look: one h1 per page, no skipped levels. Titles wrap, never truncate.
   No inline `<h1>`/`<h2>` recipes.
5. **One field anatomy.** `FormField` = label, control, hint, error, required
   asterisk. `Input`, `Textarea`, `Select` (searchable), `NativeSelect` (plain)
   render through it and wire `useId` into `htmlFor`/`id` themselves, so the
   label is really associated; a control with no `label` falls back to its
   placeholder as `aria-label`. Wrap anything else in it. Actions sit in the Modal/Drawer
   `footer` slot: `FormFooter` for Cancel/Confirm, `useMultiStepForm` for wizards.
6. **Buttons are `Button`, `IconButton`, `PillButton`** (chips: `shape`, `count`,
   `lead`), `ViewToggle`, `ToolbarActionButton`, `ToolbarSelect`, `CopyButton`. All press
   (`active:scale`); the global `*:focus-visible` ring is the focus indicator.
   `IconButton` requires `label` (accessible name + tooltip): an icon alone has
   no name. One primary button per page or area.
   **Tabs** (`TabBar`) are underlined, for sections of one page, with arrow-key
   navigation; a choice of how a list reads (cards/table, group by) is a
   `ToolbarSelect`, not tabs.
   **Menus** (`DropdownMenu`): `DropdownMenuGroup title` (title every group once
   there are two), `DropdownMenuItem elemBefore elemAfter description`,
   `DropdownMenuRadioItem` (pick one, closes) and `DropdownMenuCheckboxItem`
   (toggle, stays open).
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
    **Status vs tag vs count**: `Lozenge` (square-ish, `appearance`
    default/success/removed/inprogress/new/moved) is a state — `Badge
    variant="situacao"` uses the same shape; `Tag` (neutral pill, optionally
    removable) is metadata you filter by; `Badge` is a category or a count.
    **Feedback**: `useFlag()` for outcomes of background actions (bottom-left,
    8s auto-dismiss for success/info, errors stay, max 2 actions);
    `StatusAlert` for a problem with the page/section; `FormError` in forms.
    **Confirm**: `useConfirm()` (`ResponsiveModal size="sm"` + `FormFooter`),
    `danger` for destructive, `requireText` for irreversible (backup restore).
    **Overlays**: `Modal`/`ResponsiveModal size="sm|md|lg|xl"` (400/672/800/968px).
    **Row actions**: `RowActions` ("…" menu, trigger named after the row).
    **Side nav** (`NAV_SECTIONS` in `lib/constants`): `count` on an item shows a
    live number after the label (a dot on the icon in the collapsed rail;
    announced as a phrase, never a bare number); `collapsible` on a section
    folds its items under the heading (remembered; the section holding the
    current page never folds; the rail always shows the icons).
    **Empty state**: 464px (304 compact), heading + description, one CTA
    (`action`) and an optional `secondaryAction`.
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
    sans, not a third face). `CardHeader titleFont` and `PageHeader
    titleFont` carry the choice per entity.
14. **Modals and drawers are dialogs**: `role="dialog"`, `aria-modal`,
    labelled by their title, closable with Escape, and a named close button.
    `Modal` and `Drawer` both do this — use them rather than a fixed overlay.
15. **Motion is capped.** `.stagger-in` delays cap at 8 items (~240ms total);
    surfaces and buttons transition at `duration-150`, colour-only changes at
    `duration-100`. Cards do not lift on hover — they change surface. No
    glows. `prefers-reduced-motion` collapses all of it and must stay.
16. **Elevation is `--elevate` / `--elevate-hi`**, never `--shadow-*` directly
    on a surface. In dark they are a 1px inner top highlight (a drop shadow
    has nowhere to land on `#080c14`); in light they map to the real shadows.
17. **State is never colour alone** (WCAG 1.4.1). A status dot carries shape
    (filled vs ring) and an accessible name; prefer showing the label.
18. **Fixed anatomy.** Every slot of a card and of a page header renders, in
    a fixed order, whether or not it has data — the user learns where each
    fact lives, and position carries meaning. Empty values read "–" (muted),
    a zero count is a faint `0`, a meter with no reading shows its empty
    track and "–", a tag row with no tags shows "–", an indicator whose data
    the API doesn't send is `disabled` (dimmed, "não disponível"), and a flag
    (`hideCount`) keeps its off state. `CardMetadataGrid`, `CardIndicator`,
    `CardTagsSection` and `CardHeader` implement this; cards in a row stretch
    to the tallest (`InventoryContent` cells; the card `<Link>` is `block h-full`).

---

## 2. Card Anatomy

Every inventory card has five vertical sections inside `<Card accent={situacaoAccent(...)}>`:

```
+------------------------------------------+
| HEADER: title / subtitle / status / desc |  <- CardHeader (+ corner)
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

- Title: sm, semibold, mono when it is an identifier (`titleFont`). Subtitle (`subtitleFont`): xs, "–" when absent.
  Status: `<SituacaoText>` (or `StatusText`) — text with a dot, never a badge. Description: xs, muted, one line, "–" when absent.
  `corner`: the top-right slot (host quick-look). No chevron: the whole card is the link.
- Metadata grid `grid-cols-2 gap-x-4 gap-y-3`; labels muted xs, values secondary xs; empty values "–" (rule 18).
- Tags: `mt-3 pt-3 border-t border-subtle`, max 4 + `+N`; "–" when there are none.
- Indicators: `mt-auto pt-4 border-t`; every indicator always present — 0 faint, unavailable `disabled`, flags (`hideCount`, e.g. idle) off when off.
- Hosts add the resources block (CPU/RAM/Disco), always shown, "–" without a scan.
- Card accent: entity cards pass `situacaoAccent(situacao, enumColor)`; services pass
  `danger | cyan | warning` for external-dependency / in-house / vendor.

### Redundancy rules

Each piece of data should appear in **exactly one** card section:

| Data Type              | Appears In      | NOT In            |
|-----------------------|-----------------|-------------------|
| Primary name/ID       | Header (title)  | Grid, indicators  |
| Secondary identifier  | Header (subtitle)| Grid              |
| Status/situacao       | Header (status line) | Grid, indicators |
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
| Projects, Issues | `--accent` |
| Containers | `--info` |
| Chamados | `--warning` |

`lib/constants.ts` `ENTITY_INDICATOR_COLORS` is on tokens as of the 2026-09-10 sweep.

**There is no `--purple`.** It sat in the same indigo/violet arc as `--accent`
(which the backend overrides at runtime with `app_color`), so entity identity
and "this is interactive" read as the same family. Everything that was purple
is `--accent`. The `purple`/`violet` keys survive in `Badge`, `Card`,
`StatusDot` and `CardIndicator` only as aliases for `app/secrets`, which a
tooling guardrail prevents editing — do not use them in new code.

---

## 4. Spacing

| Element | Spacing |
|---|---|
| Page header margin | `mb-6` (inside `PageHeader`) |
| KPI section | `mb-5` (inside `KpiGrid`) |
| Listing label | `mb-3` (inside `SectionHeading`) |
| Toolbar | none of its own; `PageHeader controls` (header rows `space-y-3`, `mb-6`) |
| Card grid | `gap-4` |
| Card padding | `p-3.5 md:p-5` (`Card padding="md"`), `p-4` (`"sm"`) |
| Modal / Drawer body and footer | `p-4 md:p-5` |
| Metadata grid | `gap-x-4 gap-y-3` |

---

## 5. Badges

| Usage | Props |
|---|---|
| Entity status | `<SituacaoText situacao>` (text + dot; the badge variant is legacy, secrets only) |
| Meaning | `color="success | warning | danger | info"` |
| Category | `color="cyan | purple | rose"` (hue keys `emerald`, `amber`, `red`, `sky` still resolve) |
| Tags | default |

Compact badges are a dot that expands with its label on hover.

---

## 6. Card Navigation

- All cards wrap in `<Link>` (semantic HTML, SSR-friendly, right-click works); no corner chevron — hover changes the surface.
- Whole-card controls that are not navigation (catalog offerings, atlas tables) are `<Card as="button">`.
- Never `onClick` + `router.push()` for navigation.

---

## 7. Page Header and List Page Pattern

`PageHeader` is the one header for list and detail pages. Its rows never move:

```
1  title ····························· actions · edit · delete · add · [tools toggle]
2  subtitle / slug
3  status  |  indicators
4  description
5  controls  (search, filters, group, view, export, ações em lote, personalizar) — hidable (controlsKey)
6  bottom edge: divider | tabs (top, underlined) | side list (variant "side", Configurações)
```

```tsx
<PageShell>
  <PageHeader title addLabel onAdd hideAddOnPhone
    controls={<ListToolbar search filters actions viewMode onViewModeChange />}
    controlsKey="hosts" controlsBadge={activeFilterCount}
    tabs={{ idBase, label, active, onChange, items }}>     {/* optional */}
    {/* tab content */}
  </PageHeader>
  <SearchBadge search onClear />
  {hasItems && <SectionHeading>{t("x.listing")}</SectionHeading>}
  <InventoryContent columns={4 | 3} ... />
  <Drawer footer={formFooter}> <EntityForm onFooterChange={setFormFooter} /> </Drawer>
  <InventoryFilterDrawer ... />
  <InventoryFAB ... />                                                       {/* phones */}
</PageShell>
```

Hosts is the reference: tabs **Visão geral** (statistics `KpiGrid layout="list"` in the left quarter,
sticky; the listing in the other three quarters with `InventoryContent columns={3}`) and **Dashboard**
(KPI tiles + breakdown bar lists; a row click applies its filter). Content grid: `columns={4}`
→ `md:grid-cols-2 xl:grid-cols-4`, `columns={3}` → `md:grid-cols-2 xl:grid-cols-3`; items
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

Same `PageHeader`, with the entity's state and CRUD built in:

```tsx
<PageShell>
  <PageHeader
    title={entity.name}
    titleFont="mono"            // mono for slugs/domains, display for names
    subtitle={entity.slug} subtitleFont="mono"
    status={<SituacaoText situacao={entity.situacao} />}
    indicators={<>{/* CardIndicator: alerts, issues, chamados, idle… — always present */}</>}
    description={entity.description}
    actions={/* entity extras, e.g. SSH config */}
    onEdit={canEdit ? openEdit : undefined}
    onDelete={isAdmin ? remove : undefined} deleteConfirmMessage={…}   // confirms via useConfirm
    tabs={{ idBase: "host", label: entity.name, active: tab, onChange: setTab, items: tabs }}
  >
    {tab === "overview" && <OverviewTab />}
    {tab === "issues" && <IssuesTab />}
  </PageHeader>

  <Drawer open={showEditDrawer} title="Edit" subHeader={formSubHeader}>
    <EntityForm initial={entity} onSuccess={...} onSubHeaderChange={...} />
  </Drawer>

  {/* Phones: the header's CRUD row covers edit/delete; a FAB only for tab-specific "add" actions. */}
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
CopyButton, DateTimeInput, Divider, Drawer, DrawerSection, DropdownMenu (+Item),
EmptyState, Field, FloatingActionButton, FormError, FormField, FormFooter, Icon, IconButton, Input,
LinkedEntityList, ListToolbar, MarkdownEditor, Modal, NativeSelect, OperationOutput, PageHeader, Pagination,
PillButton, RadioGroup, ResponsiveModal, SearchBadge, SectionHeading, Select, Skeleton (+Card/Table/Stats),
SortDropdown, SortableTable, Spinner, StatCard, StatusAlert, StatusDot, StepIndicator, TabBar, Table
(`tableClasses`), TagInput, Textarea, Toggle, ToolbarActionButton, Tooltip, ViewToggle.

`components/inventory/`: CardHeader, CardMetadataGrid, CardTagsSection, CardIndicator, CardIndicatorSeparator,
InventoryContent, InventoryFilterDrawer, InventoryFAB, KpiGrid, ResponsavelList,
ResponsaveisSection.

`hooks/`: useMultiStepForm, useCopy, useDebounce, useInventoryFilters, useSecretReveal, useMediaQuery.
`lib/`: icon-paths (`ICON_PATHS`, `NAV_ICONS`, `REQUEST_TYPE_ICON`), constants (`SITUACAO_*`, `situacaoAccent`,
`NAV_SECTIONS`), utils (`formatPhone`, `getTimeAgo`, ...), requests (`transitionVariant`, ...).

Retired: `ListingLabel`, the catalog's `SectionLabel`/`StepHeading`/`chipClass`, `VaultPage.Chip`, the
`.stagger-1..9` classes, `--shadow-glow`, `.animate-shimmer`, `.animate-slide-right`.
