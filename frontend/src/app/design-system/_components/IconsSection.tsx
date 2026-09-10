"use client";

import Icon from "@/components/ui/Icon";
import { ICON_PATHS, REQUEST_TYPE_ICON } from "@/lib/icon-paths";
import { VIEW_ICONS } from "@/components/ui/ViewToggle";
import { Section, Specimen } from "./Section";

export default function IconsSection() {
  return (
    <Section id="icons" title="Icons">
      <Specimen title="ICON_PATHS" source="lib/icon-paths.ts" wide>
        <div className="grid grid-cols-4 sm:grid-cols-6 lg:grid-cols-10 gap-3">
          {Object.entries(ICON_PATHS).map(([key, d]) => (
            <div key={key} className="flex flex-col items-center gap-1 min-w-0">
              <Icon path={d} className="w-5 h-5 text-[var(--text-secondary)]" />
              <span
                className="text-[10px] text-[var(--text-faint)] truncate w-full text-center"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {key}
              </span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="REQUEST_TYPE_ICON" source="lib/icon-paths.ts" wide>
        <div className="grid grid-cols-4 sm:grid-cols-6 lg:grid-cols-10 gap-3">
          {Object.entries(REQUEST_TYPE_ICON).map(([key, d]) => (
            <div key={key} className="flex flex-col items-center gap-1 min-w-0">
              <Icon path={d} className="w-5 h-5 text-[var(--text-secondary)]" />
              <span
                className="text-[10px] text-[var(--text-faint)] truncate w-full text-center"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {key}
              </span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="VIEW_ICONS" source="components/ui/ViewToggle.tsx" wide>
        <div className="grid grid-cols-4 sm:grid-cols-6 lg:grid-cols-10 gap-3">
          {Object.entries(VIEW_ICONS).map(([key, d]) => (
            <div key={key} className="flex flex-col items-center gap-1 min-w-0">
              <Icon path={d} className="w-5 h-5 text-[var(--text-secondary)]" />
              <span
                className="text-[10px] text-[var(--text-faint)] truncate w-full text-center"
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {key}
              </span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Icon" source="components/ui/Icon.tsx" wide>
        <div className="flex flex-wrap items-end gap-6">
          {(["xs", "sm", "md", "lg"] as const).map((size) => (
            <div key={size} className="flex flex-col items-center gap-1">
              <Icon path={ICON_PATHS.server} size={size} />
              <code className="text-2xs text-[var(--text-faint)] font-mono">size=&quot;{size}&quot;</code>
            </div>
          ))}
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-8 h-8 text-[var(--text-secondary)]" strokeWidth={1} />
            <code className="text-2xs text-[var(--text-faint)] font-mono">className + strokeWidth override</code>
          </div>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          <code>size</code> maps xs/sm/md/lg to w-3.5/4/5/6 with strokes 2/2/1.75/1.5, so small glyphs stop reading heavier
          than large ones. <code>className</code> and <code>strokeWidth</code> still override for one-offs.
          DESIGN_SYSTEM.md &sect;5: <code>w-3.5</code> for card indicators, <code>w-4</code> for toolbar buttons.
        </p>
      </Specimen>

      <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
        <li>
          One registry now: <code>lib/icon-paths.ts</code> holds <code>ICON_PATHS</code> (86 keys after two naming passes) plus <code>NAV_ICONS</code> (the sidebar set, still its own drawing).{" "}
          <code>EmptyState</code>, <code>ViewToggle</code>, <code>StatusAlert</code>, <code>Modal</code>, <code>PageHeader</code>{" "}
          and <code>Card</code> read from it instead of private maps.
        </li>
        <li>
          Inline <code>&lt;svg&gt;</code> went from 239 to 45 over two sweeps (2026-09-10): 175 sites in 90 files now render{" "}
          <code>Icon</code>, and the registry holds 86 named paths. What remains is meant to stay inline: multi-path glyphs,
          filled brand marks (WhatsApp, Keycloak, login providers), the Spinner arc, plus <code>Header.tsx</code> (in flight)
          and <code>app/secrets</code> (guardrailed).
        </li>
        <li>
          The magnifier path <code>M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z</code> alone is inlined in 11 files.
        </li>
      </ul>
    </Section>
  );
}
