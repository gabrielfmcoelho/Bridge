"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import { Skeleton } from "@/components/ui/Skeleton";
import EmptyState from "@/components/ui/EmptyState";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import ApiReference from "./ApiReference";
import ShareBundleModal from "./ShareBundleModal";
import ProjectSecretsSheet from "./ProjectSecretsSheet";
import EditApiModal from "./EditApiModal";

export default function ApiDetailInner({ id }: { id: number }) {
  const { t } = useLocale();
  const router = useRouter();
  const qc = useQueryClient();
  const [sharing, setSharing] = useState(false);
  const [secretsOpen, setSecretsOpen] = useState(false);
  const [editing, setEditing] = useState(false);

  const { data: api, isLoading, isError } = useQuery({
    queryKey: ["api-catalog", id],
    queryFn: () => apiCatalogAPI.get(id),
  });

  const { data: spec } = useQuery({
    queryKey: ["api-catalog", id, "spec"],
    queryFn: () => apiCatalogAPI.getSpec(id),
    enabled: !!api,
  });

  // Best-effort deep-jump: when arriving via an endpoint-search result
  // (/atlas/apis/{id}?op=<op_key>), scroll the Scalar frame toward that
  // operation. Scalar owns its DOM and its anchor scheme is version-dependent,
  // so this is intentionally fuzzy — we scan for the path text after a short
  // mount delay and scroll the first match into view. No match → no-op (the
  // API still opens; Scalar's own sidebar is the reliable fallback).
  const searchParams = useSearchParams();
  const op = searchParams.get("op");
  useEffect(() => {
    if (!op || !spec || !api) return;
    const target = api.operations?.find((o) => o.op_key === op);
    const path = target?.path;
    if (!path) return;
    const timer = setTimeout(() => {
      try {
        const root = document.querySelector(".scalar-api-reference") || document;
        const nodes = Array.from(root.querySelectorAll("h1, h2, h3, a, [class*='endpoint'], [class*='operation']"));
        const hit = nodes.find((n) => n.textContent?.includes(path)) as HTMLElement | undefined;
        hit?.scrollIntoView({ behavior: "smooth", block: "start" });
      } catch {
        /* best-effort only */
      }
    }, 800);
    return () => clearTimeout(timer);
  }, [op, spec, api]);

  const refetch = useMutation({
    mutationFn: () => apiCatalogAPI.refetch(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["api-catalog", id] }),
  });

  const remove = useMutation({
    mutationFn: () => apiCatalogAPI.remove(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-catalog"] });
      router.push("/atlas/apis");
    },
  });

  if (isLoading) {
    return (
      <PageShell>
        <Skeleton className="h-8 w-64 mb-4" />
        <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
      </PageShell>
    );
  }

  if (isError || !api) {
    return (
      <PageShell>
        <EmptyState icon="search" title={t("atlas.apis.empty")} />
        <div className="mt-4">
          <Link href="/atlas/apis" className="text-sm text-[var(--accent)] hover:underline">
            ← {t("atlas.apis.title")}
          </Link>
        </div>
      </PageShell>
    );
  }

  // Open externally prefers the human docs page, then the API base, then the
  // spec-derived server, then the spec source.
  const externalURL = api.docs_url || api.base_url || api.external_url || api.source_url;
  // Test Request should hit the real API host: prefer the explicit base_url.
  const serverUrl = api.base_url || undefined;
  const scopeLabel = api.scope === "projeto" ? t("atlas.apis.scopeProjeto") : t("atlas.apis.scopeAvulso");
  const hasProject = api.scope === "projeto" && api.parent_id != null;

  return (
    <PageShell>
      <Link href="/atlas/apis" className="text-xs text-[var(--accent)] hover:underline">
        ← {t("atlas.apis.title")}
      </Link>

      {/* Header */}
      <div className="flex flex-wrap items-start justify-between gap-3 mt-2 mb-4">
        <div className="min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold truncate" style={{ fontFamily: "var(--font-display)" }}>
              {api.name}
            </h1>
            <Badge color={api.scope === "projeto" ? "purple" : "gray"}>{scopeLabel}</Badge>
            <Badge color="cyan">{api.spec_version}</Badge>
            {api.version_label && <Badge color="default">v{api.version_label}</Badge>}
          </div>
          {api.description && <p className="text-sm text-[var(--text-muted)] mt-1">{api.description}</p>}
        </div>
        <div className="flex items-center gap-2">
          <Button variant="primary" size="sm" type="button" onClick={() => setSharing(true)}>
            {t("atlas.apis.share")}
          </Button>
          {hasProject && (
            <Button variant="secondary" size="sm" type="button" onClick={() => setSecretsOpen(true)}>
              {t("atlas.apis.secretsButton")}
            </Button>
          )}
          {externalURL && (
            <a href={externalURL} target="_blank" rel="noopener noreferrer">
              <Button variant="secondary" size="sm" type="button">
                {t("atlas.apis.openExternally")} ↗
              </Button>
            </a>
          )}
          <Button variant="secondary" size="sm" type="button" onClick={() => setEditing(true)}>
            {t("atlas.apis.edit")}
          </Button>
          {api.source_type === "url" && (
            <Button variant="secondary" size="sm" type="button" loading={refetch.isPending} onClick={() => refetch.mutate()}>
              {t("atlas.apis.refetch")}
            </Button>
          )}
          <Button
            variant="danger"
            size="sm"
            type="button"
            loading={remove.isPending}
            onClick={() => {
              if (window.confirm(t("atlas.apis.deleteConfirm"))) remove.mutate();
            }}
          >
            {t("atlas.apis.delete")}
          </Button>
        </div>
      </div>

      {/* Scalar reference only — full width, no surrounding chrome. */}
      <div className="min-h-[70vh]">
        {spec ? (
          <ApiReference content={spec} serverUrl={serverUrl} />
        ) : (
          <Skeleton className="h-[60vh] w-full rounded-[var(--radius-md)]" />
        )}
      </div>

      <EditApiModal open={editing} onClose={() => setEditing(false)} api={api} />
      <ShareBundleModal open={sharing} onClose={() => setSharing(false)} api={api} />
      {hasProject && (
        <ProjectSecretsSheet open={secretsOpen} onClose={() => setSecretsOpen(false)} projectId={api.parent_id!} />
      )}
    </PageShell>
  );
}
