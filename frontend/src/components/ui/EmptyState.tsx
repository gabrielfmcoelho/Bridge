interface EmptyStateProps {
  icon?: "server" | "globe" | "folder" | "box" | "search" | "key" | "topology";
  title: string;
  description?: string;
  action?: React.ReactNode;
  compact?: boolean;
}

const icons: Record<string, string> = {
  server: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2z",
  globe: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93z",
  folder: "M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z",
  box: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z",
  search: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
  key: "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z",
  topology: "M13 10V3L4 14h7v7l9-11h-7z",
};

export default function EmptyState({ icon = "search", title, description, action, compact }: EmptyStateProps) {
  return (
    <div className={`flex flex-col items-center justify-center animate-fade-in px-6 text-center ${compact ? "py-12" : "min-h-[50vh]"}`}>
      <div className="w-16 h-16 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] flex items-center justify-center mb-4">
        <svg className="w-7 h-7 text-[var(--text-faint)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d={icons[icon]} />
        </svg>
      </div>
      <h3 className="text-sm font-medium text-[var(--text-secondary)] mb-1">{title}</h3>
      {description && <p className="text-xs text-[var(--text-muted)] mb-4">{description}</p>}
      {action}
    </div>
  );
}
