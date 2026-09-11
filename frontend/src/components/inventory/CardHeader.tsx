import type { ReactNode } from "react";

/**
 * 3-line card header: title, subtitle (always mono — it is a slug or domain),
 * description + badge. `titleFont` follows the rule: mono when the title is a
 * machine identifier (hostname, domain), display when it is prose (a project
 * name).
 */
export default function CardHeader({
  title,
  subtitle,
  description,
  badge,
  titleFont = "mono",
}: {
  title: string;
  subtitle?: string;
  description?: string;
  badge: ReactNode;
  titleFont?: "mono" | "display";
}) {
  return (
    <div className="flex items-start justify-between mb-3">
      <div className="min-w-0 flex-1">
        <h3
          className={`font-semibold text-[var(--text-primary)] text-sm truncate ${titleFont === "mono" ? "font-mono" : "font-display"}`}
        >
          {title}
        </h3>
        {subtitle && (
          <p
            className="text-xs text-[var(--text-faint)] truncate mt-0.5 font-mono"
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
