import SectionHeading from "@/components/ui/SectionHeading";
import StatCard from "@/components/ui/StatCard";
import { accentColor } from "@/components/ui/Card";
import SectionCard from "@/components/ui/SectionCard";
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
  /** "list": compact rows for a narrow side column (Hosts "Visão geral"). */
  layout?: "grid" | "list";
  /** One line under the heading (list layout). */
  description?: string;
}

const gridCols: Record<number, string> = {
  2: "grid-cols-2",
  3: "grid-cols-2 sm:grid-cols-3",
  4: "grid-cols-2 sm:grid-cols-4",
  5: "grid-cols-2 sm:grid-cols-3 lg:grid-cols-5",
};

const isZero = (v: string | number) => v === 0 || v === "0";

export default function KpiGrid({ kpis, heading, columns, actions, layout = "grid", description }: KpiGridProps) {
  if (layout === "list") return <KpiList kpis={kpis} heading={heading} description={description} actions={actions} />;
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

/** The statistics section as rows (icon, label, hint, value) in a
 *  SectionCard. A row that maps to a filter is a toggle button
 *  (aria-pressed); zero rows stay, faint (fixed anatomy). */
function KpiList({ kpis, heading, description, actions }: { kpis: Kpi[]; heading?: string; description?: string; actions?: React.ReactNode }) {
  return (
    <SectionCard title={heading ?? ""} description={description} controls={actions} body="flush">
      <ul className="divide-y divide-[var(--border-subtle)]">
        {kpis.map((k) => {
          const zero = isZero(k.value);
          const body = (
            <>
              <Icon path={k.icon} className="w-4 h-4 shrink-0" style={{ color: accentColor(k.color) }} />
              <span className="min-w-0 flex-1">
                <span className="block text-sm text-[var(--text-secondary)] truncate">{k.label}</span>
                {k.hint && <span className="block text-xs text-[var(--text-muted)] truncate mt-0.5">{k.hint}</span>}
              </span>
              <span className={`font-mono tabular-nums text-lg font-semibold ${zero ? "text-[var(--text-muted)]" : "text-[var(--text-primary)]"}`}>{k.value}</span>
            </>
          );
          const row = "flex w-full items-center gap-4 px-5 py-3.5 text-left";
          return (
            <li key={k.label}>
              {k.onClick ? (
                <button
                  type="button"
                  onClick={k.onClick}
                  aria-pressed={!!k.active}
                  className={`${row} transition-colors duration-100 hover:bg-[var(--bg-elevated)] ${k.active ? "bg-[var(--accent-muted)]" : ""}`}
                >
                  {body}
                </button>
              ) : (
                <div className={row}>{body}</div>
              )}
            </li>
          );
        })}
      </ul>
    </SectionCard>
  );
}
