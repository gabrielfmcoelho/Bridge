"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Input from "@/components/ui/Input";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalog, OperationSearchResult } from "@/lib/types";
import AddApiModal from "./AddApiModal";

const SCOPES = ["all", "projeto", "avulso"] as const;
type SearchMode = "apis" | "endpoints";

const methodColor: Record<string, string> = {
  GET: "text-emerald-400 border-emerald-500/30 bg-emerald-500/10",
  POST: "text-cyan-400 border-cyan-500/30 bg-cyan-500/10",
  PUT: "text-amber-400 border-amber-500/30 bg-amber-500/10",
  PATCH: "text-amber-400 border-amber-500/30 bg-amber-500/10",
  DELETE: "text-rose-400 border-rose-500/30 bg-rose-500/10",
};
function methodClass(m: string): string {
  return methodColor[m.toUpperCase()] || "text-[var(--text-secondary)] border-[var(--border-default)] bg-[var(--bg-overlay)]";
}

export default function ApiListView() {
  const { t } = useLocale();
  const router = useRouter();
  const qc = useQueryClient();
  const [mode, setMode] = useState<SearchMode>("apis");
  const [scope, setScope] = useState<(typeof SCOPES)[number]>("all");
  const [q, setQ] = useState("");
  const [adding, setAdding] = useState(false);

  const scopeArg = scope === "all" ? undefined : scope;

  const { data: apis = [], isLoading: apisLoading } = useQuery({
    queryKey: ["api-catalog", scope, q],
    queryFn: () => apiCatalogAPI.list({ scope: scopeArg, q: q.trim() || undefined }),
    enabled: mode === "apis",
  });

  const { data: ops = [], isLoading: opsLoading } = useQuery({
    queryKey: ["api-ops", scope, q],
    queryFn: () => apiCatalogAPI.searchOperations({ scope: scopeArg, q: q.trim() || undefined }),
    enabled: mode === "endpoints",
  });

  const scopeLabel = (s: string) =>
    s === "all" ? t("atlas.apis.scopeAll") : s === "projeto" ? t("atlas.apis.scopeProjeto") : t("atlas.apis.scopeAvulso");

  const segBtn = (active: boolean) =>
    `px-3 py-1.5 text-xs rounded-[var(--radius-md)] border ${
      active
        ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
        : "border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
    }`;

  return (
    <PageShell>
      <PageHeader title={t("atlas.apis.title")} addLabel={t("atlas.apis.add")} onAdd={() => setAdding(true)} />

      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-4">
        {/* APIs | Endpoints toggle */}
        <div className="flex gap-1.5">
          <button type="button" className={segBtn(mode === "apis")} onClick={() => setMode("apis")}>
            {t("atlas.apis.searchModeApis")}
          </button>
          <button type="button" className={segBtn(mode === "endpoints")} onClick={() => setMode("endpoints")}>
            {t("atlas.apis.searchModeEndpoints")}
          </button>
        </div>
        <div className="hidden sm:block w-px h-6 bg-[var(--border-subtle)]" />
        {/* Scope chips */}
        <div className="flex gap-1.5">
          {SCOPES.map((s) => (
            <button key={s} type="button" className={segBtn(scope === s)} onClick={() => setScope(s)}>
              {scopeLabel(s)}
            </button>
          ))}
        </div>
        <div className="flex-1 sm:max-w-md">
          <Input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder={mode === "apis" ? t("atlas.apis.search") : t("atlas.apis.searchEndpoints")}
          />
        </div>
      </div>

      {mode === "apis" ? (
        apisLoading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-28 rounded-[var(--radius-lg)]" />
            ))}
          </div>
        ) : apis.length === 0 ? (
          <EmptyState icon="box" title={t("atlas.apis.empty")} description={t("atlas.apis.emptyHint")} />
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {apis.map((api: ApiCatalog) => (
              <Link key={api.id} href={`/atlas/apis/${api.id}`}>
                <Card className="h-full p-4 hover:border-[var(--border-strong)] transition-colors">
                  <div className="flex items-start justify-between gap-2 mb-2">
                    <h3 className="font-semibold text-[var(--text-primary)] truncate">{api.name}</h3>
                    <Badge color={api.scope === "projeto" ? "purple" : "gray"}>{scopeLabel(api.scope)}</Badge>
                  </div>
                  {api.description && <p className="text-xs text-[var(--text-muted)] line-clamp-2 mb-3">{api.description}</p>}
                  <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
                    <span className="px-1.5 py-0.5 rounded border border-[var(--border-default)] bg-[var(--bg-overlay)]">{api.spec_version}</span>
                    <span>{api.operation_count} {t("atlas.apis.endpoints")}</span>
                    {api.version_label && <span>· v{api.version_label}</span>}
                  </div>
                </Card>
              </Link>
            ))}
          </div>
        )
      ) : opsLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-12 rounded-[var(--radius-md)]" />
          ))}
        </div>
      ) : ops.length === 0 ? (
        <EmptyState icon="search" title={t("atlas.apis.noEndpointMatches")} />
      ) : (
        <div className="space-y-1.5">
          {ops.map((op: OperationSearchResult, i: number) => (
            <Link
              key={`${op.api_id}-${op.op_key}-${i}`}
              href={`/atlas/apis/${op.api_id}?op=${encodeURIComponent(op.op_key)}`}
              className="flex items-center gap-3 px-3 py-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)] transition-colors"
            >
              <span
                className={`shrink-0 text-[10px] font-bold px-1.5 py-0.5 rounded border ${methodClass(op.method)}`}
                style={{ minWidth: "3.25rem", textAlign: "center" }}
              >
                {op.method}
              </span>
              <div className="min-w-0 flex-1">
                <p className="font-mono text-xs text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
                  {op.path}
                </p>
                {(op.summary || op.description) && (
                  <p className="text-[11px] text-[var(--text-muted)] truncate">{op.summary || op.description}</p>
                )}
              </div>
              <span className="shrink-0 text-[11px] text-[var(--text-muted)] flex items-center gap-1.5">
                <Badge color={op.scope === "projeto" ? "purple" : "gray"}>{op.api_name}</Badge>
              </span>
            </Link>
          ))}
        </div>
      )}

      <AddApiModal
        open={adding}
        onClose={() => setAdding(false)}
        onCreated={(api) => {
          qc.invalidateQueries({ queryKey: ["api-catalog"] });
          router.push(`/atlas/apis/${api.id}`);
        }}
      />
    </PageShell>
  );
}
