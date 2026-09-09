"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import PillButton from "@/components/ui/PillButton";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import FloatingActionButton from "@/components/ui/FloatingActionButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

const noop = () => {};

const BUTTON_VARIANTS = ["primary", "secondary", "danger", "ghost"] as const;
const BUTTON_SIZES = ["sm", "md", "lg"] as const;
const ICON_BUTTON_VARIANTS = ["default", "outline", "accent", "danger", "active"] as const;
const ICON_BUTTON_SIZES = ["sm", "md"] as const;
const PILL_KEYS = ["all", "prod", "staging"] as const;

export default function ButtonsSection() {
  const [pillFilter, setPillFilter] = useState<string>("all");
  const [copied, setCopied] = useState(false);

  const handleCopy = () => {
    navigator.clipboard
      .writeText("Host web-01\n  HostName 10.0.0.4")
      .then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      })
      .catch(() => {});
  };

  return (
    <Section id="buttons" title="Buttons">
      <Specimen title="Button" source="components/ui/Button.tsx" wide>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
          {BUTTON_VARIANTS.map((variant) =>
            BUTTON_SIZES.map((size) => (
              <Button key={`${variant}-${size}`} variant={variant} size={size}>
                {variant} / {size}
              </Button>
            )),
          )}
        </div>
        <div className="flex flex-wrap gap-3 mt-3">
          <Button loading>Loading</Button>
          <Button disabled>Disabled</Button>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>danger</code> reads <code>--danger</code> (theme-tuned in globals.css); the old light/dark branch via{" "}
          <code>useTheme()</code> is gone.
        </p>
      </Specimen>

      <Specimen title="IconButton" source="components/ui/IconButton.tsx">
        {ICON_BUTTON_VARIANTS.map((variant) =>
          ICON_BUTTON_SIZES.map((size) => (
            <IconButton key={`${variant}-${size}`} variant={variant} size={size} title={`${variant} / ${size}`}>
              <Icon path={ICON_PATHS.edit} />
            </IconButton>
          )),
        )}
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">No loading state.</p>
      </Specimen>

      <Specimen title="PillButton" source="components/ui/PillButton.tsx">
        {PILL_KEYS.map((key) => (
          <PillButton key={key} active={pillFilter === key} onClick={() => setPillFilter(key)}>
            {key}
          </PillButton>
        ))}
      </Specimen>

      <Specimen title="ToolbarActionButton" source="components/ui/ToolbarActionButton.tsx">
        <ToolbarActionButton icon={ICON_PATHS.exportDoc} label="Export" onClick={noop} hideLabel="sm" />
        <ToolbarActionButton icon={ICON_PATHS.exportDoc} label="Export" onClick={noop} hideLabel="md" />
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">
          Visually close to <code>Button</code> secondary/sm, but a frozen one-off style.
        </p>
      </Specimen>

      <Specimen title="FloatingActionButton" source="components/ui/FloatingActionButton.tsx">
        <FloatingActionButton
          actions={[
            { label: "New host", icon: ICON_PATHS.plus, onClick: noop },
            // ponytail: FAB color prop is a plain string — a CSS var works, no hex needed.
            { label: "Scan", icon: ICON_PATHS.scan, onClick: noop, color: "var(--purple)" },
          ]}
        />
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">
          Mobile only (<code>md:hidden</code>, fixed bottom-right) — resize below 768px to see it.
        </p>
      </Specimen>

      <Specimen
        title="Copy to clipboard"
        source="app/hosts/[slug]/_components/SSHConfigDrawer.tsx:68-90"
        alsoIn={[
          "components/atlas/apis/ShareBundleModal.tsx:328",
          "components/atlas/apis/ProjectSecretsSheet.tsx:48",
          "components/wiki/WikiShareModal.tsx:100",
          "components/glpi/DropdownCatalogueEditorModal.tsx:112",
          "app/ssh-config/page.tsx:77",
          "app/settings/IntegrationsTab.tsx:1475",
          "app/share/[token]/page.tsx:429",
          "app/secrets/_components/ShareLinkModal.tsx:111 (guardrailed path)",
        ]}
        wide
      >
        {/* specimen: app/hosts/[slug]/_components/SSHConfigDrawer.tsx:68-90 */}
        <div className="relative group">
          <pre
            className="text-xs text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-4 overflow-x-auto whitespace-pre"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            {"Host web-01\n  HostName 10.0.0.4"}
          </pre>
          <button
            type="button"
            onClick={handleCopy}
            className="absolute top-2 right-2 p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:border-[var(--border-default)] transition-all"
            title="Copy"
          >
            {copied ? (
              <svg className="w-4 h-4 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                />
              </svg>
            )}
          </button>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          9 inline implementations, no shared <code>CopyButton</code>.
        </p>
      </Specimen>

      <div className="space-y-1">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Worklist</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            ~220 {"<button>"} tags in src, 79 files import <code>Button</code>.
          </li>
          <li>
            Raw-button hot spots: <code>app/settings/page.tsx</code> (16), <code>components/layout/Header.tsx</code> (13),{" "}
            <code>app/wiki/page.tsx</code> (11), <code>app/tools/page.tsx</code> (8),{" "}
            <code>app/share/[token]/page.tsx</code> (8), <code>app/settings/IntegrationsTab.tsx</code> (8).
          </li>
          <li>
            <code>app/releases/page.tsx:146</code> copies <code>PillButton</code>&apos;s class string;{" "}
            <code>components/glpi/FormcreatorFormDrawer.tsx:365</code> hand-rolls a primary button;{" "}
            <code>components/ui/Modal.tsx:47</code> hand-rolls an <code>IconButton</code>.
          </li>
          <li>
            <code>Button</code> and <code>IconButton</code> danger now use <code>--danger</code>; the ad-hoc copies (
            <code>bg-red-500/10 text-red-400</code>) still do not.
          </li>
        </ul>
      </div>
    </Section>
  );
}
