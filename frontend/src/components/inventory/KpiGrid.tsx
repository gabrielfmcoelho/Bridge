import SectionHeading from "@/components/ui/SectionHeading";
import StatCard from "@/components/ui/StatCard";
import { accentColor } from "@/components/ui/Card";
import Icon from "@/components/ui/Icon";

interface Kpi {
  label: string;
  value: string | number;
  color: string;
  icon: string;
  hint?: string;
  onClick?: () => void;
  active?: boolean;
}

interface KpiGridProps {
  kpis: Kpi[];
  heading?: string;
  columns?: 2 | 3 | 4 | 5;
  /** Beside the heading (e.g. the "customize" menu). */
  actions?: React.ReactNode;
  /** "list": the compact grid above a listing (Hosts "Visão geral"). */
  layout?: "grid" | "list";
}

const gridCols: Record<number, string> = {
  2: "grid-cols-2",
  3: "grid-cols-2 sm:grid-cols-3",
  4: "grid-cols-2 sm:grid-cols-4",
  5: "grid-cols-2 sm:grid-cols-3 lg:grid-cols-5",
};

const isZero = (v: string | number) => v === 0 || v === "0";

export default function KpiGrid({ kpis, heading, columns, actions, layout = "grid" }: KpiGridProps) {
  if (layout === "list") return <KpiList kpis={kpis} heading={heading} actions={actions} />;
  const cols = columns || Math.min(kpis.length, 5) as 2 | 3 | 4 | 5;
  // On a phone the KPI strip was taking 38% of the viewport to say "0" twice,
  // pushing the list — the thing the page exists for — below the fold. Tiles
  // reading zero are hidden below md, unless they all are.
  const hideZeros = kpis.some((k) => !isZero(k.value));

  return (
    <div className="mb-5">
      {heading && <SectionHeading actions={actions}>{heading}</SectionHeading>}
      <div className={`grid ${gridCols[cols] || gridCols[4]} gap-3`}>
        {kpis.map((kpi) => (
          <StatCard
            key={kpi.label}
            label={kpi.label}
            value={kpi.value}
            color={kpi.color}
            icon={kpi.icon}
            hint={kpi.hint}
            onClick={kpi.onClick}
            active={kpi.active}
            className={hideZeros && isZero(kpi.value) ? "max-md:hidden" : ""}
          />
        ))}
      </div>
    </div>
  );
}

/** Indicators beside a listing: a tight grid, value over label, hint on hover —
 *  a strip above the listing below lg, two columns in the side column on
 *  desktop (Hosts "Visão geral"). The listing is what the page is for. */
function KpiList({ kpis, heading, actions }: { kpis: Kpi[]; heading?: string; actions?: React.ReactNode }) {
  return (
    <div>
      {heading && <SectionHeading actions={actions}>{heading}</SectionHeading>}
      {/* Cell borders right/bottom, pulled 1px under the frame, so a short last row stays surface-coloured. */}
      <div className="overflow-hidden rounded-[var(--radius-lg)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]">
      <ul className="grid grid-cols-3 sm:grid-cols-5 lg:grid-cols-2 -mr-px -mb-px">
        {kpis.map((k) => {
          const zero = isZero(k.value);
          const body = (
            <>
              <span className="flex items-center gap-1.5">
                <Icon path={k.icon} className="w-3.5 h-3.5 shrink-0" style={{ color: accentColor(k.color) }} />
                <span className={`font-mono tabular-nums text-base font-semibold ${zero ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{k.value}</span>
              </span>
              <span className="block text-xs text-[var(--text-muted)] truncate mt-0.5">{k.label}</span>
            </>
          );
          const cell = "block w-full h-full px-3 py-2.5 text-left";
          return (
            <li key={k.label} className="min-w-0 border-r border-b border-[var(--border-subtle)]">
              {k.onClick ? (
                <button type="button" onClick={k.onClick} aria-pressed={!!k.active} title={k.hint}
                  className={`${cell} transition-colors duration-100 hover:bg-[var(--bg-elevated)] ${k.active ? "!bg-[var(--accent-muted)]" : ""}`}>
                  {body}
                </button>
              ) : (
                <div className={cell} title={k.hint}>{body}</div>
              )}
            </li>
          );
        })}
      </ul>
      </div>
    </div>
  );
}
