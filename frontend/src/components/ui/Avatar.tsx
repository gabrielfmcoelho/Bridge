// Initials avatar. The app has no user photos; the first letter of the display
// name is the identity, sized to the row it sits in.
const sizes = { sm: "w-8 h-8 text-xs", md: "w-9 h-9 text-sm", lg: "w-12 h-12 text-base" };

export default function Avatar({ name, size = "sm", className = "" }: { name: string; size?: keyof typeof sizes; className?: string }) {
  return (
    <span
      aria-hidden
      className={`inline-flex items-center justify-center rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] font-semibold text-[var(--text-secondary)] shrink-0 ${sizes[size]} ${className}`}
    >
      {(name || "?").charAt(0).toUpperCase()}
    </span>
  );
}
