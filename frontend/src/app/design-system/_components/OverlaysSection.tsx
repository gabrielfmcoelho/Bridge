"use client";

import { useState } from "react";
import { Section, Specimen } from "./Section";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Modal from "@/components/ui/Modal";
import Drawer from "@/components/ui/Drawer";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import StepIndicator from "@/components/ui/StepIndicator";
import Tooltip from "@/components/ui/Tooltip";
import SortDropdown from "@/components/ui/SortDropdown";
import { ICON_PATHS } from "@/lib/icon-paths";

const noop = () => {};

// roleColors: copied verbatim from components/layout/Header.tsx:13-17 (app/settings/page.tsx
// duplicates the same map).
const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-purple-500/10 text-purple-400/70 border-purple-500/15",
  viewer: "bg-[var(--bg-overlay)] text-[var(--text-faint)] border-[var(--border-subtle)]",
};

const STATIC_USER = { display_name: "Ana Souza", username: "ana", role: "admin" };

type SortKey = "name" | "date";
const SORT_OPTIONS: { key: SortKey; label: string }[] = [
  { key: "name", label: "Name" },
  { key: "date", label: "Date" },
];

export default function OverlaysSection() {
  const [open, setOpen] = useState<string | null>(null);
  const close = () => setOpen(null);

  const [sortValue, setSortValue] = useState<SortKey>("name");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("asc");

  return (
    <Section id="overlays" title="Overlays">
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" size="sm" onClick={() => setOpen("modal")}>Modal</Button>
        <Button variant="secondary" size="sm" onClick={() => setOpen("drawer")}>Drawer</Button>
        <Button variant="secondary" size="sm" onClick={() => setOpen("drawer-bottom")}>Drawer (bottom)</Button>
        <Button variant="secondary" size="sm" onClick={() => setOpen("drawer-wide")}>Drawer (wide)</Button>
        <Button variant="secondary" size="sm" onClick={() => setOpen("responsive")}>ResponsiveModal</Button>
      </div>

      <Specimen title="Modal" source="components/ui/Modal.tsx" wide>
        <Modal
          open={open === "modal"}
          onClose={close}
          title="Edit host"
          subHeader={<StepIndicator steps={["Basics", "Scope"]} current={1} />}
          footer={
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={close}>Cancel</Button>
              <Button>Save</Button>
            </div>
          }
        >
          <Input label="Hostname" defaultValue="web-01" />
        </Modal>
        <p className="text-[11px] text-[var(--text-muted)]">
          Hand-rolled (no Radix/vaul), <code>.glass</code> panel, <code>max-w-2xl</code>, sets{" "}
          <code>document.body.style.overflow</code>; desktop-only — 4 components use it bare and get a cramped
          modal on phones (<code>components/wiki/WikiShareModal.tsx</code>,{" "}
          <code>components/atlas/apis/{"{AddApiModal,EditApiModal,ShareBundleModal}"}.tsx</code>).
        </p>
      </Specimen>

      <Specimen title="Drawer" source="components/ui/Drawer.tsx" wide>
        <Drawer
          open={open === "drawer"}
          onClose={close}
          title="Edit host"
          headerAction={
            <IconButton variant="outline" size="sm" onClick={noop} title="More">
              <Icon path={ICON_PATHS.edit} />
            </IconButton>
          }
          onBack={noop}
          footer={
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={close}>Cancel</Button>
              <Button>Save</Button>
            </div>
          }
        >
          <Input label="Hostname" defaultValue="web-01" />
        </Drawer>
        <Drawer open={open === "drawer-bottom"} onClose={close} title="Bottom drawer" side="bottom">
          <Input label="Hostname" defaultValue="web-01" />
        </Drawer>
        <Drawer open={open === "drawer-wide"} onClose={close} title="Wide drawer" wide>
          <Input label="Hostname" defaultValue="web-01" />
        </Drawer>
        <p className="text-[11px] text-[var(--text-muted)]">
          Built on <strong>vaul</strong> (the only vaul import); guards <code>onPointerDownOutside</code> against{" "}
          <code>[data-portal-dropdown]</code> so Radix popovers inside don&apos;t close it;{" "}
          <code>DESIGN_SYSTEM.md</code> §14 says entity forms always live in <code>Drawer</code>.
        </p>
      </Specimen>

      <Specimen title="ResponsiveModal" source="components/ui/ResponsiveModal.tsx">
        <ResponsiveModal open={open === "responsive"} onClose={close} title="Edit host">
          <Input label="Hostname" defaultValue="web-01" />
        </ResponsiveModal>
        <p className="text-[11px] text-[var(--text-muted)] w-full">
          Drawer below 768px, Modal above; 22 consumers — the default choice.
        </p>
      </Specimen>

      <Specimen title="Tooltip" source="components/ui/Tooltip.tsx">
        <Tooltip content="Applies to the selected rows" side="top">
          <button className="px-3 py-1.5 text-xs rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors">
            Hover (top)
          </button>
        </Tooltip>
        <Tooltip content="Applies to the selected rows" side="right">
          <button className="px-3 py-1.5 text-xs rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors">
            Hover (right)
          </button>
        </Tooltip>
        <p className="text-[11px] text-[var(--text-muted)] w-full">
          Instantiates its own Radix <code>Provider</code> per instance instead of once in{" "}
          <code>app/providers.tsx</code>.
        </p>
      </Specimen>

      <Specimen title="SortDropdown" source="components/ui/SortDropdown.tsx">
        <SortDropdown
          options={SORT_OPTIONS}
          value={sortValue}
          direction={sortDir}
          onChange={(key, dir) => {
            setSortValue(key);
            setSortDir(dir);
          }}
        />
        <p className="text-[11px] text-[var(--text-muted)] w-full">
          Ad-hoc dropdown with a <code>fixed inset-0</code> click-catcher; 2 consumers.
        </p>
      </Specimen>

      <Specimen
        title="Header user dropdown"
        source="components/layout/Header.tsx:151-191"
        alsoIn={["components/ui/SortDropdown.tsx:31 (fixed inset-0 catcher)", "app/ssh-config/page.tsx:137 (third variant)"]}
        wide
      >
        {/* specimen: components/layout/Header.tsx:151-191 */}
        <div className="relative inline-block w-56 h-[170px]">
          <button
            onClick={noop}
            className="w-8 h-8 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-xs font-semibold text-[var(--text-secondary)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-all"
          >
            {STATIC_USER.display_name.charAt(0).toUpperCase()}
          </button>

          <div className="absolute right-0 top-full mt-2 w-56 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-lg)] shadow-[var(--shadow-lg)] overflow-hidden z-50 animate-fade-in">
            <div className="px-4 py-3 border-b border-[var(--border-subtle)]">
              <div className="flex items-center gap-2.5">
                <div className="w-9 h-9 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-sm font-semibold text-[var(--text-secondary)]">
                  {STATIC_USER.display_name.charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-[var(--text-primary)] truncate">
                    {STATIC_USER.display_name}
                  </p>
                  <p className="text-[11px] text-[var(--text-faint)] mt-0.5">@{STATIC_USER.username}</p>
                  <span className={`inline-block mt-1 text-[9px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[STATIC_USER.role] || roleColors.viewer}`}>
                    {STATIC_USER.role}
                  </span>
                </div>
              </div>
            </div>

            <div className="p-1.5">
              <button
                onClick={noop}
                className="w-full flex items-center gap-2 px-3 py-2 text-sm rounded-[var(--radius-md)] text-red-400 hover:bg-red-500/10 transition-colors"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
                </svg>
                Logout
              </button>
            </div>
          </div>
        </div>
        <p className="text-[11px] text-[var(--text-muted)]">
          3 dropdown-menu implementations, two outside-click strategies, while Radix Popover is already used in 5
          places (<code>Select</code>, <code>TagInput</code>, <code>AsyncPicker</code>,{" "}
          <code>components/wiki/CollectionMultiSelect.tsx</code>, <code>app/hosts/_components/SituacaoCell.tsx</code>)
          with no shared wrapper.
        </p>
      </Specimen>

      <p className="text-xs text-[var(--text-muted)]">
        <strong>FloatingActionButton</strong> — rendered in Buttons (mobile-only).
      </p>

      <div>
        <p className="text-xs text-[var(--text-muted)] mb-1">Describe only:</p>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            <code>components/lineage/SearchOmnibar.tsx</code> — the only command palette, ⌘K from{" "}
            <code>AtlasToolbar</code>/<code>LineageToolbar</code>
          </li>
          <li><code>components/ai/AiChatDrawer.tsx</code></li>
          <li><code>components/glpi/{"{FormcreatorFormDrawer,TicketDetailDrawer}"}.tsx</code></li>
          <li><code>components/lineage/DetailDrawer.tsx</code></li>
          <li><code>components/atlas/apis/ProjectSecretsSheet.tsx</code> (<code>side=&quot;right&quot;</code>)</li>
          <li>
            <code>app/hosts/[slug]/_components/{"{ChamadoDrawer,SSHConfigDrawer,SSHOperations,IssueDrawers}"}.tsx</code>
          </li>
          <li>
            Filter drawers — <code>components/inventory/InventoryFilterDrawer.tsx</code> (generic),{" "}
            <code>app/requests/_components/RequestFilterDrawer.tsx</code>,{" "}
            <code>app/{"{hosts,dns,services,projects}"}/FilterDrawer.tsx</code> (4 near-identical)
          </li>
          <li><code>app/hosts/_components/BatchOperationShell.tsx</code></li>
          <li><code>app/secrets/_components/HistoryDrawer.tsx</code> (guardrailed)</li>
          <li>No toast; <code>window.confirm()</code> ×32</li>
        </ul>
      </div>
    </Section>
  );
}
