import SectionHeading from "@/components/ui/SectionHeading";
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

const isZero = (v: string | number) => v === 0 || v === "0";

export default function KpiGrid({ kpis, heading, columns }: KpiGridProps) {
  const cols = columns || Math.min(kpis.length, 5) as 2 | 3 | 4 | 5;
  // On a phone the KPI strip was taking 38% of the viewport to say "0" twice,
  // pushing the list — the thing the page exists for — below the fold. Tiles
  // reading zero are hidden below md, unless they all are.
  const hideZeros = kpis.some((k) => !isZero(k.value));

  return (
    <div className="mb-5">
      {heading && <SectionHeading>{heading}</SectionHeading>}
      <div className={`grid ${gridCols[cols] || gridCols[4]} gap-3`}>
        {kpis.map((kpi) => (
          <StatCard
            key={kpi.label}
            label={kpi.label}
            value={kpi.value}
            color={kpi.color}
            icon={kpi.icon}
            className={hideZeros && isZero(kpi.value) ? "max-md:hidden" : ""}
          />
        ))}
      </div>
    </div>
  );
}
