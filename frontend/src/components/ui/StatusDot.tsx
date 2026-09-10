// Coloured dot for state. `color` takes a token name (success, warning, danger,
// info, accent, cyan, purple, rose, muted) or any class via `className`.
const colors: Record<string, string> = {
  success: "bg-[var(--success)]",
  warning: "bg-[var(--warning)]",
  danger: "bg-[var(--danger)]",
  info: "bg-[var(--info)]",
  accent: "bg-[var(--accent)]",
  cyan: "bg-[var(--cyan)]",
  purple: "bg-[var(--purple)]",
  rose: "bg-[var(--rose)]",
  muted: "bg-[var(--text-faint)]",
};
const sizes = { xs: "w-1.5 h-1.5", sm: "w-2 h-2", md: "w-2.5 h-2.5" };

export default function StatusDot({
  color,
  size = "sm",
  pulse = false,
  className = "",
  title,
}: {
  color?: keyof typeof colors;
  size?: keyof typeof sizes;
  pulse?: boolean;
  className?: string;
  title?: string;
}) {
  return (
    <span
      aria-hidden={title ? undefined : true}
      title={title}
      className={`inline-block rounded-full shrink-0 ${sizes[size]} ${color ? colors[color] : ""} ${pulse ? "animate-pulse-glow" : ""} ${className}`}
    />
  );
}
