interface CardProps {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  onClick?: () => void;
  accent?: "cyan" | "emerald" | "purple" | "amber" | "red" | "none";
  hover?: boolean;
  clickIndicator?: "link" | "drawer";
}

const accentBorders: Record<string, string> = {
  cyan: "border-l-cyan-500",
  emerald: "border-l-emerald-500",
  purple: "border-l-purple-500",
  amber: "border-l-amber-500",
  red: "border-l-red-500",
  none: "",
};

const indicatorIcons: Record<string, string> = {
  link: "M9 5l7 7-7 7",
  drawer: "M4 6h16M4 12h16M4 18h7",
};

export default function Card({
  children,
  className = "",
  style,
  onClick,
  accent = "none",
  hover = true,
  clickIndicator,
}: CardProps) {
  const accentClass = accent !== "none" ? `border-l-[3px] ${accentBorders[accent]}` : "";

  return (
    <div
      className={`
        relative
        bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] p-5
        transition-all duration-200 ease-out
        ${hover ? "hover:border-[var(--border-default)] hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)]" : ""}
        ${onClick ? "cursor-pointer" : ""}
        ${clickIndicator ? "group/card" : ""}
        ${accentClass}
        ${className}
      `}
      style={style}
      onClick={onClick}
    >
      {clickIndicator && (
        <svg
          className="absolute bottom-2 right-2.5 w-3.5 h-3.5 text-[var(--text-faint)] opacity-80 group-hover/card:opacity-100 transition-opacity shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d={indicatorIcons[clickIndicator]} />
        </svg>
      )}
      {children}
    </div>
  );
}
