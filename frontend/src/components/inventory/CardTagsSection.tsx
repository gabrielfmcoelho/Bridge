import Tag from "@/components/ui/Tag";

/** Shared tags row for inventory cards — max visible, +N overflow; always
 *  rendered (fixed anatomy), a muted "–" when there are no tags. */
export default function CardTagsSection({
  tags,
  maxVisible = 4,
}: {
  tags?: string[];
  maxVisible?: number;
}) {
  if (!tags || tags.length === 0) {
    return <div className="mt-3 pt-3 border-t border-[var(--border-subtle)] text-xs text-[var(--text-muted)]">–</div>;
  }

  return (
    <div className="flex flex-wrap gap-1.5 mt-3 pt-3 border-t border-[var(--border-subtle)]">
      {tags.slice(0, maxVisible).map((tag) => (
        <Tag key={tag}>{tag}</Tag>
      ))}
      {tags.length > maxVisible && (
        <span className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium text-[var(--text-muted)]" title={tags.slice(maxVisible).join(", ")}>
          +{tags.length - maxVisible}
        </span>
      )}
    </div>
  );
}
