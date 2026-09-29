"use client";

import { useState, useRef, useId } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import * as Popover from "@radix-ui/react-popover";
import { useQuery } from "@tanstack/react-query";
import { tagsAPI } from "@/lib/api";
import FormField, { INPUT_CLASS } from "./FormField";
import Tag from "./Tag";
import Button from "./Button";

interface TagInputProps {
  label?: string;
  tags: string[];
  onChange: (tags: string[]) => void;
  suggestions?: string[];
  entityType?: string;
  placeholder?: string;
}

export default function TagInput({ label, tags, onChange, suggestions: externalSuggestions, entityType, placeholder }: TagInputProps) {
  const { t } = useLocale();
  const id = useId();
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
    <FormField label={label} htmlFor={id}>
      <Popover.Root open={shouldShowDropdown} onOpenChange={(isOpen) => { if (!isOpen) setShowSuggestions(false); }}>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <Popover.Anchor asChild>
              <input
                ref={inputRef}
                id={id}
                type="text"
                value={input}
                onChange={(e) => { setInput(e.target.value); setShowSuggestions(true); }}
                onFocus={() => setShowSuggestions(true)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") { e.preventDefault(); addTag(); }
                  if (e.key === "Escape") setShowSuggestions(false);
                }}
                placeholder={placeholder || t("common.addTag")}
                className={INPUT_CLASS}
              />
            </Popover.Anchor>
          </div>
          <Button type="button" variant="secondary" onClick={() => addTag()} disabled={!input.trim()}>
            {t("form.addTag")}
          </Button>
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
        <div className="flex flex-wrap gap-1.5 mt-2">
          {tags.map((tag) => (
            <Tag key={tag} onRemove={() => removeTag(tag)} removeLabel={t("common.removeItem", { label: tag })}>{tag}</Tag>
          ))}
        </div>
      )}
    </FormField>
  );
}
