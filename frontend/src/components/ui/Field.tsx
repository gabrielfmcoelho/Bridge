export default function Field({
  label,
  value,
  mono,
  link,
  href,
  className = "",
}: {
  label: string;
  value: string;
  mono?: boolean;
  link?: boolean;
  href?: string;
  className?: string;
}) {
  const textStyle = mono ? { fontFamily: "var(--font-mono)" } : undefined;
  const displayValue = value || "-";
  const url = href || (link ? value : undefined);

  return (
    <div className={className}>
      <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{label}</span>
      {url && value ? (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-[var(--accent)] hover:text-[var(--accent-hover)] truncate block transition-colors"
          style={textStyle}
        >
          {displayValue}
        </a>
      ) : (
        <p className="text-sm text-[var(--text-primary)] truncate" style={textStyle}>
          {displayValue}
        </p>
      )}
    </div>
  );
}
