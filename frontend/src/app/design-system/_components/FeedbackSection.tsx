"use client";

import { Section, Specimen } from "./Section";
import StatusAlert from "@/components/ui/StatusAlert";
import OperationOutput from "@/components/ui/OperationOutput";
import { Skeleton, SkeletonCard, SkeletonTable, SkeletonStats } from "@/components/ui/Skeleton";
import EmptyState from "@/components/ui/EmptyState";
import Badge from "@/components/ui/Badge";
import Spinner from "@/components/ui/Spinner";
import StatusDot from "@/components/ui/StatusDot";
import Avatar from "@/components/ui/Avatar";
import Button from "@/components/ui/Button";
import LayerBadge from "@/components/atlas/shared/LayerBadge";
import RoleBadge from "@/components/atlas/shared/RoleBadge";
import AlertBadge from "@/app/hosts/_components/AlertBadge";
import { UsageBar, ResourceCard } from "@/app/hosts/[slug]/_components/UsageBar";
import { TABLE_LAYERS } from "@/lib/lineage/indexes";
import { ICON_PATHS } from "@/lib/icon-paths";

const STATUS_VARIANTS = ["success", "error", "warning", "info", "loading"] as const;
const BADGE_COLORS = ["emerald", "cyan", "amber", "accent", "red", "rose", "gray"] as const;
const EMPTY_STATE_ICONS = ["server", "globe", "folder", "box", "search", "key", "topology"] as const;
const ROLES = ["admin", "editor", "viewer"] as const;

// roleColors copied verbatim from app/settings/page.tsx:30 (duplicated again at
// components/layout/Header.tsx:13; same object, two separate consts).
const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-purple-500/10 text-purple-400/70 border-purple-500/15",
  viewer: "bg-[var(--bg-overlay)] text-[var(--text-faint)] border-[var(--border-subtle)]",
};

export default function FeedbackSection() {
  return (
    <Section id="feedback" title="Feedback">
      <Specimen title="StatusAlert" source="components/ui/StatusAlert.tsx" wide>
        <div className="space-y-2">
          {STATUS_VARIANTS.map((variant) => (
            <StatusAlert key={variant} variant={variant}>
              {variant} message
            </StatusAlert>
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          Reads <code>--success</code>/<code>--danger</code>/<code>--warning</code>/<code>--info</code> with opacity
          modifiers, so it follows the theme. (Before step 1 it used the raw palette and washed out in light mode.)
        </p>
      </Specimen>

      <p className="text-xs text-[var(--text-muted)]">
        <code>FormError</code> rendered in Inputs &amp; Forms; overlaps <code>StatusAlert variant=&quot;error&quot;</code>.
      </p>

      <Specimen title="OperationOutput" source="components/ui/OperationOutput.tsx">
        <OperationOutput data={{ message: "Scan complete", output: "eth0: 10.0.0.4\nlo: 127.0.0.1" }} />
        <OperationOutput data={{ error: "Permission denied (publickey)." }} />
      </Specimen>

      <Specimen title="Skeleton" source="components/ui/Skeleton.tsx" wide>
        <div className="flex flex-wrap items-start gap-3">
          <Skeleton className="h-4 w-40" />
          <SkeletonCard />
        </div>
        <div className="mt-3">
          <SkeletonTable rows={3} />
        </div>
        <div className="mt-3">
          <SkeletonStats />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>.skeleton</code> class in <code>globals.css</code>. The loading glyph is <code>Spinner</code> now
          (Button, StatusAlert, InventoryContent, PasswordField all use it):{" "}
          <span className="inline-flex items-center gap-2 align-middle text-[var(--text-secondary)]">
            <Spinner size="xs" /> <Spinner /> <Spinner size="md" /> <Spinner size="lg" />
          </span>
        </p>
      </Specimen>

      <Specimen title="EmptyState" source="components/ui/EmptyState.tsx" wide>
        <EmptyState
          icon="server"
          title="No hosts yet"
          description="Add your first host to get started."
          action={<Button size="sm">Add host</Button>}
        />
      </Specimen>

      <Specimen title="EmptyState (compact)" source="components/ui/EmptyState.tsx" wide>
        <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-2">
          {EMPTY_STATE_ICONS.map((icon) => (
            <EmptyState key={icon} icon={icon} title={icon} compact />
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">Private 7-entry icon map (a 3rd icon registry).</p>
      </Specimen>

      <Specimen title="Badge" source="components/ui/Badge.tsx" wide>
        <div className="flex flex-wrap gap-2">
          <Badge>default</Badge>
          {BADGE_COLORS.map((color) => (
            <Badge key={color} color={color}>
              {color}
            </Badge>
          ))}
          <Badge dot>dot</Badge>
        </div>
        <div className="flex flex-wrap gap-2 mt-2">
          <Badge variant="situacao" situacao="active">
            active
          </Badge>
          <Badge variant="situacao" situacao="maintenance">
            maintenance
          </Badge>
          <Badge variant="situacao" situacao="inactive">
            inactive
          </Badge>
          <Badge variant="situacao" situacao="active" compact>
            Active
          </Badge>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          <code>situacao</code> mode runs <code>useQuery([&quot;enums&quot;,&quot;situacao&quot;])</code> inside the
          badge; a network call per badge instance (deduped by key); falls back to <code>SITUACAO_COLORS</code> on
          error.
        </p>
      </Specimen>

      <Specimen title="StatusDot" source="components/ui/StatusDot.tsx">
        {(["success", "warning", "danger", "info", "accent", "cyan", "accent", "rose", "muted"] as const).map((c) => (
          <span key={c} className="inline-flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
            <StatusDot color={c} />
            {c}
          </span>
        ))}
        <span className="inline-flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
          <StatusDot color="success" pulse /> pulse
        </span>
        <span className="inline-flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
          <StatusDot size="xs" color="warning" /> xs <StatusDot color="warning" /> sm <StatusDot size="md" color="warning" /> md
        </span>
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">
          21 literal dots in 9 files swept in; the ones built from a class expression (Badge, AlertBadge, LayerBadge,
          AsyncPicker, TicketList) still draw their own span.
        </p>
      </Specimen>

      <Specimen title="LayerBadge" source="components/atlas/shared/LayerBadge.tsx" wide>
        <div className="flex flex-wrap gap-1.5">
          {TABLE_LAYERS.map((layer) => (
            <LayerBadge key={layer} layer={layer} size="md" />
          ))}
        </div>
        <div className="flex flex-wrap gap-1.5 mt-2">
          {TABLE_LAYERS.map((layer) => (
            <LayerBadge key={`sm-${layer}`} layer={layer} size="sm" />
          ))}
        </div>
        <div className="flex flex-wrap gap-1.5 mt-2">
          {TABLE_LAYERS.map((layer) => (
            <LayerBadge key={`nodot-${layer}`} layer={layer} dot={false} />
          ))}
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          5 badge implementations total: <code>Badge</code>, <code>LayerBadge</code>, <code>RoleBadge</code>,{" "}
          <code>AlertBadge</code>, and ad-hoc role/entidade chips.
        </p>
      </Specimen>

      <Specimen title="RoleBadge" source="components/atlas/shared/RoleBadge.tsx">
        <RoleBadge role="source" size="sm" />
        <RoleBadge role="source" size="md" />
        <RoleBadge role="built" size="sm" />
        <RoleBadge role="built" size="md" />
      </Specimen>

      <Specimen title="AlertBadge" source="app/hosts/_components/AlertBadge.tsx">
        <AlertBadge alerts={[{ type: "cpu", level: "critical", message: "CPU 97%", source: "auto" }]} />
        <AlertBadge alerts={[{ type: "disk", level: "warning", message: "Disk 85%", source: "auto" }]} />
        <AlertBadge alerts={[{ type: "info", level: "info", message: "Reboot pending", source: "manual" }]} />
      </Specimen>

      <Specimen title="UsageBar / ResourceCard" source="app/hosts/[slug]/_components/UsageBar.tsx" wide>
        <div className="space-y-3 max-w-sm">
          <UsageBar label="Disk" total="100G" used="32G" percent="32%" />
          <UsageBar label="Disk" total="100G" used="55G" percent="55%" />
          <UsageBar label="Disk" total="100G" used="85G" percent="85%" />
        </div>
        <div className="mt-3 max-w-xs">
          <ResourceCard label="Kernel" value="6.8.0" icon={ICON_PATHS.terminal} />
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-2">The only progress bar in the app; not in <code>ui/</code>.</p>
      </Specimen>

      <Specimen title="Role chip" source="app/settings/page.tsx:30,513" wide>
        <div className="space-y-2">
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1">app/settings/page.tsx:513 (table row)</p>
            <div className="flex flex-wrap gap-1.5">
              {/* specimen: app/settings/page.tsx:513 */}
              {ROLES.map((r) => (
                <span
                  key={r}
                  className={`text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[r] || roleColors.viewer}`}
                >
                  {r}
                </span>
              ))}
            </div>
          </div>
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1">components/layout/Header.tsx:171 (avatar dropdown)</p>
            <div className="flex flex-wrap gap-1.5">
              {/* specimen: components/layout/Header.tsx:171 */}
              {ROLES.map((r) => (
                <span
                  key={r}
                  className={`inline-block mt-1 text-[9px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[r] || roleColors.viewer}`}
                >
                  {r}
                </span>
              ))}
            </div>
          </div>
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1">app/settings/RoleMappingsTab.tsx:96 (mapping row, inline ternary)</p>
            <div className="flex flex-wrap gap-1.5">
              {/* specimen: app/settings/RoleMappingsTab.tsx:96-104 */}
              {ROLES.map((r) => (
                <span
                  key={r}
                  className={`inline-block px-2 py-0.5 rounded text-[11px] font-medium border ${
                    r === "admin"
                      ? "bg-cyan-500/15 text-cyan-400 border-cyan-500/25"
                      : r === "editor"
                      ? "bg-purple-500/15 text-purple-400 border-purple-500/25"
                      : "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]"
                  }`}
                >
                  {r}
                </span>
              ))}
            </div>
          </div>
          <div>
            <p className="text-[11px] text-[var(--text-faint)] mb-1">components/layout/Header.tsx:219 (mobile drawer)</p>
            <div className="flex flex-wrap gap-1.5">
              {/* specimen: components/layout/Header.tsx:219 */}
              {ROLES.map((r) => (
                <span
                  key={r}
                  className={`inline-block mt-1 text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[r] || roleColors.viewer}`}
                >
                  {r}
                </span>
              ))}
            </div>
          </div>
          <p className="text-[11px] text-[var(--text-muted)]">
            The mobile-drawer chip (<code>Header.tsx:219</code>) is byte-identical to the avatar-dropdown chip
            above except <code>text-[10px]</code> instead of <code>text-[9px]</code>; one pixel of type size
            apart.
          </p>
        </div>
      </Specimen>

      <Specimen title="Entidade chip" source="app/settings/page.tsx:520">
        {/* specimen: app/settings/page.tsx:520 */}
        <span
          className="px-1.5 py-0.5 rounded border text-[10px] border-[var(--accent)]/40 text-[var(--accent)] bg-[var(--accent-muted)]"
          title="Primary"
        >
          SEAD-PI
        </span>
        <span className="px-1.5 py-0.5 rounded border text-[10px] border-[var(--border-subtle)] text-[var(--text-muted)]">
          DGTI
        </span>
      </Specimen>

      <Specimen title="Avatar" source="components/ui/Avatar.tsx" alsoIn={["components/layout/Header.tsx:153, :200, :214 (in flight)"]}>
        <Avatar name="Ana Souza" />
        <Avatar name="Ana Souza" size="md" />
        <Avatar name="Ana Souza" size="lg" />
        <p className="text-xs text-[var(--text-muted)] w-full mt-1">
          Initials only, sized to the row. The two settings sites (table cell, user card) use it; Header keeps its copies
          until that file lands.
        </p>
      </Specimen>

      <div className="space-y-1">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">Worklist</h3>
        <ul className="text-xs text-[var(--text-muted)] list-disc pl-5 space-y-1">
          <li>
            Ad-hoc pills in <code>app/issues/IssueBoard.tsx</code>,{" "}
            <code>app/hosts/[slug]/_components/IssueViews.tsx</code>,{" "}
            <code>app/projects/[id]/_components/IssueBoard.tsx</code>, <code>app/catalog/_components/OfferingCard.tsx</code>.
          </li>
          <li>
            <code>window.confirm()</code> ×32 for destructive confirms; no toast/snackbar anywhere.
          </li>
        </ul>
      </div>
    </Section>
  );
}
