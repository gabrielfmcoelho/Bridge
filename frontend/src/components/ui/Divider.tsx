// Standalone rule. Section borders on padded containers stay as borders;
// this is for the free-standing lines between groups and inline items.
export default function Divider({ vertical = false, className = "" }: { vertical?: boolean; className?: string }) {
  return vertical ? (
    <span aria-hidden className={`w-px self-stretch shrink-0 bg-[var(--border-subtle)] ${className}`} />
  ) : (
    <div role="separator" className={`border-t border-[var(--border-subtle)] ${className}`} />
  );
}
