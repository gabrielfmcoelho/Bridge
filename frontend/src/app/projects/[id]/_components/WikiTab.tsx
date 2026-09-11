"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { outlineAPI, type OutlineDocumentSummary } from "@/lib/api";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import WikiDocumentList from "@/components/wiki/WikiDocumentList";
import CreateDocumentModal from "@/components/wiki/CreateDocumentModal";
import WikiSearchBar from "@/components/wiki/WikiSearchBar";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  projectId: number;
  canEdit: boolean;
}

export default function WikiTab({ projectId, canEdit }: Props) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["project-wiki", projectId],
    queryFn: () => outlineAPI.projectWiki(projectId),
    retry: false,
  });

  const { data: searchData, isFetching: searchFetching } = useQuery({
    queryKey: ["project-wiki-search", projectId, searchQuery],
    queryFn: () => outlineAPI.searchProjectWiki(projectId, searchQuery),
    enabled: !!searchQuery.trim(),
    retry: false,
  });

  const createMutation = useMutation({
    mutationFn: (title: string) => outlineAPI.createProjectDocument(projectId, title),
    onSuccess: (res) => {
      // Open the freshly-created doc in Outline so the user can fill in the body,
      // and refresh the local list so the row appears immediately on return.
      if (res.browse_url) window.open(res.browse_url, "_blank", "noopener,noreferrer");
      queryClient.invalidateQueries({ queryKey: ["project-wiki", projectId] });
    },
  });

  if (isLoading) {
    return (
      <div className="space-y-3 animate-fade-in">
        <Skeleton className="h-12 w-full rounded-[var(--radius-md)]" />
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-16 w-full rounded-[var(--radius-md)]" />
        ))}
      </div>
    );
  }

  if (!data?.enabled) {
    return (
      <EmptyState
        icon="folder"
        title={t("project.outlineDisabledTitle")}
        description={t("project.outlineDisabledDesc")}
        compact
      />
    );
  }
  if (!data.configured) {
    return (
      <EmptyState
        icon="key"
        title={t("project.outlineNotConfiguredTitle")}
        description={t("project.outlineNotConfiguredDesc")}
        compact
      />
    );
  }
  if (data.warning === "no_collection_linked") {
    return (
      <EmptyState
        icon="folder"
        title={t("project.noWikiCollectionTitle")}
        description={t("project.noWikiCollectionDesc")}
        compact
      />
    );
  }

  const isSearching = searchQuery.trim().length > 0;
  const searchHits = searchData?.results ?? [];

  // Normalize search hits to the same shape WikiDocumentList renders, so both
  // modes (recent list + search results) share a renderer.
  const searchDocs: OutlineDocumentSummary[] = searchHits.map((h) => ({
    id: h.id,
    url_id: h.url_id,
    title: h.title,
    excerpt: h.context,
    updated_at: h.updated_at,
    browse_url: h.browse_url,
  }));

  return (
    <div className="space-y-4 animate-fade-in">
      <Card hover={false} className="!p-3">
        <div className="flex items-center justify-between gap-3 mb-3">
          <div className="min-w-0">
            <p className="text-xs text-[var(--text-muted)]">{t("project.collectionLabel")}</p>
            <p className="text-sm font-semibold text-[var(--text-primary)] truncate">
              {data.collection?.name ?? t("project.loadingPlaceholder")}
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {data.collection_browse_url && (
              <Link
                href={data.collection_browse_url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-[var(--accent)] hover:underline inline-flex items-center gap-1"
              >
                {t("project.openInOutline")}
                <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
              </Link>
            )}
            {canEdit && (
              <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
                {t("project.newPage")}
              </Button>
            )}
          </div>
        </div>
        <WikiSearchBar value={searchQuery} onChange={setSearchQuery} />
      </Card>

      {data.warning && data.warning !== "no_collection_linked" && (
        <div className="rounded-[var(--radius-md)] border border-[var(--warning)]/30 bg-[var(--warning)]/10 text-[var(--warning)] text-xs px-3 py-2">
          {data.warning}
        </div>
      )}

      {isSearching ? (
        searchFetching && searchHits.length === 0 ? (
          <p className="text-xs text-[var(--text-muted)] text-center py-4 animate-pulse">{t("project.searchingEllipsis")}</p>
        ) : (
          <WikiDocumentList
            documents={searchDocs}
            emptyLabel={t("project.noSearchResults", { query: searchQuery })}
          />
        )
      ) : (
        <WikiDocumentList
          documents={data.documents}
          emptyLabel={canEdit ? t("project.noPagesEditableHint") : t("project.noPagesHint")}
        />
      )}

      <CreateDocumentModal
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onSubmit={(title) => createMutation.mutateAsync(title).then(() => undefined)}
        submitting={createMutation.isPending}
      />
    </div>
  );
}
