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
  new: "bg-[var(--cyan)]/10 text-[var(--cyan)] border-[var(--cyan)]/30",
  assigned: "bg-[var(--warning)]/10 text-[var(--warning)] border-[var(--warning)]/30",
  planned: "bg-[var(--purple)]/10 text-[var(--purple)] border-[var(--purple)]/30",
  waiting: "bg-[var(--text-faint)]/10 text-[var(--text-muted)] border-[var(--border-default)]",
  solved: "bg-[var(--success)]/10 text-[var(--success)] border-[var(--success)]/30",
  closed: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-subtle)]",
  unknown: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-subtle)]",
};

const priorityColor: Record<number, string> = {
  6: "bg-[var(--danger)]",
  5: "bg-[var(--danger)]",
  4: "bg-[var(--warning)]",
  3: "bg-[var(--cyan)]",
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
  const { locale, t } = useLocale();
  if (tickets.length === 0) {
    return <p className="text-xs text-[var(--text-muted)] py-6 text-center">{emptyLabel ?? t("glpi.noTickets")}</p>;
  }
  const sorted = [...tickets].sort((a, b) => (b.date ?? "").localeCompare(a.date ?? ""));
  const rowClass =
    "block w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2 hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-colors";
  return (
    <ul className="space-y-1.5">
      {sorted.map((tk) => {
        const content = (
          <div className="flex items-center gap-3">
            <span className={`w-2 h-2 rounded-full shrink-0 ${priorityColor[tk.priority] || "bg-[var(--text-faint)]"}`} />
            <code className="text-xs text-[var(--text-muted)] shrink-0">#{tk.id}</code>
            <span className="text-sm text-[var(--text-primary)] truncate flex-1">{tk.name || t("glpi.untitled")}</span>
            {tk.date && (
              <span
                className="shrink-0 text-2xs text-[var(--text-faint)] font-mono"
                title={getTimeAgo(tk.date.replace(" ", "T"), locale)}
              >
                {formatDate(tk.date, locale)}
              </span>
            )}
            <span className={`shrink-0 text-2xs uppercase tracking-wide px-1.5 py-0.5 rounded border ${statusColor[tk.status_slug] || statusColor.unknown}`}>
              {tk.status_label}
            </span>
          </div>
        );
        return (
          <li key={tk.id}>
            {onOpenDetails ? (
              <button type="button" className={rowClass} onClick={() => onOpenDetails(tk.id)}>
                {content}
              </button>
            ) : (
              <Link href={tk.url} target="_blank" rel="noopener noreferrer" className={rowClass}>
                {content}
              </Link>
            )}
          </li>
        );
      })}
    </ul>
  );
}
