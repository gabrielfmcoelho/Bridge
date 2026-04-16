import StatCard from "@/components/ui/StatCard";

interface Kpi {
  label: string;
  value: string | number;
  color: string;
  icon: string;
}

interface KpiGridProps {
  kpis: Kpi[];
  heading?: string;
  columns?: 2 | 3 | 4 | 5;
}

const gridCols: Record<number, string> = {
  2: "grid-cols-2",
  3: "grid-cols-2 sm:grid-cols-3",
  4: "grid-cols-2 sm:grid-cols-4",
  5: "grid-cols-2 sm:grid-cols-3 lg:grid-cols-5",
};

export default function KpiGrid({ kpis, heading, columns }: KpiGridProps) {
  const cols = columns || Math.min(kpis.length, 5) as 2 | 3 | 4 | 5;

  return (
    <div className="mb-5">
      {heading && (
        <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
          {heading}
        </h2>
      )}
      <div className={`grid ${gridCols[cols] || gridCols[4]} gap-3`}>
        {kpis.map((kpi) => (
          <StatCard key={kpi.label} label={kpi.label} value={kpi.value} color={kpi.color} icon={kpi.icon} />
        ))}
      </div>
    </div>
  );
}
