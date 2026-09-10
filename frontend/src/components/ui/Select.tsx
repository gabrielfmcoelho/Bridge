"use client";

import { useState, useRef, type SelectHTMLAttributes } from "react";
import * as Popover from "@radix-ui/react-popover";
import FormField from "./FormField";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "onChange"> {
  label?: string;
  hint?: string;
  error?: string;
  options: { value: string; label: string }[];
  onChange?: (e: { target: { value: string } }) => void;
  searchPlaceholder?: string;
}

export default function Select({
  label,
  hint,
  error,
  required,
  options,
  className = "",
  value,
  onChange,
  disabled,
  searchPlaceholder,
}: SelectProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  const selectedLabel = options.find((o) => o.value === value)?.label || "";

  const filtered = search
    ? options.filter((o) => o.label.toLowerCase().includes(search.toLowerCase()))
    : options;

  const select = (val: string) => {
    onChange?.({ target: { value: val } });
    setOpen(false);
    setSearch("");
  };

  return (
    <FormField label={label} required={required} hint={hint} error={error}>
      <Popover.Root open={open} onOpenChange={(isOpen) => { setOpen(isOpen); if (!isOpen) setSearch(""); }}>
        <Popover.Trigger asChild disabled={disabled}>
          <button
            type="button"
            disabled={disabled}
            className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border rounded-[var(--radius-md)] px-3 py-2 text-sm text-left transition duration-200 flex items-center justify-between gap-2 disabled:opacity-40 ${
              open ? "border-[var(--accent)] ring-2 ring-[var(--accent-muted)]" : error ? "border-[var(--danger)]" : "border-[var(--border-default)]"
            } ${className}`}
          >
            <span className={value ? "" : "text-[var(--text-faint)]"}>
              {selectedLabel || "--"}
            </span>
            <Icon path={ICON_PATHS.chevronDown} className={`w-3.5 h-3.5 text-[var(--text-faint)] shrink-0 transition-transform ${open ? "rotate-180" : ""}`} />
          </button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            side="bottom"
            sideOffset={4}
            align="start"
            className="z-[100] bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg overflow-hidden animate-fade-in"
            style={{ width: "var(--radix-popover-trigger-width)" }}
            onOpenAutoFocus={(e) => {
              if (options.length > 5 && inputRef.current) {
                e.preventDefault();
                inputRef.current.focus();
              }
            }}
          >
            {options.length > 5 && (
              <div className="p-1.5 border-b border-[var(--border-subtle)]">
                <input
                  ref={inputRef}
                  type="text"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={searchPlaceholder || "Search..."}
                  className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2.5 py-1.5 text-sm focus:outline-none focus:border-[var(--accent)]"
                />
              </div>
            )}
            <div className="max-h-48 overflow-y-auto">
              <button
                type="button"
                onClick={() => select("")}
                className={`w-full text-left px-3 py-1.5 text-sm transition-colors ${
                  !value ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-faint)] hover:bg-[var(--bg-elevated)]"
                }`}
              >
                --
              </button>
              {filtered.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => select(opt.value)}
                  className={`w-full text-left px-3 py-1.5 text-sm transition-colors ${
                    opt.value === value
                      ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                      : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
              {filtered.length === 0 && (
                <div className="px-3 py-2 text-xs text-[var(--text-faint)]">No results</div>
              )}
            </div>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
    </FormField>
  );
}
