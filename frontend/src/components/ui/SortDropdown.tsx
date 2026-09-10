"use client";

import { useState } from "react";
import Button from "./Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function SortDropdown<K extends string>({
  options,
  value,
  direction,
  onChange,
}: {
  options: { key: K; label: string }[];
  value: K;
  direction: "asc" | "desc";
  onChange: (key: K, direction: "asc" | "desc") => void;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex items-center gap-1">
      <div className="relative">
        <Button size="sm" variant="secondary" onClick={() => setOpen(!open)}>
          {options.find((o) => o.key === value)?.label}
          <Icon path={ICON_PATHS.chevronDown} className="w-3 h-3 ml-0.5" />
        </Button>
        {open && (
          <>
            <div className="fixed inset-0 z-20" onClick={() => setOpen(false)} />
            <div className="absolute right-0 top-full mt-1 z-30 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-[var(--shadow-lg)] py-1 min-w-[120px]">
              {options.map((opt) => (
                <button
                  key={opt.key}
                  type="button"
                  onClick={() => {
                    onChange(opt.key, opt.key === value ? direction : "asc");
                    setOpen(false);
                  }}
                  className={`w-full text-left px-3 py-1.5 text-xs transition-colors ${
                    opt.key === value
                      ? "text-[var(--accent)]"
                      : "text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>
          </>
        )}
      </div>
      <Button size="sm" variant="secondary" onClick={() => onChange(value, direction === "asc" ? "desc" : "asc")}>
        <Icon path={ICON_PATHS.chevronUp} className={`w-3.5 h-3.5 transition-transform ${direction === "desc" ? "rotate-180" : ""}`} />
      </Button>
    </div>
  );
}
