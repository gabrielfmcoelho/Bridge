"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import { Skeleton } from "@/components/ui/Skeleton";
import { glpiAPI, type GlpiTicketEvent } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";

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

const eventLabel: Record<GlpiTicketEvent["type"], string> = {
  followup: "Follow-up",
  task: "Tarefa",
  solution: "Solução",
};

const eventAccent: Record<GlpiTicketEvent["type"], string> = {
  followup: "border-[var(--border-subtle)]",
  task: "border-purple-500/30",
  solution: "border-emerald-500/30",
};

const taskStateLabel: Record<number, string> = {
  0: "Informação",
  1: "A fazer",
  2: "Concluída",
};

const solutionStatusLabel: Record<number, string> = {
  1: "Proposta",
  2: "Aceita",
  3: "Recusada",
};

export default function TicketDetailDrawer({ open, onClose, ticketID, profileID }: Props) {
  const { locale } = useLocale();

  const { data, isLoading, error } = useQuery({
    queryKey: ["glpi-ticket-details", ticketID, profileID],
    queryFn: () => glpiAPI.ticketDetails(ticketID!, profileID!),
    enabled: open && !!ticketID && !!profileID,
    retry: false,
  });

  const title = data?.ticket
    ? `#${data.ticket.id} · ${data.ticket.name || "(sem título)"}`
    : ticketID
    ? `#${ticketID}`
    : "Chamado";

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
            Abrir no GLPI
            <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
            </svg>
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
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-3 py-2">
            Falha: {(error as Error).message}
          </div>
        )}

        {data && (
          <>
            {/* Metadata row */}
            <div className="flex flex-wrap items-center gap-2 text-[11px] text-[var(--text-muted)]">
              <span className="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                {data.ticket.status_label}
              </span>
              {data.ticket.date && (
                <span title={getTimeAgo(data.ticket.date.replace(" ", "T"), locale)}>
                  Aberto {getTimeAgo(data.ticket.date.replace(" ", "T"), locale)}
                </span>
              )}
              {data.requester?.name && <span>· por {data.requester.name}</span>}
              <span>·</span>
              <span>
                {data.event_counts.followup} follow-ups · {data.event_counts.task} tarefas · {data.event_counts.solution} soluções
              </span>
            </div>

            {data.warnings && data.warnings.length > 0 && (
              <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2 space-y-1">
                {data.warnings.map((w, i) => (
                  <p key={i}>{w}</p>
                ))}
              </div>
            )}

            {/* Description */}
            {data.ticket.content && (
              <section>
                <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                  Descrição
                </h3>
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
              <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-2">
                Timeline
              </h3>
              {data.events.length === 0 ? (
                <p className="text-xs text-[var(--text-muted)]">Sem follow-ups, tarefas ou soluções.</p>
              ) : (
                <ul className="space-y-2">
                  {data.events.map((ev) => (
                    <li
                      key={`${ev.type}-${ev.id}`}
                      className={`border rounded-[var(--radius-md)] px-3 py-2.5 bg-[var(--bg-elevated)] ${eventAccent[ev.type]}`}
                    >
                      <div className="flex items-center justify-between gap-2 mb-1.5 flex-wrap">
                        <div className="flex items-center gap-2 text-[11px]">
                          <span className="font-semibold text-[var(--text-secondary)] uppercase tracking-wide">
                            {eventLabel[ev.type]}
                          </span>
                          {ev.user_name && <span className="text-[var(--text-primary)]">{ev.user_name}</span>}
                          {ev.is_private && (
                            <span className="text-[10px] px-1 py-0 rounded border border-amber-500/30 text-amber-400">
                              privado
                            </span>
                          )}
                          {ev.type === "task" && ev.state !== undefined && (
                            <span className="text-[10px] px-1 py-0 rounded border border-purple-500/30 text-purple-400">
                              {taskStateLabel[ev.state] ?? `state ${ev.state}`}
                            </span>
                          )}
                          {ev.type === "solution" && ev.status !== undefined && (
                            <span className="text-[10px] px-1 py-0 rounded border border-emerald-500/30 text-emerald-400">
                              {solutionStatusLabel[ev.status] ?? `status ${ev.status}`}
                            </span>
                          )}
                        </div>
                        <time
                          className="text-[10px] text-[var(--text-faint)]"
                          title={ev.date}
                          style={{ fontFamily: "var(--font-mono)" }}
                        >
                          {getTimeAgo(ev.date.replace(" ", "T"), locale)}
                        </time>
                      </div>
                      <div
                        className="text-sm text-[var(--text-primary)] glpi-content"
                        dangerouslySetInnerHTML={{
                          __html: ev.content
                            ? prepareContent(ev.content, data.glpi_base_url, profileID)
                            : "<em>(vazio)</em>",
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
