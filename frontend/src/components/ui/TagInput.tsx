"use client";

import { useState, useRef } from "react";
import * as Popover from "@radix-ui/react-popover";
import { useQuery } from "@tanstack/react-query";
import { tagsAPI } from "@/lib/api";

interface TagInputProps {
  label?: string;
  tags: string[];
  onChange: (tags: string[]) => void;
  suggestions?: string[];
  entityType?: string;
}

export default function TagInput({ label, tags, onChange, suggestions: externalSuggestions, entityType }: TagInputProps) {
  const [input, setInput] = useState("");
  const [showSuggestions, setShowSuggestions] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  const { data: fetchedTags = [] } = useQuery({
    queryKey: ["tags", entityType || "__all"],
    queryFn: () => tagsAPI.list(entityType),
    enabled: !externalSuggestions,
  });

  const allSuggestions = externalSuggestions || (Array.isArray(fetchedTags) ? fetchedTags : []);

  const filtered = input.trim()
    ? allSuggestions.filter((s: string) => s.toLowerCase().includes(input.toLowerCase()) && !tags.includes(s))
    : allSuggestions.filter((s: string) => !tags.includes(s));

  const addTag = (value?: string) => {
    const v = (value || input).trim();
    if (v && !tags.includes(v)) {
      onChange([...tags, v]);
    }
    setInput("");
    setShowSuggestions(false);
  };

  const removeTag = (tag: string) => {
    onChange(tags.filter((t) => t !== tag));
  };

  const shouldShowDropdown = showSuggestions && filtered.length > 0;

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <Popover.Root open={shouldShowDropdown} onOpenChange={(isOpen) => { if (!isOpen) setShowSuggestions(false); }}>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Popover.Anchor asChild>
              <input
                ref={inputRef}
                type="text"
                value={input}
                onChange={(e) => { setInput(e.target.value); setShowSuggestions(true); }}
                onFocus={() => setShowSuggestions(true)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") { e.preventDefault(); addTag(); }
                  if (e.key === "Escape") setShowSuggestions(false);
                }}
                placeholder="Add tag..."
                className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none"
              />
            </Popover.Anchor>
          </div>
          <button
            type="button"
            onClick={() => addTag()}
            className="px-3 py-2 text-sm font-medium bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20 rounded-[var(--radius-md)] hover:bg-[var(--accent)]/20 transition-colors"
          >
            +
          </button>
        </div>
        <Popover.Portal>
          <Popover.Content
            side="bottom"
            sideOffset={4}
            align="start"
            className="z-[100] bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg overflow-hidden animate-fade-in"
            style={{ width: "var(--radix-popover-anchor-width)" }}
            onOpenAutoFocus={(e) => e.preventDefault()}
            onCloseAutoFocus={(e) => e.preventDefault()}
          >
            <div className="max-h-36 overflow-y-auto">
              {filtered.slice(0, 15).map((s: string) => (
                <button
                  key={s}
                  type="button"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => addTag(s)}
                  className="w-full text-left px-3 py-1.5 text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--accent)] transition-colors"
                >
                  {s}
                </button>
              ))}
            </div>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mt-3">
          {tags.map((tag) => (
            <span key={tag} className="inline-flex items-center gap-1 px-2.5 py-1 text-xs rounded-full bg-[var(--bg-overlay)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">
              {tag}
              <button type="button" onClick={() => removeTag(tag)} className="w-3.5 h-3.5 flex items-center justify-center rounded-full hover:bg-red-500/20 hover:text-red-400 transition-colors text-[10px] leading-none">
                &times;
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
