"use client";

import { useRef, type KeyboardEvent, type ReactNode } from "react";
import Icon from "./Icon";

export interface Tab {
  key: string;
  label: string;
  icon?: string;
  badge?: number;
}

/**
 * Underlined tabs for sections of one page (ADS tabs): a hairline under the
 * row, the selected tab marked by an accent underline. Not for switching how
 * a list reads — that is ToolbarSelect. Arrow keys move between tabs
 * (automatic activation); labels stay visible and the row scrolls on phones.
 */
export default function TabBar({
  tabs,
  activeTab,
  onChange,
  className = "",
  idBase,
}: {
  tabs: Tab[];
  activeTab: string;
  onChange: (key: string) => void;
  className?: string;
  /** Links each tab to its <TabPanel idBase> (aria-controls / aria-labelledby). */
  idBase?: string;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent, i: number) => {
    const last = tabs.length - 1;
    const next = { ArrowRight: i === last ? 0 : i + 1, ArrowLeft: i === 0 ? last : i - 1, Home: 0, End: last }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    refs.current[next]?.focus();
    onChange(tabs[next].key);
  };

  return (
    <div role="tablist" className={`flex gap-1 border-b border-[var(--border-subtle)] overflow-x-auto ${className}`}>
      {tabs.map((tab, i) => {
        const selected = activeTab === tab.key;
        return (
          <button
            key={tab.key}
            ref={(el) => { refs.current[i] = el; }}
            type="button"
            role="tab"
            id={idBase ? `${idBase}-tab-${tab.key}` : undefined}
            aria-controls={idBase ? `${idBase}-panel` : undefined}
            aria-selected={selected}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(tab.key)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={`relative -mb-px flex items-center gap-1.5 px-3 py-2.5 text-sm font-medium whitespace-nowrap border-b-2 transition-colors duration-100 ${
              selected
                ? "border-[var(--accent)] text-[var(--accent)]"
                : "border-transparent text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:border-[var(--border-strong)]"
            }`}
          >
            {tab.icon && <Icon path={tab.icon} className="w-3.5 h-3.5 shrink-0" />}
            {tab.label}
            {tab.badge !== undefined && tab.badge > 0 && (
              <span className="min-w-4 h-4 px-1 rounded-full bg-[var(--warning)]/20 text-[var(--warning)] text-2xs font-bold font-mono flex items-center justify-center shrink-0">
                {tab.badge}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

/** The selected tab's content, named by its tab. Wrap the whole region below
 *  the TabBar once; `activeTab` keeps the labelling in step. */
export function TabPanel({ idBase, activeTab, children, className = "" }: { idBase: string; activeTab: string; children: ReactNode; className?: string }) {
  return (
    <div role="tabpanel" id={`${idBase}-panel`} aria-labelledby={`${idBase}-tab-${activeTab}`} tabIndex={0} className={className}>
      {children}
    </div>
  );
}
