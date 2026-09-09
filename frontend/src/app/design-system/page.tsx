"use client";

import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import EmptyState from "@/components/ui/EmptyState";
import { Section } from "./_components/Section";
import TokensSection from "./_components/TokensSection";
import TypographySection from "./_components/TypographySection";
import ButtonsSection from "./_components/ButtonsSection";
import FormsSection from "./_components/FormsSection";
import FeedbackSection from "./_components/FeedbackSection";
import CardsSection from "./_components/CardsSection";
import TablesSection from "./_components/TablesSection";
import TabsSection from "./_components/TabsSection";
import OverlaysSection from "./_components/OverlaysSection";
import NavSection from "./_components/NavSection";
import IconsSection from "./_components/IconsSection";

// Dev-facing catalog: every shared primitive next to its ad-hoc duplicates.
// English-only on purpose (not user-facing); only nav.designSystem is i18n'd.
const SECTIONS = [
  ["tokens", "Tokens"],
  ["shells", "Shells"],
  ["typography", "Typography"],
  ["buttons", "Buttons"],
  ["forms", "Inputs & Forms"],
  ["feedback", "Feedback"],
  ["cards", "Cards"],
  ["tables", "Tables & Lists"],
  ["tabs", "Tabs & Toggles"],
  ["overlays", "Overlays"],
  ["nav", "Navigation & Toolbars"],
  ["icons", "Icons"],
] as const;

export default function DesignSystemPage() {
  const { user } = useAuth();

  // Same inline role check the rest of the app uses (app/settings/page.tsx).
  if (user?.role !== "admin") {
    return (
      <PageShell>
        <EmptyState icon="key" title="Admins only" description="The design-system catalog is restricted to admin users." />
      </PageShell>
    );
  }

  return (
    <PageShell>
      <h1 className="text-2xl font-bold mb-2" style={{ fontFamily: "var(--font-display)" }}>
        Design System
      </h1>
      <p className="text-sm text-[var(--text-muted)] mb-4">
        Inventory of every UI primitive in Bridge — shared components from <code>components/ui</code> next to the
        ad-hoc copies scattered through pages. Each block names its source and the places that duplicate it. Counts
        on this page are grep tallies taken as of 2026-09-09 and will drift as the codebase changes.
      </p>

      {/* ponytail: anchors jump, no scroll-spy. Sticky works because <main> in PageShell is the scroll container;
          the negative top/margins cancel main's p-3 md:p-6 so the bar sits flush under the Header. */}
      <nav className="sticky -top-3 md:-top-6 z-20 -mx-3 md:-mx-6 px-3 md:px-6 py-2 mb-6 bg-[var(--bg-base)]/90 backdrop-blur border-b border-[var(--border-subtle)] flex flex-wrap gap-1">
        {SECTIONS.map(([id, label]) => (
          <a
            key={id}
            href={`#${id}`}
            className="px-2.5 py-1 text-xs rounded-[var(--radius-md)] text-[var(--text-muted)] hover:text-[var(--accent)] hover:bg-[var(--bg-elevated)] transition-colors"
          >
            {label}
          </a>
        ))}
      </nav>

      <div className="space-y-14">
        <TokensSection />

        {/* ponytail: shells are prose — you are looking at them. */}
        <Section id="shells" title="Shells">
          <ul className="text-sm text-[var(--text-secondary)] space-y-2 list-disc pl-5">
            <li>
              <code>components/layout/PageShell.tsx</code> — <code>{"{children, fullBleed?}"}</code>. Auth guard + redirect
              to <code>/setup</code> / <code>/login</code>, private <code>LoadingSkeleton</code>, then Sidebar + Header +{" "}
              <code>{"<main>"}</code> + MobileBottomNav. Every authenticated page wraps itself in it.
            </li>
            <li>
              <code>components/layout/Sidebar.tsx</code> — <code>{"{collapsed, mobileOpen, onCloseMobile}"}</code>. Sections
              from <code>NAV_SECTIONS</code> (<code>lib/constants.ts</code>) filtered by <code>user.permissions</code> /{" "}
              <code>user.role</code>. Owns a private 21-key icon map (separate from <code>lib/icon-paths.ts</code>).
            </li>
            <li>
              <code>components/layout/Header.tsx</code> — collapse toggle, Breadcrumbs, AI-chat trigger (<code>ai.use</code>
              ), EN/PT segmented toggle, theme toggle, avatar dropdown (ad-hoc — see Overlays).
            </li>
            <li>
              <code>components/layout/Breadcrumbs.tsx</code> + <code>lib/breadcrumbs.ts</code> — pure{" "}
              <code>buildCrumbs(pathname, NAV_ITEMS)</code>; this page&apos;s crumb resolves from the nav entry.
            </li>
            <li>
              <code>components/layout/MobileBottomNav.tsx</code> — its own hardcoded 5-item Portuguese list, not{" "}
              <code>NAV_SECTIONS</code>, not i18n&apos;d.
            </li>
          </ul>
        </Section>

        <TypographySection />
        <ButtonsSection />
        <FormsSection />
        <FeedbackSection />
        <CardsSection />
        <TablesSection />
        <TabsSection />
        <OverlaysSection />
        <NavSection />
        <IconsSection />
      </div>
    </PageShell>
  );
}
