import type { ReactNode } from "react";

/**
 * Card header, fixed anatomy (every line always renders so cards align):
 *   title          — mono for identifiers (hostname, domain), display for names
 *   subtitle       — slug / type, mono; "–" when absent
 *   status         — the situação line (SituacaoText) or equivalent state
 *   description    — one muted line; "–" when absent
 * `corner` is the top-right slot (e.g. the quick-look button).
 */
export default function CardHeader({
  title,
  subtitle,
  status,
  description,
  corner,
  titleFont = "mono",
  subtitleFont = "mono",
}: {
  title: string;
  subtitle?: string;
  status?: ReactNode;
  description?: string;
  corner?: ReactNode;
  titleFont?: "mono" | "display";
  subtitleFont?: "mono" | "display";
}) {
  return (
    <div className="flex items-start justify-between gap-2 mb-3">
      <div className="min-w-0 flex-1">
        <h3 className={`font-semibold text-[var(--text-primary)] text-sm truncate ${titleFont === "mono" ? "font-mono" : "font-display"}`} title={title}>
          {title}
        </h3>
        <p className={`text-xs text-[var(--text-muted)] truncate mt-0.5 ${subtitle && subtitleFont === "mono" ? "font-mono" : ""}`}>{subtitle || "–"}</p>
        <div className="mt-1 min-h-4 flex items-center">{status ?? <span className="text-xs text-[var(--text-muted)]">–</span>}</div>
        <p className="text-xs text-[var(--text-muted)] mt-1 truncate" title={description || undefined}>{description || "–"}</p>
      </div>
      {corner}
    </div>
  );
}
