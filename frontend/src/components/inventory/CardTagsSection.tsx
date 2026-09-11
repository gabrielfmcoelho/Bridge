import Badge from "@/components/ui/Badge";

/** Shared tags row for inventory cards — max visible, +N overflow, nothing when empty. */
export default function CardTagsSection({
  tags,
  maxVisible = 4,
}: {
  tags?: string[];
  maxVisible?: number;
}) {
  if (!tags || tags.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-1.5 mt-3 pt-3 border-t border-[var(--border-subtle)]">
      {tags.slice(0, maxVisible).map((tag) => (
        <Badge key={tag}>{tag}</Badge>
      ))}
      {tags.length > maxVisible && (
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium text-[var(--text-muted)] bg-[var(--bg-overlay)] border border-[var(--border-default)]">
          +{tags.length - maxVisible}
        </span>
      )}
    </div>
  );
}
