"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import { Skeleton } from "@/components/ui/Skeleton";
import { glpiAPI, type GlpiTicketEvent } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import SectionHeading from "@/components/ui/SectionHeading";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

// Some GLPI deployments (< 10.0.4) ignore the ?sanitize=false flag and still
// return HTML as entity-encoded text ("&lt;div&gt;…"). The textarea trick
// decodes any HTML entity the browser knows about in one pass. Skipped during
// SSR to avoid "document is not defined".
function decodeGlpiHtml(s?: string): string {
  if (!s) return "";
  if (typeof document === "undefined") return s;
  const ta = document.createElement("textarea");
  ta.innerHTML = s;
  return ta.value;
}

// Pull the docid from a GLPI document URL (e.g.
// "/front/document.send.php?docid=17320&itemtype=Ticket&items_id=54109").
// Accepts both absolute and relative forms. Returns null when the URL isn't a
// document download.
function extractGlpiDocID(raw: string): number | null {
  const m = /document\.send\.php\?([^#]*)/i.exec(raw);
  if (!m) return null;
  const params = new URLSearchParams(m[1]);
  const id = parseInt(params.get("docid") || "", 10);
  return Number.isFinite(id) && id > 0 ? id : null;
}

// Rewrite URLs inside GLPI ticket HTML so:
//   1. Document downloads (`/front/document.send.php?docid=…`) route through
//      sshcm's /api/glpi/documents/{id} proxy — authenticated via the drawer's
//      profile. Lets <img> render inline and <a> downloads inherit our session.
//   2. Other relative URLs get the GLPI base prefix so the link still works
//      when opened in a new tab.
//   3. Absolute/mailto/tel/data/javascript URLs are left alone.
function rewriteGlpiLinks(
  html: string,
  glpiBaseURL: string,
  profileID: number | null
): string {
  if (!html) return html;
  if (typeof document === "undefined") return html;
  const base = (glpiBaseURL || "").replace(/\/+$/, "");
  const doc = new DOMParser().parseFromString(`<div>${html}</div>`, "text/html");

  const proxyDoc = (docID: number) =>
    `/api/glpi/documents/${docID}${profileID ? `?profile_id=${profileID}` : ""}`;

  const fix = (el: Element, attr: "href" | "src") => {
    const v = el.getAttribute(attr);
    if (!v) return;

    // 1. Document downloads — always proxy, regardless of original (relative
    //    vs absolute) form. Needs an authenticated session.
    const docID = extractGlpiDocID(v);
    if (docID != null && profileID) {
      el.setAttribute(attr, proxyDoc(docID));
      if (attr === "href") {
        // Open in a new tab so browsers use their own inline viewer (PDF etc.).
        el.setAttribute("target", "_blank");
        el.setAttribute("rel", "noopener noreferrer");
      }
      return;
    }

    // 2. Leave protocol-qualified and special schemes alone.
    if (/^(https?:)?\/\//i.test(v)) return;
    if (/^(mailto:|tel:|data:|javascript:|#)/i.test(v)) return;

    // 3. Other relative URLs → prefix with GLPI base.
    if (!base) return;
    const prefixed = v.startsWith("/") ? base + v : base + "/" + v;
    el.setAttribute(attr, prefixed);
    if (attr === "href") {
      el.setAttribute("target", "_blank");
      el.setAttribute("rel", "noopener noreferrer");
    }
  };

  doc.querySelectorAll("a[href]").forEach((el) => fix(el, "href"));
  doc.querySelectorAll("img[src]").forEach((el) => fix(el, "src"));
  return doc.body.firstElementChild?.innerHTML ?? html;
}

function prepareContent(
  raw: string | undefined,
  glpiBaseURL: string,
  profileID: number | null
): string {
  return rewriteGlpiLinks(decodeGlpiHtml(raw), glpiBaseURL, profileID);
}

interface Props {
  open: boolean;
  onClose: () => void;
  ticketID: number | null;
  profileID: number | null;
}

// Values are catalogue keys, translated at usage time (module scope can't call the hook).
const eventLabel: Record<GlpiTicketEvent["type"], string> = {
  followup: "glpi.eventFollowup",
  task: "glpi.eventTask",
  solution: "glpi.eventSolution",
};

const eventAccent: Record<GlpiTicketEvent["type"], string> = {
  followup: "border-[var(--border-subtle)]",
  task: "border-[var(--accent)]/30",
  solution: "border-[var(--success)]/30",
};

const taskStateLabel: Record<number, string> = {
  0: "glpi.taskStateInfo",
  1: "glpi.taskStateTodo",
  2: "glpi.taskStateDone",
};

const solutionStatusLabel: Record<number, string> = {
  1: "glpi.solutionStatusProposed",
  2: "glpi.solutionStatusAccepted",
  3: "glpi.solutionStatusRejected",
};

export default function TicketDetailDrawer({ open, onClose, ticketID, profileID }: Props) {
  const { locale, t } = useLocale();

  const { data, isLoading, error } = useQuery({
    queryKey: ["glpi-ticket-details", ticketID, profileID],
    queryFn: () => glpiAPI.ticketDetails(ticketID!, profileID!),
    enabled: open && !!ticketID && !!profileID,
    retry: false,
  });

  const title = data?.ticket
    ? `#${data.ticket.id} · ${data.ticket.name || t("glpi.untitled")}`
    : ticketID
    ? `#${ticketID}`
    : t("glpi.ticketFallbackTitle");

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={title}
      wide
      headerAction={
        data?.ticket?.url ? (
          <Link
            href={data.ticket.url}
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-[var(--accent)] hover:underline inline-flex items-center gap-1"
          >
            {t("glpi.openInGlpi")}
            <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
          </Link>
        ) : undefined
      }
    >
      <div className="p-4 space-y-5">
        {isLoading && (
          <div className="space-y-3">
            <Skeleton className="h-5 w-1/3 rounded" />
            <Skeleton className="h-24 w-full rounded-[var(--radius-md)]" />
            <Skeleton className="h-16 w-full rounded-[var(--radius-md)]" />
            <Skeleton className="h-16 w-full rounded-[var(--radius-md)]" />
          </div>
        )}

        {error && (
          <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-sm px-3 py-2">
            {t("glpi.ticketLoadError", { message: (error as Error).message })}
          </div>
        )}

        {data && (
          <>
            {/* Metadata row */}
            <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--text-muted)]">
              <span className="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                {data.ticket.status_label}
              </span>
              {data.ticket.date && (
                <span title={getTimeAgo(data.ticket.date.replace(" ", "T"), locale)}>
                  {t("glpi.openedTimeAgo", { time: getTimeAgo(data.ticket.date.replace(" ", "T"), locale) })}
                </span>
              )}
              {data.requester?.name && <span>{t("glpi.byRequester", { name: data.requester.name })}</span>}
              <span>·</span>
              <span>
                {t("glpi.eventCounts", {
                  followups: String(data.event_counts.followup),
                  tasks: String(data.event_counts.task),
                  solutions: String(data.event_counts.solution),
                })}
              </span>
            </div>

            {data.warnings && data.warnings.length > 0 && (
              <div className="rounded-[var(--radius-md)] border border-[var(--warning)]/30 bg-[var(--warning)]/10 text-[var(--warning)] text-xs px-3 py-2 space-y-1">
                {data.warnings.map((w, i) => (
                  <p key={i}>{w}</p>
                ))}
              </div>
            )}

            {/* Description */}
            {data.ticket.content && (
              <section>
                <SectionHeading as="h3">
                  {t("common.description")}
                </SectionHeading>
                <div
                  className="text-sm text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2.5 glpi-content"
                  // GLPI stores content as HTML. We render it directly — the source is
                  // the authenticated GLPI instance, same trust domain as sshcm admins.
                  dangerouslySetInnerHTML={{
                    __html: prepareContent(data.ticket.content, data.glpi_base_url, profileID),
                  }}
                />
              </section>
            )}

            {/* Timeline */}
            <section>
              <SectionHeading as="h3">
                {t("glpi.timelineHeading")}
              </SectionHeading>
              {data.events.length === 0 ? (
                <p className="text-xs text-[var(--text-muted)]">{t("glpi.noEvents")}</p>
              ) : (
                <ul className="space-y-2">
                  {data.events.map((ev) => (
                    <li
                      key={`${ev.type}-${ev.id}`}
                      className={`border rounded-[var(--radius-md)] px-3 py-2.5 bg-[var(--bg-elevated)] ${eventAccent[ev.type]}`}
                    >
                      <div className="flex items-center justify-between gap-2 mb-1.5 flex-wrap">
                        <div className="flex items-center gap-2 text-xs">
                          <span className="font-semibold text-[var(--text-secondary)]">
                            {t(eventLabel[ev.type])}
                          </span>
                          {ev.user_name && <span className="text-[var(--text-primary)]">{ev.user_name}</span>}
                          {ev.is_private && (
                            <span className="text-2xs px-1 py-0 rounded border border-[var(--warning)]/30 text-[var(--warning)]">
                              {t("glpi.privateBadge")}
                            </span>
                          )}
                          {ev.type === "task" && ev.state !== undefined && (
                            <span className="text-2xs px-1 py-0 rounded border border-[var(--accent)]/30 text-[var(--accent)]">
                              {t(taskStateLabel[ev.state] ?? "glpi.unknownStateFallback", { state: String(ev.state) })}
                            </span>
                          )}
                          {ev.type === "solution" && ev.status !== undefined && (
                            <span className="text-2xs px-1 py-0 rounded border border-[var(--success)]/30 text-[var(--success)]">
                              {t(solutionStatusLabel[ev.status] ?? "glpi.unknownStatusFallback", { status: String(ev.status) })}
                            </span>
                          )}
                        </div>
                        <time
                          className="text-2xs text-[var(--text-faint)] font-mono"
                          title={ev.date}
                        >
                          {getTimeAgo(ev.date.replace(" ", "T"), locale)}
                        </time>
                      </div>
                      <div
                        className="text-sm text-[var(--text-primary)] glpi-content"
                        dangerouslySetInnerHTML={{
                          __html: ev.content
                            ? prepareContent(ev.content, data.glpi_base_url, profileID)
                            : `<em>${t("glpi.emptyContent")}</em>`,
                        }}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </section>
          </>
        )}
      </div>
    </Drawer>
  );
}
