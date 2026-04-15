const variantStyles: Record<string, string> = {
  success: "bg-emerald-500/10 border border-emerald-500/25 text-emerald-400",
  error: "bg-red-500/10 border border-red-500/25 text-red-400",
  warning: "bg-amber-500/10 border border-amber-500/25 text-amber-300",
  info: "bg-cyan-500/10 border border-cyan-500/25 text-cyan-400",
  loading: "bg-cyan-500/10 border border-cyan-500/25 text-cyan-400",
};

const icons: Record<string, React.ReactNode> = {
  success: (
    <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  error: (
    <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  warning: (
    <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
  ),
  info: (
    <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  loading: (
    <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  ),
};

export default function StatusAlert({
  variant,
  children,
  className = "",
}: {
  variant: "success" | "error" | "warning" | "info" | "loading";
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-[var(--radius-md)] p-3 text-sm animate-slide-up ${variantStyles[variant]} ${className}`}>
      <div className="flex items-center gap-2">
        {icons[variant]}
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </div>
  );
}
