"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import type { ComponentType } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import {
  ApiError, apiCatalogAPI, dashboardAPI, globalIssuesAPI, hostsAPI, outlineAPI, requestsAPI,
} from "@/lib/api";
import { NAV_ITEMS, canSeeNavItem } from "@/lib/constants";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import { statusColor, statusLabelKey } from "@/lib/requests";
import { getTimeAgo } from "@/lib/utils";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import SectionCard from "@/components/ui/SectionCard";
import StatusAlert from "@/components/ui/StatusAlert";
import { Skeleton } from "@/components/ui/Skeleton";
import { ListRow, RowGroup, RowList, RowText } from "@/components/ui/RowList";
import { peakUsage, HIGH_USAGE } from "@/app/hosts/_components/hostInsights";
import { AiUnavailable, ChatComposer, ChatSuggestions, ChatThread, useAiStatus } from "@/components/ai/Chat";
import { useAiChat } from "@/components/ai/chatStore";
import type { Persona, WidgetId } from "./homeModel";

/** One widget per id in homeModel.WIDGETS; each fetches its own data. */
export const WIDGET_COMPONENTS: Record<WidgetId, ComponentType<{ personas: Persona[] }>> = {
  assistant: Assistant,
  toHandle: ToHandle,
  myRequests: MyRequests,
  myIssues: MyIssues,
  infraHealth: InfraHealth,
  atlas: Atlas,
  quickActions: QuickActions,
  chamados: Chamados,
  inventory: Inventory,
  wiki: Wiki,
};

const ROWS = 5;

function Rows({ n = 3 }: { n?: number }) {
  return (
    <div className="px-5 py-3 space-y-3" aria-hidden>
      {Array.from({ length: n }, (_, i) => <Skeleton key={i} className="h-8 w-full" />)}
    </div>
  );
}

function SeeAll({ href, label }: { href: string; label: string }) {
  return (
    <Link href={href} className="inline-flex items-center gap-1 text-xs font-medium text-[var(--accent)] hover:underline">
      {label}
      <Icon path={ICON_PATHS.arrowRight} className="w-3.5 h-3.5" />
    </Link>
  );
}

function Failed({ title, error }: { title: string; error: Error }) {
  return (
    <SectionCard as="h3" title={title}>
      <StatusAlert variant="error">{error.message}</StatusAlert>
    </SectionCard>
  );
}

// ── Main column: work waiting on the user ────────────────────────────────

/** A mini assistant: the same thread as the header drawer (chatStore), with
 *  starter prompts for the user's personas. Says so while AI isn't configured. */
function Assistant({ personas }: { personas: Persona[] }) {
  const { t } = useLocale();
  const chat = useAiChat();
  const status = useAiStatus();
  if (status.loading) return null;
  if (!status.ready) {
    return (
      <SectionCard as="h3" title={t("home.w.assistant")} icon={ICON_PATHS.sparkles}>
        <AiUnavailable disabled={status.disabled} compact />
      </SectionCard>
    );
  }
  const prompts = [...personas.map((p) => `ai.chat.suggest.${p}`), "ai.chat.suggest.summary"].slice(0, 3).map((k) => t(k));
  return (
    <SectionCard
      as="h3" title={t("home.w.assistant")} icon={ICON_PATHS.sparkles}
      description={chat.messages.length ? undefined : t("home.w.assistantHint")}
      controls={
        <span className="flex gap-1">
          {chat.messages.length > 0 && (
            <IconButton label={t("ai.chat.newChat")} onClick={chat.clear}><Icon path={ICON_PATHS.plus} className="w-4 h-4" /></IconButton>
          )}
          <IconButton label={t("home.w.openAssistant")} onClick={chat.open}><Icon path={ICON_PATHS.externalLink} className="w-4 h-4" /></IconButton>
        </span>
      }
      footer={<ChatComposer pending={chat.pending} onSend={chat.send} onStop={chat.stop} placeholder={t("home.w.assistantPlaceholder")} />}
    >
      <ChatThread
        messages={chat.messages}
        pending={chat.pending}
        onRetry={chat.retry}
        className={chat.messages.length || chat.pending ? "h-96" : ""}
        empty={<ChatSuggestions prompts={prompts} onPick={chat.send} />}
      />
    </SectionCard>
  );
}

function ToHandle() {
  const { t } = useLocale();
  const approve = useQuery({ queryKey: ["requests", "home", "approve"], queryFn: () => requestsAPI.listPaginated({ view: "approve", per_page: ROWS }), retry: false });
  const fulfill = useQuery({ queryKey: ["requests", "home", "fulfill"], queryFn: () => requestsAPI.listPaginated({ view: "fulfill", per_page: ROWS }), retry: false });
  const title = t("home.w.toHandle");
  // ponytail: /service-requests has no Go backend yet (mock only); a 404 hides
  // the widget instead of erroring. Drop this once the backend lands.
  if ([approve.error, fulfill.error].some((e) => e instanceof ApiError && e.status === 404)) return null;
  const error = approve.error ?? fulfill.error;
  if (error) return <Failed title={title} error={error} />;
  const groups = [
    { key: "approve", label: t("home.w.toApprove"), rows: approve.data?.data ?? [], total: approve.data?.meta.total ?? 0 },
    { key: "fulfill", label: t("home.w.toFulfill"), rows: fulfill.data?.data ?? [], total: fulfill.data?.meta.total ?? 0 },
  ].filter((g) => g.rows.length);
  const loading = approve.isLoading || fulfill.isLoading;
  return (
    <SectionCard
      as="h3" title={title} icon={ICON_PATHS.clipboard} body="flush"
      count={loading ? undefined : groups.reduce((n, g) => n + g.total, 0)}
      empty={!loading && !groups.length ? t("home.w.toHandleEmpty") : undefined}
      footer={<SeeAll href="/requests" label={t("home.w.openRequests")} />}
    >
      {loading ? <Rows /> : groups.map((g) => (
        <RowGroup key={g.key} title={g.label}>
          {g.rows.map((r) => (
            <ListRow key={r.id} href={`/requests/${r.id}`}>
              <RowText title={r.title} meta={<><span className="font-mono">#{r.id}</span>{r.requester_name && <span>{r.requester_name}</span>}</>} />
              <Badge color={statusColor(r.status)}>{t(statusLabelKey(r.status))}</Badge>
            </ListRow>
          ))}
        </RowGroup>
      ))}
    </SectionCard>
  );
}

function MyRequests() {
  const { t, locale } = useLocale();
  const q = useQuery({ queryKey: ["requests", "home", "mine"], queryFn: () => requestsAPI.listPaginated({ view: "mine", per_page: ROWS }), retry: false });
  const title = t("home.w.myRequests");
  if (q.error instanceof ApiError && q.error.status === 404) return null; // ponytail: see ToHandle
  if (q.error) return <Failed title={title} error={q.error} />;
  const rows = q.data?.data ?? [];
  return (
    <SectionCard
      as="h3" title={title} icon={ICON_PATHS.clipboard} body="flush" count={q.data?.meta.total}
      empty={!q.isLoading && !rows.length ? t("home.w.myRequestsEmpty") : undefined}
      emptyAction={<SeeAll href="/catalog" label={t("home.w.browseCatalog")} />}
      footer={rows.length ? <SeeAll href="/requests" label={t("home.w.openRequests")} /> : undefined}
    >
      {q.isLoading ? <Rows /> : (
        <RowList>
          {rows.map((r) => (
            <ListRow key={r.id} href={`/requests/${r.id}`}>
              <RowText title={r.title} meta={<><span className="font-mono">#{r.id}</span><span>{getTimeAgo(r.updated_at, locale)}</span></>} />
              <Badge color={statusColor(r.status)}>{t(statusLabelKey(r.status))}</Badge>
            </ListRow>
          ))}
        </RowList>
      )}
    </SectionCard>
  );
}

const PRIORITY_RANK: Record<string, number> = { critical: 0, high: 1, medium: 2, low: 3 };

function MyIssues() {
  const { t } = useLocale();
  const { user } = useAuth();
  const q = useQuery({ queryKey: ["issues", "home"], queryFn: () => globalIssuesAPI.list() });
  const title = t("home.w.myIssues");
  if (q.error) return <Failed title={title} error={q.error} />;
  const mine = (q.data ?? [])
    .filter((i) => i.status !== "done" && !!user && (i.assignee_ids ?? []).includes(user.id))
    .sort((a, b) => (PRIORITY_RANK[a.priority] ?? 9) - (PRIORITY_RANK[b.priority] ?? 9) || (a.expected_end_date ?? "9").localeCompare(b.expected_end_date ?? "9"));
  return (
    <SectionCard
      as="h3" title={title} icon={NAV_ICONS.ListChecks} body="flush" count={q.isLoading ? undefined : mine.length}
      empty={!q.isLoading && !mine.length ? t("home.w.myIssuesEmpty") : undefined}
      footer={mine.length > ROWS ? <SeeAll href="/issues" label={t("home.w.seeAllIssues", { count: String(mine.length) })} /> : undefined}
    >
      {q.isLoading ? <Rows /> : (
        <RowList>
          {mine.slice(0, ROWS).map((i) => (
            <ListRow key={i.id} href="/issues">
              <RowText title={i.title} meta={<><span>{t(i.status === "in_progress" ? "issue.inProgress" : `issue.${i.status}`)}</span>{i.expected_end_date && <span className="font-mono">{i.expected_end_date.slice(0, 10)}</span>}</>} />
              {i.priority && <Badge color={i.priority === "critical" ? "red" : i.priority === "high" ? "amber" : "gray"}>{t(`issue.${i.priority}`)}</Badge>}
            </ListRow>
          ))}
        </RowList>
      )}
    </SectionCard>
  );
}

function InfraHealth() {
  const { t } = useLocale();
  const q = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const title = t("home.w.infraHealth");
  if (q.error) return <Failed title={title} error={q.error} />;
  const hosts = q.data ?? [];
  const critical = hosts.filter((h) => h.alerts?.some((a) => a.level === "critical"));
  const counts = [
    { key: "critical", label: t("host.kpi.critical"), n: critical.length, tone: "var(--danger)" },
    { key: "warning", label: t("home.w.withWarnings"), n: hosts.filter((h) => h.alerts?.some((a) => a.level === "warning")).length, tone: "var(--warning)" },
    { key: "high", label: t("host.kpi.highUsage"), n: hosts.filter((h) => (peakUsage(h) ?? 0) >= HIGH_USAGE).length, tone: "var(--warning)" },
    { key: "noScan", label: t("host.kpi.noScan"), n: hosts.filter((h) => !h.has_scan).length, tone: "var(--text-secondary)" },
  ];
  return (
    <SectionCard
      as="h3" title={title} icon={ICON_PATHS.serverStack} body="flush"
      footer={<SeeAll href="/hosts?tab=dashboard" label={t("home.w.hostsDashboard")} />}
    >
      {q.isLoading ? <Rows n={2} /> : (
        <>
          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-px bg-[var(--border-subtle)] border-b border-[var(--border-subtle)]">
            {counts.map((c) => (
              <div key={c.key} className="bg-[var(--bg-surface)] px-5 py-3">
                <dt className="text-2xs text-[var(--text-muted)]">{c.label}</dt>
                <dd className="mt-0.5 text-xl font-semibold font-mono tabular-nums" style={{ color: c.n ? c.tone : "var(--text-muted)" }}>{c.n}</dd>
              </div>
            ))}
          </dl>
          {critical.length ? (
            <RowList>
              {critical.slice(0, ROWS).map((h) => (
                <ListRow key={h.id} href={`/hosts/${h.oficial_slug}`}>
                  <span aria-hidden className="w-1.5 h-1.5 rounded-full bg-[var(--danger)] shrink-0" />
                  <RowText mono title={h.nickname} meta={h.alerts?.find((a) => a.level === "critical")?.message} />
                </ListRow>
              ))}
            </RowList>
          ) : (
            <p className="px-5 py-3 text-sm text-[var(--text-muted)]">{t("home.w.noCritical")}</p>
          )}
        </>
      )}
    </SectionCard>
  );
}

function Atlas() {
  const { t, locale } = useLocale();
  const q = useQuery({ queryKey: ["api-catalog", "home"], queryFn: () => apiCatalogAPI.list() });
  const title = t("home.w.atlas");
  if (q.error) return <Failed title={title} error={q.error} />;
  const recent = [...(q.data ?? [])].sort((a, b) => b.updated_at.localeCompare(a.updated_at)).slice(0, ROWS);
  const links = [
    { href: "/atlas/catalog", label: "nav.catalog", icon: NAV_ICONS.Database },
    { href: "/atlas/lineage", label: "nav.lineage", icon: NAV_ICONS.GitBranch },
    { href: "/atlas/topology", label: "nav.topology", icon: NAV_ICONS.Network },
    { href: "/atlas/pipeline", label: "nav.pipeline", icon: NAV_ICONS.Workflow },
  ];
  return (
    <SectionCard as="h3" title={title} icon={NAV_ICONS.Plug} body="flush" description={t("home.w.atlasHint")}>
      <div className="flex flex-wrap gap-2 px-5 py-3 border-b border-[var(--border-subtle)]">
        {links.map((l) => (
          <Link
            key={l.href} href={l.href}
            className="inline-flex items-center gap-1.5 rounded-[var(--radius-md)] border border-[var(--border-default)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] active:scale-[0.98] transition"
          >
            <Icon path={l.icon} className="w-3.5 h-3.5" />
            {t(l.label)}
          </Link>
        ))}
      </div>
      {q.isLoading ? <Rows /> : recent.length ? (
        <RowGroup title={t("home.w.recentApis")}>
          {recent.map((a) => (
            <ListRow key={a.id} href={`/atlas/apis/${a.id}`}>
              <RowText title={a.name} meta={<><span>{a.main_responsavel_name}</span><span>{getTimeAgo(a.updated_at, locale)}</span></>} />
            </ListRow>
          ))}
        </RowGroup>
      ) : (
        <p className="px-5 py-3 text-sm text-[var(--text-muted)]">{t("home.w.noApis")}</p>
      )}
    </SectionCard>
  );
}

// ── Side column: shortcuts and reference ─────────────────────────────────

const PERSONA_ACTIONS: Record<Persona, string[]> = {
  infra: ["/hosts", "/services", "/dns", "/secrets"],
  dev: ["/projects", "/issues", "/atlas/apis"],
  chamados: ["/catalog", "/chamados"],
  suporte: ["/chamados"],
  atlas: ["/atlas/catalog", "/atlas/lineage", "/atlas/apis"],
};

function QuickActions({ personas }: { personas: Persona[] }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const hrefs = [...new Set([...personas.flatMap((p) => PERSONA_ACTIONS[p]), "/wiki"])];
  const items = hrefs
    .map((href) => NAV_ITEMS.find((i) => i.href === href))
    .filter((i) => !!i && canSeeNavItem(i, user?.permissions ?? [], user?.role))
    .slice(0, 6);
  return (
    <SectionCard as="h3" title={t("home.w.quickActions")} body="flush">
      <RowList>
        {items.map((i) => (
          <ListRow key={i!.href} href={i!.href}>
            <Icon path={NAV_ICONS[i!.icon] ?? NAV_ICONS.Server} className="w-4 h-4 text-[var(--text-muted)]" strokeWidth={1.5} />
            <RowText title={t(i!.label)} />
            <Icon path={ICON_PATHS.chevronRight} className="w-3.5 h-3.5 text-[var(--text-muted)]" />
          </ListRow>
        ))}
      </RowList>
    </SectionCard>
  );
}

function Chamados() {
  const { t } = useLocale();
  return (
    <SectionCard as="h3" title={t("nav.chamados")} icon={NAV_ICONS.Ticket} body="flush" description={t("home.w.chamadosHint")}>
      <RowList>
        <ListRow href="/chamados/forms">
          <Icon path={ICON_PATHS.plus} className="w-4 h-4 text-[var(--accent)]" />
          <RowText title={t("chamado.newAction")} />
        </ListRow>
        <ListRow href="/chamados">
          <Icon path={ICON_PATHS.clipboard} className="w-4 h-4 text-[var(--text-muted)]" />
          <RowText title={t("home.w.followChamados")} />
        </ListRow>
      </RowList>
    </SectionCard>
  );
}

function Inventory() {
  const { t } = useLocale();
  const q = useQuery({ queryKey: ["dashboard"], queryFn: dashboardAPI.get, staleTime: 60_000 });
  const title = t("nav.inventory");
  if (q.error) return <Failed title={title} error={q.error} />;
  const s = q.data;
  const rows = s ? [
    { href: "/hosts", label: "nav.hosts", icon: "Server", n: s.hosts.total },
    { href: "/services", label: "nav.services", icon: "Boxes", n: s.services },
    { href: "/dns", label: "nav.dns", icon: "Globe", n: s.dns_records },
    { href: "/projects", label: "nav.projects", icon: "FolderKanban", n: s.projects },
    { href: "/issues", label: "nav.issues", icon: "ListChecks", n: s.open_issues },
  ] : [];
  return (
    <SectionCard as="h3" title={title} body="flush">
      {q.isLoading ? <Rows n={4} /> : (
        <RowList>
          {rows.map((r) => (
            <ListRow key={r.href} href={r.href}>
              <Icon path={NAV_ICONS[r.icon]} className="w-4 h-4 text-[var(--text-muted)]" strokeWidth={1.5} />
              <RowText title={t(r.label)} />
              <span className="font-mono tabular-nums text-sm text-[var(--text-primary)]">{r.n}</span>
            </ListRow>
          ))}
        </RowList>
      )}
    </SectionCard>
  );
}

function Wiki() {
  const { t, locale } = useLocale();
  const q = useQuery({ queryKey: ["wiki", "common"], queryFn: outlineAPI.commonWiki, retry: false, staleTime: 5 * 60_000 });
  // The wiki is an optional integration: off or unconfigured hides the widget.
  if (q.error || (q.data && (!q.data.enabled || !q.data.configured))) return null;
  const docs = (q.data?.sections ?? [])
    .flatMap((s) => s.documents)
    .sort((a, b) => b.updated_at.localeCompare(a.updated_at))
    .slice(0, ROWS);
  return (
    <SectionCard
      as="h3" title={t("home.w.wiki")} icon={NAV_ICONS.Book} body="flush"
      empty={!q.isLoading && !docs.length ? t("home.w.wikiEmpty") : undefined}
      footer={<SeeAll href="/wiki" label={t("home.w.openWiki")} />}
    >
      {q.isLoading ? <Rows /> : (
        <RowList>
          {docs.map((d) => (
            <ListRow key={d.id} href={`/wiki?doc=${encodeURIComponent(d.id)}`}>
              <RowText title={d.title} meta={<><span>{getTimeAgo(d.updated_at, locale)}</span>{d.updated_by && <span>{d.updated_by}</span>}</>} />
            </ListRow>
          ))}
        </RowList>
      )}
    </SectionCard>
  );
}
