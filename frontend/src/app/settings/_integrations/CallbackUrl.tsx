"use client";

import CopyButton from "@/components/ui/CopyButton";

/** The OAuth redirect URI an admin pastes into the identity provider. */
export default function CallbackUrl({ label, path }: { label: string; path: string }) {
  const url = typeof window !== "undefined" ? `${window.location.origin}${path}` : path;
  return (
    <div>
      <p className="text-xs font-medium text-[var(--text-muted)] mb-1">{label}</p>
      <div className="flex items-center gap-2">
        <code className="flex-1 min-w-0 bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-xs text-[var(--text-secondary)] font-mono break-all">
          {url}
        </code>
        <CopyButton value={url} size="sm" icon />
      </div>
    </div>
  );
}
