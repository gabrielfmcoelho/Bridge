"use client";

import Button from "@/components/ui/Button";
import SectionHeading from "@/components/ui/SectionHeading";
import { ICON_PATHS } from "@/lib/icon-paths";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import { Section, Specimen } from "./Section";

// Data-driven ramps; counts are grep tallies across src at the time this
// catalog was written, not live (ponytail: a live grep-on-render is not
// worth the complexity for a dev-facing inventory page).
const SIZE_RAMP: [string, number][] = [
  ["text-2xl", 20],
  ["text-lg", 17],
  ["text-base", 13],
  ["text-sm", 390],
  ["text-xs", 647],
  ["text-[11px]", 122],
  ["text-[10px]", 343],
  ["text-[9px]", 40],
];

const WEIGHT_SIZE_COMBOS: [string, number][] = [
  ["text-xs font-medium", 97],
  ["text-xs font-semibold", 82],
  ["text-sm font-semibold", 80],
  ["text-sm font-medium", 55],
  ["text-2xl font-bold", 19],
  ["text-[10px] font-semibold", 14],
];

const H1_ALSO_IN = [
  "app/ssh-config/page.tsx:99",
  "app/ssh-keys/page.tsx:49",
  "app/releases/page.tsx:92",
  "app/issues/IssueBoard.tsx:240",
  "app/atlas/lineage/page.tsx:73",
  "app/atlas/topology/page.tsx:30",
  "app/tools/page.tsx:150",
  "components/atlas/pipeline/PipelinePageInner.tsx:82",
  "components/atlas/catalog/CatalogPageInner.tsx:111",
  "components/atlas/apis/ApiDetailInner.tsx:117",
];

export default function TypographySection() {
  return (
    <Section id="typography" title="Typography">
      <Specimen title="Page title (h1)" source="app/page.tsx:175 (inline)" alsoIn={H1_ALSO_IN} wide>
        <div className="flex flex-col gap-3">
          {/* specimen: app/page.tsx:175 */}
          <div className="flex flex-wrap items-baseline gap-3">
            <h1 className="text-2xl font-bold mb-6" style={{ fontFamily: "var(--font-display)" }}>
              Hosts
            </h1>
            <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              text-2xl font-bold mb-6 (app/page.tsx:175)
            </code>
          </div>
          {/* specimen: components/atlas/catalog/CatalogPageInner.tsx:111 */}
          <div className="flex flex-wrap items-baseline gap-3">
            <h1 className="text-2xl font-bold tracking-tight" style={{ fontFamily: "var(--font-display)" }}>
              Hosts
            </h1>
            <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              text-2xl font-bold tracking-tight (CatalogPageInner.tsx:111)
            </code>
          </div>
          {/* specimen: app/setup/page.tsx:61 (also app/login/page.tsx:123) */}
          <div className="flex flex-wrap items-baseline gap-3">
            <h1 className="text-3xl font-bold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
              Hosts
            </h1>
            <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              text-3xl font-bold text-[var(--text-primary)] (login / setup)
            </code>
          </div>
          {/* specimen: app/hosts/[slug]/HostDetail.tsx:145 */}
          <div className="flex flex-wrap items-baseline gap-3">
            <h1 className="text-2xl font-bold text-[var(--accent)]" style={{ fontFamily: "var(--font-display)" }}>
              Hosts
            </h1>
            <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              text-2xl font-bold text-[var(--accent)] (HostDetail.tsx:145)
            </code>
          </div>
          {/* specimen: components/ui/PageHeader.tsx:15 */}
          <div className="flex flex-wrap items-baseline gap-3">
            <h1 className="text-xl sm:text-2xl font-bold font-display">Hosts</h1>
            <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              text-xl sm:text-2xl font-bold font-display (ui/PageHeader.tsx; the one page title now; mb-6 on the wrapper)
            </code>
          </div>
        </div>
      </Specimen>

      <Specimen title="Section title (h2)" source="ad-hoc; no single canonical file" wide>
        <div className="space-y-2">
          <h2 className="text-sm font-semibold text-[var(--text-secondary)]">Section title</h2>
          <p className="text-xs text-[var(--text-muted)]">
            <code>text-sm font-semibold text-[var(--text-secondary)]</code>; 7 different spacing permutations
            around it across pages.
          </p>
        </div>
      </Specimen>

      <Specimen title="SectionHeading" source="components/ui/SectionHeading.tsx" wide>
        <div className="space-y-4">
          <SectionHeading actions={<Button size="sm" variant="ghost">Action</Button>}>
            variant=&quot;label&quot; (default)
          </SectionHeading>
          <SectionHeading variant="section">variant=&quot;section&quot;</SectionHeading>
          <SectionHeading variant="rule" count={15}>
            variant=&quot;rule&quot; count
          </SectionHeading>
          <SectionHeading variant="rule" as="h3" icon={ICON_PATHS.user} hint="Who is asking, and how to reach them.">
            variant=&quot;rule&quot; as=&quot;h3&quot; icon hint
          </SectionHeading>
          <p className="text-xs text-[var(--text-muted)]">
            The three heading recipes in one component. Replaced <code>ListingLabel</code>, the catalog&apos;s{" "}
            <code>SectionLabel</code> and <code>StepHeading</code>, and <code>KpiGrid</code>&apos;s h2. None of them is
            uppercase any more: uppercase letterspaced 10-12px text is the slowest text on a page to read, so it is
            reserved for table column headers (<code>tableClasses.th</code>), where the row below it is data. Everything
            else separates by weight and colour at the same size — <code>uppercase tracking-*</code> went from 149 uses
            to 2.
          </p>
        </div>
      </Specimen>

      <Specimen title="Size ramp" source="tailwind text-size utilities" wide>
        <div className="space-y-1.5">
          {SIZE_RAMP.map(([cls, count]) => (
            <div key={cls} className="flex flex-wrap items-baseline gap-3">
              <span className={cls}>Sample text</span>
              <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                {cls}
              </code>
              <span className="text-[10px] text-[var(--text-faint)]">{count}&times; in src</span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Colour ramp" source="app/globals.css" wide>
        <div className="space-y-1.5">
          <p className="text-sm text-[var(--text-primary)]">text-primary; primary body text</p>
          <p className="text-sm text-[var(--text-secondary)]">text-secondary; secondary text</p>
          <p className="text-sm text-[var(--text-muted)]">text-muted; muted / helper text</p>
          <p className="text-sm text-[var(--text-faint)]">text-faint; faint / label text</p>
        </div>
      </Specimen>

      <Specimen title="Weight / size combos" source="tailwind text-size + font-weight utilities" wide>
        <div className="space-y-1.5">
          {WEIGHT_SIZE_COMBOS.map(([cls, count]) => (
            <div key={cls} className="flex flex-wrap items-baseline gap-3">
              <span className={cls}>Sample text</span>
              <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                {cls}
              </code>
              <span className="text-[10px] text-[var(--text-faint)]">{count}&times; in src</span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Link" source="app/design-system/page.tsx (inline)" wide>
        <div className="space-y-2">
          <a href="#typography" className="text-[var(--accent)] hover:underline">
            Inline link
          </a>
          <p className="text-xs text-[var(--text-muted)]">
            2&times; in src. <code>components/ui/Field.tsx</code> link variant uses{" "}
            <code>text-sm text-[var(--accent)] hover:text-[var(--accent-hover)]</code>. External links follow{" "}
            <code>target=&quot;_blank&quot; rel=&quot;noopener noreferrer&quot;</code> (24&times;); no{" "}
            <code>&lt;ExternalLink&gt;</code> component exists.
          </p>
        </div>
      </Specimen>

      <Specimen title="Markdown prose" source="components/ui/MarkdownEditor.tsx" wide>
        <div className="space-y-2">
          <MarkdownContent
            content={"# Heading\n\nParagraph with **bold**, _italic_ and `code`.\n\n- item one\n- item two"}
          />
          <p className="text-xs text-[var(--text-muted)]">
            Scoped by <code>.markdown-preview</code>. <code>.wiki-doc .markdown-preview</code> restyles it for
            long-form content (15px / 1.7 line-height); <code>.glpi-content</code> is a separate scope for
            sanitised third-party HTML.
          </p>
        </div>
      </Specimen>
    </Section>
  );
}
