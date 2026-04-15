import Link from "next/link";

interface DetailHeaderProps {
  backHref: string;
  backLabel: string;
  title: string;
  titleFont?: "display" | "mono";
  titleColor?: string;
  subtitle?: string;
  description?: string;
  badges?: React.ReactNode;
  counters?: React.ReactNode;
  children?: React.ReactNode;
}

export default function DetailHeader({
  backHref,
  backLabel,
  title,
  titleFont = "display",
  titleColor,
  subtitle,
  description,
  badges,
  counters,
  children,
}: DetailHeaderProps) {
  const fontVar = titleFont === "mono" ? "var(--font-mono)" : "var(--font-display)";

  return (
    <div className="mb-6">
      <Link
        href={backHref}
        className="inline-flex items-center gap-1.5 text-sm text-[var(--text-muted)] hover:text-[var(--accent)] transition-colors mb-4"
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        {backLabel}
      </Link>

      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h1
            className="text-2xl font-bold truncate"
            style={{ fontFamily: fontVar, color: titleColor || "var(--text-primary)" }}
          >
            {title}
          </h1>
          {subtitle && (
            <p className="text-sm text-[var(--text-muted)] mt-0.5">{subtitle}</p>
          )}
          {description && (
            <p className="text-sm text-[var(--text-secondary)] mt-1 line-clamp-2">{description}</p>
          )}
          {(badges || counters) && (
            <div className="flex flex-wrap items-center gap-2 mt-2">
              {badges}
              {counters}
            </div>
          )}
        </div>
        {children}
      </div>
    </div>
  );
}
