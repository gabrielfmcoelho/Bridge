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
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-3.5 h-3.5 text-[var(--text-secondary)]" />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              w-3.5 h-3.5
            </code>
          </div>
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-4 h-4 text-[var(--text-secondary)]" />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              w-4 h-4 (default)
            </code>
          </div>
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-5 h-5 text-[var(--text-secondary)]" />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              w-5 h-5
            </code>
          </div>
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-6 h-6 text-[var(--text-secondary)]" />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              w-6 h-6
            </code>
          </div>
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-5 h-5 text-[var(--text-secondary)]" strokeWidth={1.5} />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              strokeWidth=1.5
            </code>
          </div>
          <div className="flex flex-col items-center gap-1">
            <Icon path={ICON_PATHS.server} className="w-5 h-5 text-[var(--text-secondary)]" strokeWidth={2} />
            <code className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
              strokeWidth=2 (default)
            </code>
          </div>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-3">
          DESIGN_SYSTEM.md &sect;5: <code>w-3.5</code> for card indicators, <code>w-4</code> for toolbar buttons.
        </p>
      </Specimen>

      <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
        <li>
          Five separate icon registries: <code>lib/icon-paths.ts</code> (canonical, ~39 keys, imported by 15
          files), <code>components/layout/Sidebar.tsx</code> private <code>icons</code> map (21 keys),{" "}
          <code>components/ui/EmptyState.tsx</code> private map (7), <code>components/ui/ViewToggle.tsx</code>{" "}
          <code>VIEW_ICONS</code> (3), <code>components/ui/Card.tsx</code> <code>clickIndicator</code> (2).
        </li>
        <li>
          <code>Icon.tsx</code> is imported by only 5 files while 239 raw inline <code>&lt;svg&gt;</code> exist
          across <code>.tsx</code> files.
        </li>
        <li>
          The magnifier path <code>M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z</code> alone is inlined in 11 files.
        </li>
      </ul>
    </Section>
  );
}
