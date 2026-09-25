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
  const mono_ = mono ? " font-mono" : "";
  const displayValue = value;
  const url = href || (link ? value : undefined);

  return (
    <div className={className}>
      <span className="text-[var(--text-muted)] text-xs font-medium block mb-0.5">{label}</span>
      {url && value ? (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className={`text-sm text-[var(--accent)] hover:text-[var(--accent-hover)] truncate block transition-colors${mono_}`}
        >
          {displayValue}
        </a>
      ) : value ? (
        <p className={`text-sm text-[var(--text-primary)] truncate${mono_}`} title={value}>
          {value}
        </p>
      ) : (
        // Fixed anatomy: an empty value keeps its slot as a muted dash.
        <p className="text-sm text-[var(--text-muted)]">–</p>
      )}
    </div>
  );
}
