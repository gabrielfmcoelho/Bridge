"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { GlpiTicketSummary } from "@/lib/api";

function formatDate(d?: string, locale: string = "pt-BR") {
  if (!d) return "";
  // GLPI emits "YYYY-MM-DD HH:MM:SS" in the instance's local time. Replace the
  // space with "T" so Date parses it predictably across browsers.
  const parsed = new Date(d.replace(" ", "T"));
  if (isNaN(parsed.getTime())) return d;
  return parsed.toLocaleDateString(locale === "pt-BR" ? "pt-BR" : "en-US", {
    day: "2-digit",
    month: "short",
    year: "numeric",
  });
}

const statusColor: Record<string, string> = {
  new: "bg-cyan-500/10 text-cyan-400 border-cyan-500/30",
  assigned: "bg-amber-500/10 text-amber-400 border-amber-500/30",
  planned: "bg-purple-500/10 text-purple-400 border-purple-500/30",
  waiting: "bg-gray-500/10 text-gray-400 border-gray-500/30",
  solved: "bg-emerald-500/10 text-emerald-400 border-emerald-500/30",
  closed: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-subtle)]",
  unknown: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-subtle)]",
};

const priorityColor: Record<number, string> = {
  6: "bg-red-500",
  5: "bg-red-400",
  4: "bg-amber-400",
  3: "bg-cyan-400",
  2: "bg-[var(--text-muted)]",
  1: "bg-[var(--text-faint)]",
};

// TicketList renders a list of GLPI tickets. Each row opens GLPI in a new tab.
// Shared by project Chamados tab, host chamados block, and /chamados page.
export default function TicketList({
  tickets,
  emptyLabel,
  onOpenDetails,
}: {
  tickets: GlpiTicketSummary[];
  emptyLabel?: string;
  // When provided, clicking a row opens the details drawer (via the caller)
  // instead of jumping to GLPI. Callers that still want the external-link
  // behaviour just omit this prop.
  onOpenDetails?: (ticketID: number) => void;
}) {
  const { locale } = useLocale();
  if (tickets.length === 0) {
    return <p className="text-xs text-[var(--text-muted)] py-6 text-center">{emptyLabel ?? "No tickets."}</p>;
  }
  const sorted = [...tickets].sort((a, b) => (b.date ?? "").localeCompare(a.date ?? ""));
  const rowClass =
    "block w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2 hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-colors";
  return (
    <ul className="space-y-1.5">
      {sorted.map((t) => {
        const content = (
          <div className="flex items-center gap-3">
            <span className={`w-2 h-2 rounded-full shrink-0 ${priorityColor[t.priority] || "bg-[var(--text-faint)]"}`} />
            <code className="text-[11px] text-[var(--text-muted)] shrink-0">#{t.id}</code>
            <span className="text-sm text-[var(--text-primary)] truncate flex-1">{t.name || "(sem título)"}</span>
            {t.date && (
              <span
                className="shrink-0 text-[10px] text-[var(--text-faint)]"
                title={getTimeAgo(t.date.replace(" ", "T"), locale)}
                style={{ fontFamily: "var(--font-mono)" }}
              >
                {formatDate(t.date, locale)}
              </span>
            )}
            <span className={`shrink-0 text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded border ${statusColor[t.status_slug] || statusColor.unknown}`}>
              {t.status_label}
            </span>
          </div>
        );
        return (
          <li key={t.id}>
            {onOpenDetails ? (
              <button type="button" className={rowClass} onClick={() => onOpenDetails(t.id)}>
                {content}
              </button>
            ) : (
              <Link href={t.url} target="_blank" rel="noopener noreferrer" className={rowClass}>
                {content}
              </Link>
            )}
          </li>
        );
      })}
    </ul>
  );
}
