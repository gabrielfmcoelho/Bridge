"use client";

import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import type { OutlineFullDocument } from "@/lib/api";

interface Props {
  doc: OutlineFullDocument | undefined;
  isLoading: boolean;
  error?: Error | null;
}

// The viewer renders only the markdown body now — the doc's emoji/title/meta
// live in the left nav (see /wiki/page.tsx) so the reading area is clutter-free.
export default function WikiDocumentViewer({ doc, isLoading, error }: Props) {
  if (isLoading) {
    return (
      <div className="space-y-3 animate-pulse">
        <div className="h-4 w-full bg-[var(--bg-elevated)] rounded" />
        <div className="h-4 w-5/6 bg-[var(--bg-elevated)] rounded" />
        <div className="h-4 w-11/12 bg-[var(--bg-elevated)] rounded" />
        <div className="h-4 w-3/4 bg-[var(--bg-elevated)] rounded" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-4 py-3">
        Failed to load document: {error.message}
      </div>
    );
  }

  if (!doc) return null;

  return (
    <article className="wiki-doc">
      <MarkdownContent content={doc.text || ""} />
    </article>
  );
}
