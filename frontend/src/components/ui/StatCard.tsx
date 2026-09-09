// Hue keys are what the KPI grids pass today; each resolves to a theme-aware token.
// Full literals on purpose (Tailwind only generates classes it can read verbatim).
const colors: Record<string, { text: string; border: string; gradient: string }> = {
  cyan: { text: "text-[var(--cyan)]", border: "border-[var(--cyan)]/20", gradient: "from-[var(--cyan)]/10 to-transparent" },
  emerald: { text: "text-[var(--success)]", border: "border-[var(--success)]/20", gradient: "from-[var(--success)]/10 to-transparent" },
  purple: { text: "text-[var(--purple)]", border: "border-[var(--purple)]/20", gradient: "from-[var(--purple)]/10 to-transparent" },
  amber: { text: "text-[var(--warning)]", border: "border-[var(--warning)]/20", gradient: "from-[var(--warning)]/10 to-transparent" },
  red: { text: "text-[var(--danger)]", border: "border-[var(--danger)]/20", gradient: "from-[var(--danger)]/10 to-transparent" },
  sky: { text: "text-[var(--info)]", border: "border-[var(--info)]/20", gradient: "from-[var(--info)]/10 to-transparent" },
  rose: { text: "text-[var(--rose)]", border: "border-[var(--rose)]/20", gradient: "from-[var(--rose)]/10 to-transparent" },
};

interface StatCardProps {
  label: string;
  value: string | number;
  icon: string;
  color: string;
  className?: string;
}

export default function StatCard({ label, value, icon, color, className = "" }: StatCardProps) {
  const c = colors[color] ?? colors.cyan;
  return (
    <div
      className={`relative overflow-hidden bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border ${c.border} p-4 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)] ${className}`}
    >
      <div className={`absolute inset-0 bg-gradient-to-br ${c.gradient} pointer-events-none`} />
      <svg className={`absolute right-2.5 top-2.5 w-8 h-8 ${c.text} opacity-[0.08]`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}>
        <path strokeLinecap="round" strokeLinejoin="round" d={icon} />
      </svg>
      <div className="relative">
        <p className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider font-medium">{label}</p>
        <p className={`text-2xl font-bold mt-0.5 ${c.text}`} style={{ fontFamily: "var(--font-display)" }}>{value}</p>
      </div>
    </div>
  );
}
