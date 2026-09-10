// The one loading glyph: quarter arc over a faint ring. Colour follows `currentColor`.
const sizes = { xs: "w-3 h-3", sm: "w-4 h-4", md: "w-5 h-5", lg: "w-6 h-6" };

export default function Spinner({ size = "sm", className = "" }: { size?: keyof typeof sizes; className?: string }) {
  return (
    <svg className={`${sizes[size]} animate-spin shrink-0 ${className}`} viewBox="0 0 24 24" fill="none" aria-hidden>
      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" strokeOpacity="0.25" />
      <path d="M12 2a10 10 0 019.17 6" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}
