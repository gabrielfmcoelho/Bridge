"use client";

import Card from "@/components/ui/Card";
import { SafeMarkdownContent } from "@/components/ui/MarkdownEditor";
import { useLocale } from "@/contexts/LocaleContext";
import type { BundleWikiDoc, BundleWikiItem } from "@/lib/types";

// ShareWikiSection renders one wiki bundle item on the public reveal page: a
// single document, or a collection with its nested document tree. Each document
// is anchored (wiki-{index}-doc-{id}) so the index rail can scroll to it. All
// markdown flows through SafeMarkdownContent (DOMPurify-sanitized) because this
// surface is anonymous/public.

function WikiDocBlock({
  doc,
  sectionIndex,
}: {
  doc: BundleWikiDoc;
  sectionIndex: number;
}) {
  const { t } = useLocale();
  return (
    <div id={`wiki-${sectionIndex}-doc-${doc.id}`} className="scroll-mt-6 space-y-2">
      <div className="flex items-baseline gap-2 flex-wrap">
        {doc.emoji && <span aria-hidden>{doc.emoji}</span>}
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">{doc.title}</h3>
        {doc.browse_url && (
          <a
            href={doc.browse_url}
            target="_blank"
            rel="noreferrer"
            className="text-[10px] text-[var(--accent)] hover:underline"
          >
            {t("share.wikiOpen")} ↗
          </a>
        )}
      </div>
      {doc.updated_by && (
        <p className="text-[10px] text-[var(--text-faint)]">
          {t("share.wikiUpdatedBy", { name: doc.updated_by })}
        </p>
      )}
      {doc.markdown ? (
        <SafeMarkdownContent content={doc.markdown} />
      ) : (
        <p className="text-xs italic text-[var(--text-muted)]">{t("share.wikiEmptyDoc")}</p>
      )}
      {doc.children && doc.children.length > 0 && (
        <div className="mt-3 space-y-4 border-l border-[var(--border-subtle)] pl-4">
          {doc.children.map((c) => (
            <WikiDocBlock key={c.id} doc={c} sectionIndex={sectionIndex} />
          ))}
        </div>
      )}
    </div>
  );
}

export default function ShareWikiSection({
  item,
  index,
}: {
  item: BundleWikiItem;
  index: number;
}) {
  const { t } = useLocale();
  return (
    <div id={`wiki-${index}`} className="space-y-2 scroll-mt-6">
      <div className="flex items-center gap-2 flex-wrap">
        <h2 className="text-sm font-semibold text-[var(--text-secondary)]">{item.title}</h2>
        <span className="text-[10px] px-1.5 py-0.5 rounded border border-[var(--border-default)] bg-[var(--bg-overlay)] text-[var(--text-muted)]">
          {item.kind === "collection" ? t("share.wikiCollection") : t("share.wikiDoc")}
        </span>
        {item.browse_url && (
          <a
            href={item.browse_url}
            target="_blank"
            rel="noreferrer"
            className="text-[10px] text-[var(--accent)] hover:underline"
          >
            {t("share.wikiOpen")} ↗
          </a>
        )}
      </div>
      {item.description && (
        <SafeMarkdownContent
          content={item.description}
          className="markdown-preview text-xs text-[var(--text-muted)]"
        />
      )}
      {item.truncated && (
        <p className="text-[10px] text-amber-400">{t("share.wikiTruncated")}</p>
      )}
      <Card className="space-y-4">
        {item.documents.length === 0 ? (
          <p className="text-xs text-[var(--text-muted)]">{t("share.wikiEmpty")}</p>
        ) : (
          item.documents.map((doc) => (
            <WikiDocBlock key={doc.id} doc={doc} sectionIndex={index} />
          ))
        )}
      </Card>
    </div>
  );
}
