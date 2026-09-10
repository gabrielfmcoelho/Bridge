import Link from "next/link";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
        <Icon path={ICON_PATHS.back} />
        {backLabel}
      </Link>

      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <h1
            className="text-xl sm:text-2xl font-bold truncate"
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
