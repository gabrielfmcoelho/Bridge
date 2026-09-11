"use client";

import { useEffect, useRef, useState } from "react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  debounceMs?: number;
}

// WikiSearchBar is a debounced input whose value is pushed up only after the
// user stops typing. Keeps the query endpoint from being hit on every keystroke.
export default function WikiSearchBar({ value, onChange, placeholder, debounceMs = 300 }: Props) {
  const { t } = useLocale();
  const [local, setLocal] = useState(value);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Keep local state in sync if parent clears externally (e.g. after navigation).
  useEffect(() => {
    setLocal(value);
  }, [value]);

  useEffect(() => {
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (local !== value) onChange(local);
    }, debounceMs);
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [local, debounceMs]);

  return (
    <div className="relative">
      <Icon path={ICON_PATHS.search} className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-[var(--text-muted)]" />
      <input
        type="text"
        value={local}
        onChange={(e) => setLocal(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            setLocal("");
            onChange("");
          }
        }}
        placeholder={placeholder ?? t("wiki.searchWikiPlaceholder")}
        className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-8 pr-3 py-2 text-sm text-[var(--text-primary)] placeholder-[var(--text-faint)] focus:outline-none focus:border-[var(--accent)]"
      />
      {local && (
        <button
          type="button"
          onClick={() => { setLocal(""); onChange(""); }}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)] text-sm"
          aria-label={t("wiki.clearSearch")}
        >
          ×
        </button>
      )}
    </div>
  );
}
