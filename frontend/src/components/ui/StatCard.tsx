const colors: Record<string, { text: string; border: string; gradient: string }> = {
  cyan: { text: "text-cyan-400", border: "border-cyan-500/20", gradient: "from-cyan-500/10 to-transparent" },
  emerald: { text: "text-emerald-400", border: "border-emerald-500/20", gradient: "from-emerald-500/10 to-transparent" },
  purple: { text: "text-purple-400", border: "border-purple-500/20", gradient: "from-purple-500/10 to-transparent" },
  amber: { text: "text-amber-400", border: "border-amber-500/20", gradient: "from-amber-500/10 to-transparent" },
  red: { text: "text-red-400", border: "border-red-500/20", gradient: "from-red-500/10 to-transparent" },
  sky: { text: "text-sky-400", border: "border-sky-500/20", gradient: "from-sky-500/10 to-transparent" },
  rose: { text: "text-rose-400", border: "border-rose-500/20", gradient: "from-rose-500/10 to-transparent" },
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
