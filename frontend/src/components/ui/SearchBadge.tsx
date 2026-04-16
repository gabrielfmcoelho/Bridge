"use client";

import { useLocale } from "@/contexts/LocaleContext";

interface SearchBadgeProps {
  search: string;
  onClear: () => void;
}

export default function SearchBadge({ search, onClear }: SearchBadgeProps) {
  const { t } = useLocale();

  if (!search) return null;

  return (
    <div className="flex items-center gap-2 mb-4">
      <span className="text-xs text-[var(--text-muted)]">{t("common.search")}:</span>
      <span className="text-xs text-[var(--text-primary)] font-medium">&ldquo;{search}&rdquo;</span>
      <button
        onClick={onClear}
        className="text-xs text-[var(--text-faint)] hover:text-[var(--text-secondary)]"
      >
        &times;
      </button>
    </div>
  );
}
