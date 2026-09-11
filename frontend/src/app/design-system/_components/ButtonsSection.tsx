"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Button from "@/components/ui/Button";
import IconButton from "@/components/ui/IconButton";
import PillButton from "@/components/ui/PillButton";
import CopyButton from "@/components/ui/CopyButton";
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

      <Specimen title="PillButton" source="components/ui/PillButton.tsx" wide>
        <div className="flex flex-wrap items-center gap-2">
          {PILL_KEYS.map((key) => (
            <PillButton key={key} active={pillFilter === key} onClick={() => setPillFilter(key)}>
              {key}
            </PillButton>
          ))}
          <span className="text-2xs text-[var(--text-faint)]">shape=&quot;rounded&quot; (default)</span>
        </div>
        <div className="flex flex-wrap items-center gap-2 mt-3">
          {PILL_KEYS.map((key, i) => (
            <PillButton key={key} shape="pill" count={[15, 9, 6][i]} active={pillFilter === key} onClick={() => setPillFilter(key)}>
              {key}
            </PillButton>
          ))}
          <span className="text-2xs text-[var(--text-faint)]">shape=&quot;pill&quot; count</span>
        </div>
        <div className="flex flex-wrap items-center gap-2 mt-3">
          <PillButton size="sm" shape="pill" active lead={<span className="w-1.5 h-1.5 rounded-full bg-[var(--warning)]" />} onClick={noop}>
            size=&quot;sm&quot; lead
          </PillButton>
          <PillButton size="lg" active={false} onClick={noop}>
            size=&quot;lg&quot;
          </PillButton>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          Sets <code>aria-pressed</code>; absorbed <code>CatalogSearch.chipClass()</code>, <code>VaultPage.Chip</code> and{" "}
          <code>PillFilter</code>&apos;s chips.
        </p>
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
            // ponytail: FAB color prop is a plain string; a CSS var works, no hex needed.
            { label: "Scan", icon: ICON_PATHS.scan, onClick: noop, color: "var(--accent)" },
          ]}
        />
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">
          Mobile only (<code>md:hidden</code>, fixed bottom-right); resize below 768px to see it.
        </p>
      </Specimen>

      <Specimen title="CopyButton / useCopy" source="components/ui/CopyButton.tsx, hooks/useCopy.ts" wide>
        <div className="flex flex-wrap items-center gap-3">
          <CopyButton value="Host web-01" />
          <CopyButton value="Host web-01" icon label="Copy config" copiedLabel="Copied" />
          <CopyButton value="ssh-rsa AAAA…" size="sm" variant="ghost" label="Copy key" />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>useCopy()</code> owns the clipboard call, the 2 s flash and the non-secure-context fallback;{" "}
          <code>CopyButton</code> is Button + the hook for the plain case. Migrated: ssh-config, SSHConfigDrawer (hook, two
          controls share one state), WikiShareModal, ShareBundleModal, ProjectSecretsSheet. Still inline:{" "}
          <code>glpi/DropdownCatalogueEditorModal.tsx:112</code>, <code>settings/IntegrationsTab.tsx:1464</code> (prompt
          fallback), <code>share/[token]/page.tsx:428</code> (keyed multi-copy), <code>secrets/ShareLinkModal.tsx</code> (guardrailed).
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
