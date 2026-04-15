"use client";

import { useState } from "react";
import { marked } from "marked";

marked.setOptions({ breaks: true, gfm: true });

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  rows?: number;
  placeholder?: string;
}

export default function MarkdownEditor({ value, onChange, label, rows = 5, placeholder }: MarkdownEditorProps) {
  const [mode, setMode] = useState<"write" | "preview">("write");

  return (
    <div>
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">{label}</label>
      )}
      <div className="border border-[var(--border-default)] rounded-[var(--radius-md)] overflow-hidden bg-[var(--bg-elevated)]">
        {/* Tab bar */}
        <div className="flex border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]">
          <button
            type="button"
            onClick={() => setMode("write")}
            className={`px-3 py-1.5 text-xs font-medium transition-colors ${
              mode === "write"
                ? "text-[var(--accent)] border-b-2 border-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            }`}
          >
            Write
          </button>
          <button
            type="button"
            onClick={() => setMode("preview")}
            className={`px-3 py-1.5 text-xs font-medium transition-colors ${
              mode === "preview"
                ? "text-[var(--accent)] border-b-2 border-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            }`}
          >
            Preview
          </button>
        </div>

        {mode === "write" ? (
          <textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            rows={rows}
            placeholder={placeholder}
            className="w-full bg-transparent px-3 py-2 text-sm text-[var(--text-primary)] placeholder-[var(--text-faint)] focus:outline-none resize-none"
            style={{ fontFamily: "var(--font-mono)", fontSize: "13px" }}
          />
        ) : (
          <div
            className="markdown-preview px-3 py-2 text-sm text-[var(--text-primary)] min-h-[80px]"
            dangerouslySetInnerHTML={{ __html: value ? marked.parse(value) as string : '<span class="text-[var(--text-faint)]">Nothing to preview</span>' }}
          />
        )}
      </div>
    </div>
  );
}

export function MarkdownContent({ content }: { content: string }) {
  if (!content) return null;
  return (
    <div
      className="markdown-preview text-sm text-[var(--text-secondary)]"
      dangerouslySetInnerHTML={{ __html: marked.parse(content) as string }}
    />
  );
}
