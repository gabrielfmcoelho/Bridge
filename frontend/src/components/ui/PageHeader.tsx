"use client";

import type { ReactNode } from "react";
import Button from "./Button";
import IconButton from "./IconButton";
import Icon from "./Icon";
import Divider from "./Divider";
import TabBar, { TabPanel, type Tab } from "./TabBar";
import SectionNav, { type SectionNavGroup } from "./SectionNav";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useLocalStorage } from "@/hooks/useLocalStorage";

export interface PageHeaderTabs {
  idBase: string;
  /** Accessible name of the tab set (e.g. the page title). */
  label: string;
  active: string;
  onChange: (key: string) => void;
  /** "top": underlined tabs under the header. "side": grouped list beside the content (Configurações). */
  variant?: "top" | "side";
  items: (Tab & { group?: string })[];
  /** Extra classes for the panel (full-bleed pages: "flex-1 min-h-0"). */
  panelClassName?: string;
}

interface PageHeaderProps {
  id?: string;
  title: string;
  /** mono when the title is an identifier (domain, slug), display for names. */
  titleFont?: "display" | "mono";
  subtitle?: string;
  subtitleFont?: "display" | "mono";
  /** The state line (SituacaoText, a request status). */
  status?: ReactNode;
  /** Counts and flags beside the state (CardIndicator row). */
  indicators?: ReactNode;
  description?: string;
  /** Detail pages: show a muted "Sem descrição" when there is none, so row 4 never disappears. */
  showEmptyDescription?: boolean;
  /** Entity extras shown before the CRUD buttons (e.g. SSH config). */
  actions?: ReactNode;
  addLabel?: string;
  onAdd?: () => void;
  /** Pages with a phone FAB for adding hide the header's copy there. */
  hideAddOnPhone?: boolean;
  onEdit?: () => void;
  /** Asks for confirmation (useConfirm) before calling it. */
  onDelete?: () => void;
  deleteConfirmMessage?: string;
  /** Search, filters, group, view, export, bulk actions, customize. */
  controls?: ReactNode;
  /** Makes the controls row hidable; the choice is remembered under this key. */
  controlsKey?: string;
  /** Shown on the toggle while the controls are hidden (e.g. active filters). */
  controlsBadge?: number;
  tabs?: PageHeaderTabs;
  /** The tab content when `tabs` is set. */
  children?: ReactNode;
}

/**
 * The one page header. Fixed anatomy, top to bottom — the user always finds
 * the same thing in the same place:
 *   1. title                      · actions · edit · delete · add · tools toggle
 *   2. subtitle / slug
 *   3. status  |  indicators
 *   4. description
 *   5. controls (hidable)
 *   6. bottom edge: a divider, or tabs (top) / a side list (side) + their panel
 * Breadcrumbs and "Voltar" live in the app header above it.
 */
export default function PageHeader(props: PageHeaderProps) {
  const {
    id, title, titleFont = "display", subtitle, subtitleFont = "display", status, indicators, description, showEmptyDescription = false,
    actions, addLabel, onAdd, hideAddOnPhone = false, onEdit, onDelete, deleteConfirmMessage,
    controls, controlsKey, controlsBadge, tabs, children,
  } = props;
  const { t } = useLocale();
  const confirm = useConfirm();
  const [controlsOpen, setControlsOpen] = useLocalStorage<boolean>(`ph_controls:${controlsKey ?? "_"}`, true);
  const showControls = !!controls && (!controlsKey || controlsOpen);
  const controlsId = `${controlsKey ?? "page"}-controls`;

  const del = onDelete && (async () => {
    if (await confirm({ title: t("confirm.deleteGeneric"), message: deleteConfirmMessage, danger: true, confirmLabel: t("common.delete") })) onDelete();
  });
  const crud = (compact: boolean) => (
    <>
      {actions}
      {onEdit && (
        <IconButton variant="outline" onClick={onEdit} label={t("common.edit")}>
          <Icon path={ICON_PATHS.edit} />
        </IconButton>
      )}
      {del && (
        <IconButton variant="outline" onClick={del} label={t("common.delete")} className="hover:text-[var(--danger)]">
          <Icon path={ICON_PATHS.trash} className="text-[var(--danger)]" />
        </IconButton>
      )}
      {onAdd && addLabel && (compact ? (
        !hideAddOnPhone && (
          <IconButton variant="accent" onClick={onAdd} label={addLabel}>
            <Icon path={ICON_PATHS.plus} />
          </IconButton>
        )
      ) : (
        <Button size="sm" onClick={onAdd}>
          <Icon path={ICON_PATHS.plus} className="w-4 h-4" />
          {addLabel}
        </Button>
      ))}
    </>
  );
  const hasCrud = !!(actions || onEdit || onDelete || (onAdd && addLabel));

  const sideGroups: SectionNavGroup<string>[] = [];
  if (tabs?.variant === "side") {
    for (const item of tabs.items) {
      const g = item.group ?? "";
      let group = sideGroups.find((x) => x.title === g);
      if (!group) sideGroups.push((group = { title: g, items: [] }));
      group.items.push({ key: item.key, label: item.label });
    }
  }

  return (
    <>
      <div className="mb-6 space-y-3">
        {/* 1 · title and the tools that act on the page's subject */}
        <div className="flex items-start justify-between gap-3">
          <h1
            id={id}
            className={`min-w-0 text-heading-sm font-bold break-words text-[var(--text-primary)] ${titleFont === "mono" ? "font-mono" : "font-display"}`}
          >
            {title}
          </h1>
          {(hasCrud || (controls && controlsKey)) && (
            <div className="flex items-center gap-1.5 shrink-0">
              {hasCrud && <div className="max-sm:hidden flex items-center gap-1.5">{crud(false)}</div>}
              {controls && controlsKey && (
                <span className="relative">
                  <IconButton
                    variant={controlsOpen ? "active" : "outline"}
                    onClick={() => setControlsOpen(!controlsOpen)}
                    label={controlsOpen ? t("common.hideTools") : t("common.showTools")}
                    aria-expanded={controlsOpen}
                    aria-controls={controlsId}
                  >
                    <Icon path={ICON_PATHS.filter} />
                  </IconButton>
                  {!controlsOpen && !!controlsBadge && (
                    <span aria-hidden className="absolute -top-1.5 -right-1.5 min-w-4 h-4 px-1 rounded-full bg-[var(--accent)] text-[var(--bg-base)] text-2xs font-bold flex items-center justify-center">
                      {controlsBadge}
                    </span>
                  )}
                </span>
              )}
            </div>
          )}
        </div>

        {/* 2 · subtitle / slug */}
        {subtitle && <p className={`text-sm text-[var(--text-muted)] -mt-2 ${subtitleFont === "mono" ? "font-mono" : ""}`}>{subtitle}</p>}

        {/* 3 · state and indicators */}
        {(status || indicators) && (
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
            {status}
            {status && indicators && <Divider vertical className="h-4 self-center" />}
            {indicators && <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">{indicators}</div>}
          </div>
        )}

        {/* 4 · description */}
        {description ? (
          <p className="text-sm text-[var(--text-secondary)] line-clamp-2">{description}</p>
        ) : (
          showEmptyDescription && <p className="text-sm text-[var(--text-muted)]">{t("common.noDescription")}</p>
        )}

        {/* CRUD on phones: its own row, icons only */}
        {hasCrud && <div className="sm:hidden flex items-center gap-1.5">{crud(true)}</div>}

        {/* 5 · controls (hidable) */}
        {showControls && <div id={controlsId}>{controls}</div>}

        {/* 6 · bottom edge */}
        {tabs && tabs.variant !== "side" ? (
          <TabBar tabs={tabs.items} activeTab={tabs.active} onChange={tabs.onChange} idBase={tabs.idBase} />
        ) : (
          <Divider />
        )}
      </div>

      {tabs && tabs.variant !== "side" && (
        <TabPanel idBase={tabs.idBase} activeTab={tabs.active} className={tabs.panelClassName}>
          {children}
        </TabPanel>
      )}
      {tabs?.variant === "side" && (
        <div className="md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:gap-8 max-md:space-y-4">
          <aside className="md:sticky md:top-0 md:self-start">
            <SectionNav label={tabs.label} groups={sideGroups} value={tabs.active} onChange={tabs.onChange} />
          </aside>
          <TabPanel idBase={tabs.idBase} activeTab={tabs.active} className={tabs.panelClassName}>
            {children}
          </TabPanel>
        </div>
      )}
      {!tabs && children}
    </>
  );
}
