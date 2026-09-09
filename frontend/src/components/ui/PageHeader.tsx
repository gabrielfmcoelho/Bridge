"use client";

import type { ReactNode } from "react";
import Button from "./Button";
import IconButton from "./IconButton";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  /** Right-hand slot (view toggles, extra buttons). Rendered before the add button. */
  actions?: ReactNode;
  /** Default add button: full label on desktop, icon-only on phones. */
  addLabel?: string;
  onAdd?: () => void;
}

export default function PageHeader({ title, subtitle, actions, addLabel, onAdd }: PageHeaderProps) {
  const hasAdd = !!(onAdd && addLabel);
  return (
    <div className={`flex ${subtitle ? "items-start" : "items-center"} justify-between gap-2 mb-6`}>
      <div className="min-w-0">
        <h1 className="text-xl sm:text-2xl font-bold font-display">{title}</h1>
        {subtitle && <p className="text-sm text-[var(--text-muted)] mt-1">{subtitle}</p>}
      </div>
      {(actions || hasAdd) && (
        <div className="flex items-center gap-1.5 shrink-0">
          {actions}
          {hasAdd && (
            <>
              <div className="hidden sm:block">
                <Button onClick={onAdd}>+ {addLabel}</Button>
              </div>
              <IconButton variant="accent" size="md" onClick={onAdd} title={addLabel} className="sm:hidden">
                <svg className="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
                </svg>
              </IconButton>
            </>
          )}
        </div>
      )}
    </div>
  );
}
