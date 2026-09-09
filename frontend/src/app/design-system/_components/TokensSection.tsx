"use client";

import { useEffect, useState } from "react";
import { useTheme } from "@/contexts/ThemeContext";
import Button from "@/components/ui/Button";
import { Section, Specimen } from "./Section";

// Data-driven token lists — see app/globals.css for the source of truth.
const COLORS = [
  "--bg-base", "--bg-surface", "--bg-elevated", "--bg-overlay",
  "--border-subtle", "--border-default", "--border-strong",
  "--text-primary", "--text-secondary", "--text-muted", "--text-faint",
  "--accent", "--accent-hover", "--accent-muted", "--accent-glow",
  "--success", "--success-muted", "--warning", "--warning-muted",
  "--danger", "--danger-muted", "--purple", "--purple-muted",
  "--amber", "--amber-muted", "--info", "--info-muted", "--cyan", "--rose", "--glass-bg", "--grid-line",
  "--selection-bg", "--selection-fg",
];
const RADII = ["--radius-sm", "--radius-md", "--radius-lg", "--radius-xl"];
const SHADOWS = ["--shadow-sm", "--shadow-md", "--shadow-lg", "--shadow-glow"];
const FONTS = ["--font-display", "--font-body", "--font-mono"];
const ANIMS = [
  "animate-fade-in", "animate-slide-up", "animate-slide-down", "animate-scale-in",
  "animate-slide-right", "animate-shimmer", "animate-pulse-glow",
];
const SPACING: [string, string][] = [
  ["mb-6", "page header margin"],
  ["mb-5", "KPI section / toolbar"],
  ["mb-3", "listing label"],
  ["gap-3", "KPI grid / indicator items"],
  ["gap-4", "card grid"],
  ["gap-x-4 gap-y-3", "metadata grid"],
  ["mt-3 pt-3 border-t", "tags separator"],
  ["p-3.5 md:p-5", "card padding"],
];

export default function TokensSection() {
  const { theme } = useTheme();
  const [vals, setVals] = useState<Record<string, string>>({});
  const [replay, setReplay] = useState(0);

  useEffect(() => {
    // ponytail: rAF — child effects run before ThemeProvider sets data-theme; a sync read here sees the old theme
    const id = requestAnimationFrame(() => {
      const cs = getComputedStyle(document.documentElement);
      setVals(
        Object.fromEntries(
          [...COLORS, ...RADII, ...SHADOWS, ...FONTS].map((n) => [n, cs.getPropertyValue(n).trim()])
        )
      );
    });
    return () => cancelAnimationFrame(id);
  }, [theme]);

  return (
    <Section id="tokens" title="Tokens">
      <Specimen title="Colours" source="app/globals.css" wide>
        <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3">
          {COLORS.map((n) => (
            <div key={n} className="space-y-1 min-w-0">
              <div
                style={{ background: `var(${n})` }}
                className="h-10 rounded-[var(--radius-md)] border border-[var(--border-subtle)]"
              />
              <p className="text-[11px] text-[var(--text-secondary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
                {n}
              </p>
              <p className="text-[10px] text-[var(--text-faint)] truncate">{vals[n]}</p>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Radii" source="app/globals.css" wide>
        <div className="flex flex-wrap gap-4">
          {RADII.map((n) => (
            <div key={n} className="space-y-1 text-center">
              <div
                style={{ borderRadius: `var(${n})` }}
                className="w-16 h-16 bg-[var(--bg-elevated)] border border-[var(--border-default)]"
              />
              <p className="text-[11px] text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
                {n}
              </p>
              <p className="text-[10px] text-[var(--text-faint)]">{vals[n]}</p>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Shadows" source="app/globals.css" wide>
        <div className="flex flex-wrap gap-4">
          {SHADOWS.map((n) => (
            <div key={n} className="space-y-1 text-center">
              <div
                style={{ boxShadow: `var(${n})` }}
                className="w-28 h-16 rounded-[var(--radius-md)] bg-[var(--bg-surface)]"
              />
              <p className="text-[11px] text-[var(--text-secondary)]" style={{ fontFamily: "var(--font-mono)" }}>
                {n}
              </p>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Fonts" source="app/globals.css" wide>
        <div className="space-y-3">
          {FONTS.map((n) => (
            <div key={n} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5">
              <p style={{ fontFamily: `var(${n})` }} className="text-sm text-[var(--text-primary)]">
                The quick brown fox — 0123456789
              </p>
              <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                {n}
              </code>
              <span className="text-[10px] text-[var(--text-faint)]">{vals[n]}</span>
            </div>
          ))}
          <p className="text-xs text-[var(--text-muted)]">
            <code>--font-display</code> and <code>--font-mono</code> are both JetBrains Mono; applied via inline{" "}
            <code>style</code> (201&times; mono / 90&times; display) — no <code>.font-display</code> utility exists.
          </p>
        </div>
      </Specimen>

      <Specimen title="Spacing" source="app/globals.css" wide>
        <div className="space-y-1.5">
          {SPACING.map(([cls, meaning]) => (
            <div key={cls} className="flex flex-wrap items-baseline gap-3 text-xs">
              <code className="text-[var(--text-secondary)] w-44 shrink-0" style={{ fontFamily: "var(--font-mono)" }}>
                {cls}
              </code>
              <span className="text-[var(--text-muted)]">{meaning}</span>
            </div>
          ))}
        </div>
      </Specimen>

      <Specimen title="Animations" source="app/globals.css" wide>
        <div className="space-y-4">
          <Button size="sm" variant="secondary" onClick={() => setReplay((r) => r + 1)}>
            Replay
          </Button>

          <div className="flex flex-wrap gap-3">
            {ANIMS.map((cls) => (
              <div key={`${cls}-${replay}`} className="space-y-1 text-center">
                <div className={`w-20 h-10 rounded-[var(--radius-md)] bg-[var(--accent-muted)] border border-[var(--accent)]/30 ${cls}`} />
                <p className="text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
                  {cls}
                </p>
              </div>
            ))}
          </div>

          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1.5" style={{ fontFamily: "var(--font-mono)" }}>
              .stagger-1..9
            </p>
            <div className="flex flex-wrap gap-2">
              {Array.from({ length: 9 }, (_, i) => i + 1).map((i) => (
                <div
                  key={i}
                  className={`w-10 h-10 rounded-[var(--radius-md)] bg-[var(--accent-muted)] border border-[var(--accent)]/30 animate-slide-up stagger-${i}`}
                />
              ))}
            </div>
          </div>

          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1.5" style={{ fontFamily: "var(--font-mono)" }}>
              .stagger-in (index-driven)
            </p>
            <div className="flex flex-wrap gap-2">
              {Array.from({ length: 6 }, (_, i) => i).map((i) => (
                <div
                  key={i}
                  className="w-10 h-10 rounded-[var(--radius-md)] bg-[var(--accent-muted)] border border-[var(--accent)]/30 stagger-in"
                  style={{ "--i": i } as React.CSSProperties}
                />
              ))}
            </div>
          </div>

          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1.5" style={{ fontFamily: "var(--font-mono)" }}>
              .skeleton
            </p>
            <div className="h-4 w-40 skeleton" />
          </div>

          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1.5" style={{ fontFamily: "var(--font-mono)" }}>
              .glass over .bg-grid
            </p>
            <div className="relative h-24 bg-grid rounded-[var(--radius-lg)]">
              <div className="absolute inset-4 glass rounded-[var(--radius-md)]" />
            </div>
          </div>
        </div>
      </Specimen>

      <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
        <li>No font-size tokens exist — the type scale lives in roughly 1,600 scattered classes.</li>
        <li>
          <code>@theme inline</code> maps only <code>--color-background</code> / <code>--color-foreground</code> /{" "}
          <code>--font-sans</code> / <code>--font-mono</code>.
        </li>
        <li>
          <code>@custom-variant light</code> enables <code>light:</code> classes.
        </li>
        <li>
          <code>--accent*</code> is overridden at runtime by <code>AppearanceContext</code> (<code>app_color</code>).
        </li>
        <li>
          <code>@media (prefers-reduced-motion: reduce)</code> collapses all animations.
        </li>
        <li>
          Light-theme <code>--success</code> / <code>--warning</code> / <code>--danger</code> carry WCAG contrast
          comments in <code>globals.css</code>.
        </li>
      </ul>
    </Section>
  );
}
