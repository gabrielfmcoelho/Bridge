"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import type { OutlineDocumentSummary } from "@/lib/api";

interface Props {
  documents: OutlineDocumentSummary[];
  emptyLabel?: string;
  // When provided, rows render as buttons that select the doc in-app instead of
  // links that open Outline in a new tab.
  onSelect?: (id: string) => void;
}

export default function WikiDocumentList({ documents, emptyLabel, onSelect }: Props) {
  const { locale } = useLocale();

  if (documents.length === 0) {
    return (
      <p className="text-xs text-[var(--text-muted)] py-6 text-center">
        {emptyLabel ?? "No documents yet."}
      </p>
    );
  }

  const rowClass =
    "block w-full text-left rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-3 py-2.5 hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-colors";

  return (
    <ul className="space-y-1.5">
      {documents.map((doc) => {
        const body = (
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-[var(--text-primary)] truncate">
                {doc.emoji && <span className="mr-1.5">{doc.emoji}</span>}
                {doc.title || "Untitled"}
              </p>
              {doc.excerpt && (
                <p className="text-[11px] text-[var(--text-muted)] mt-0.5 line-clamp-2">
                  {doc.excerpt}
                </p>
              )}
              <div className="flex items-center gap-2 mt-1 text-[10px] text-[var(--text-faint)]">
                {doc.updated_by && <span>{doc.updated_by}</span>}
                {doc.updated_by && <span>·</span>}
                <span style={{ fontFamily: "var(--font-mono)" }}>
                  {getTimeAgo(doc.updated_at, locale)}
                </span>
              </div>
            </div>
            {!onSelect && (
              <svg className="w-3.5 h-3.5 text-[var(--text-faint)] shrink-0 mt-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            )}
          </div>
        );
        return (
          <li key={doc.id}>
            {onSelect ? (
              <button type="button" className={rowClass} onClick={() => onSelect(doc.id)}>
                {body}
              </button>
            ) : (
              <Link href={doc.browse_url} target="_blank" rel="noopener noreferrer" className={rowClass}>
                {body}
              </Link>
            )}
          </li>
        );
      })}
    </ul>
  );
}
