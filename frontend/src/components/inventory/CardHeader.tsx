import type { ReactNode } from "react";

/**
 * 3-line card header: title (mono), subtitle (mono/faint), description (muted) + badge.
 * Establishes a consistent visual hierarchy across all inventory cards.
 */
export default function CardHeader({
  title,
  subtitle,
  description,
  badge,
}: {
  title: string;
  subtitle?: string;
  description?: string;
  badge: ReactNode;
}) {
  return (
    <div className="flex items-start justify-between mb-3">
      <div className="min-w-0 flex-1">
        <h3
          className="font-semibold text-[var(--text-primary)] text-sm truncate"
          style={{ fontFamily: "var(--font-mono)" }}
        >
          {title}
        </h3>
        {subtitle && (
          <p
            className="text-xs text-[var(--text-faint)] truncate mt-0.5"
            style={{ fontFamily: "var(--font-mono)" }}
          >
            {subtitle}
          </p>
        )}
        <p className="text-xs text-[var(--text-muted)] mt-0.5 truncate">
          {description || "-"}
        </p>
      </div>
      {badge}
    </div>
  );
}
