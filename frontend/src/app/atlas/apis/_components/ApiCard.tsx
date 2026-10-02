"use client";

import { useState } from "react";
import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import { CardHeader, CardMetadataGrid, CardIndicator, CardIndicatorGrid } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { ApiCatalog } from "@/lib/types";
import { isSpecStale, ORIGEM_COLOR, plainText } from "./apiInsights";
import { baseHost, linkedNames, useApiLinkNames } from "./apiDisplay";

export default function ApiCard({ api }: { api: ApiCatalog }) {
  const { t } = useLocale();
  const names = useApiLinkNames();
  const [open, setOpen] = useState(false);
  const stale = isSpecStale(api);
  const origem = api.origem ?? "propria";
  const services = api.service_ids ?? [];
  const projects = api.project_ids ?? [];
  const consumers = api.consumer_service_ids ?? [];
  const avulso = services.length + projects.length === 0;
  const hasText = !!(api.description?.trim() || api.use_cases?.trim());

  return (
    <Link href={`/atlas/apis/${api.id}`} className="block h-full">
      <Card accent={stale ? "warning" : origem === "propria" && avulso ? "muted" : ORIGEM_COLOR[origem]} className="h-full flex flex-col overflow-hidden">
        <CardHeader
          titleFont="display"
          title={api.name}
          subtitle={baseHost(api) || undefined}
          status={
            <span className="inline-flex items-center gap-1.5">
              <Badge color={ORIGEM_COLOR[origem]}>{t(`atlas.apis.origem.${origem}`)}</Badge>
              {api.key_management === "keycloak" && <span className="text-xs text-[var(--text-muted)]">{t("atlas.apis.keycloakKeys")}</span>}
            </span>
          }
          description={plainText(api.description) || api.title}
        />

        <CardMetadataGrid
          items={[
            { label: t("topology.projects"), value: linkedNames(projects, names.project) },
            { label: t("atlas.apis.mainResponsavel"), value: api.main_responsavel_name ?? "" },
          ]}
        />

        <CardIndicatorGrid>
          <CardIndicator icon={ICON_PATHS.code} count={api.operation_count} color="info" title={t("atlas.apis.endpointsCount", { count: String(api.operation_count) })} />
          <CardIndicator icon={ICON_PATHS.serverStack} count={services.length} color="warning" title={t("atlas.apis.servedBy")} />
          <CardIndicator icon={ICON_PATHS.link} count={consumers.length} color="accent" title={t("atlas.apis.consumedBy")} />
          <CardIndicator icon={ICON_PATHS.folder} count={projects.length} color="accent" title={t("topology.projects")} />
          <CardIndicator icon={ICON_PATHS.globe} count={api.base_url ? 1 : 0} color="success" hideCount title={api.base_url ? t("atlas.apis.baseUrl") : t("atlas.apis.kpi.noBaseUrl")} />
          <CardIndicator icon={ICON_PATHS.clock} count={stale ? 1 : 0} color="warning" hideCount title={stale ? t("atlas.apis.kpi.specStale") : t("atlas.apis.specFresh")} />
        </CardIndicatorGrid>

        {hasText && (
          <div className="mt-2 border-t border-[var(--border-subtle)] pt-2">
            {/* Inside the card's link: the toggle must not navigate. */}
            <button type="button" aria-expanded={open}
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); setOpen((v) => !v); }}
              className="flex w-full items-center gap-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]">
              <Icon path={open ? ICON_PATHS.chevronUp : ICON_PATHS.chevronDown} className="w-3.5 h-3.5" />
              {t("atlas.apis.aboutToggle")}
            </button>
            {open && (
              <div className="mt-2 space-y-3 cursor-default" onClick={(e) => { e.preventDefault(); e.stopPropagation(); }}>
                {api.description?.trim() && <MarkdownContent content={api.description} />}
                {api.use_cases?.trim() && (
                  <div>
                    <p className="text-xs font-medium text-[var(--text-secondary)] mb-1">{t("atlas.apis.useCases")}</p>
                    <MarkdownContent content={api.use_cases} />
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </Card>
    </Link>
  );
}
