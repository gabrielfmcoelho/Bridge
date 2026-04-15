import Link from "next/link";

interface LinkedItem {
  id: number | string;
  href: string;
  label: string;
  sublabel?: string;
  badge?: React.ReactNode;
  icon?: React.ReactNode;
}

interface LinkedEntityListProps {
  title: string;
  items: LinkedItem[];
  emptyMessage?: string;
}

export default function LinkedEntityList({ title, items, emptyMessage = "-" }: LinkedEntityListProps) {
  return (
    <div className="bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] p-4">
      <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
        {title}
      </h3>

      {items.length === 0 ? (
        <p className="text-sm text-[var(--text-muted)]">{emptyMessage}</p>
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <Link
              key={item.id}
              href={item.href}
              className="flex items-center gap-2 px-3 py-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] hover:bg-[var(--bg-overlay)] transition-colors group"
            >
              {item.icon}
              <div className="min-w-0 flex-1">
                <p
                  className="text-sm text-[var(--text-primary)] truncate group-hover:text-[var(--accent)] transition-colors"
                  style={{ fontFamily: "var(--font-mono)" }}
                >
                  {item.label}
                </p>
                {item.sublabel && (
                  <p className="text-xs text-[var(--text-muted)] truncate">{item.sublabel}</p>
                )}
              </div>
              {item.badge}
              <svg
                className="w-4 h-4 text-[var(--text-faint)] group-hover:text-[var(--accent)] transition-colors shrink-0"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
              </svg>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
