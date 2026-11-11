"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import { Skeleton } from "@/components/ui/Skeleton";
import Button from "@/components/ui/Button";
import Select from "@/components/ui/Select";
import Textarea from "@/components/ui/Textarea";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import { glpiAPI, type GlpiTicketEvent } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";
import { glpiStatusLabelKey } from "@/lib/glpiStatus";
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

const STATUS_VALUES = ["1", "2", "3", "4", "5", "6"];
const STATUS_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.statusNew",
  "2": "glpi.statusAssigned",
  "3": "glpi.statusPlanned",
  "4": "glpi.statusWaiting",
  "5": "glpi.statusSolved",
  "6": "glpi.statusClosed",
};
const URGENCY_VALUES = ["1", "2", "3", "4", "5"];
const URGENCY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.urgencyVeryLow",
  "2": "glpi.urgencyLow",
  "3": "glpi.urgencyMedium",
  "4": "glpi.urgencyHigh",
  "5": "glpi.urgencyVeryHigh",
};
const IMPACT_VALUES = ["1", "2", "3", "4", "5"];
const IMPACT_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.impactVeryLow",
  "2": "glpi.impactLow",
  "3": "glpi.impactMedium",
  "4": "glpi.impactHigh",
  "5": "glpi.impactVeryHigh",
};
const PRIORITY_VALUES = ["1", "2", "3", "4", "5", "6"];
const PRIORITY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.priorityVeryLow",
  "2": "glpi.priorityLow",
  "3": "glpi.priorityMedium",
  "4": "glpi.priorityHigh",
  "5": "glpi.priorityVeryHigh",
  "6": "glpi.priorityMajor",
};
const ACTOR_ROLE_LABEL_KEYS: Record<number, string> = {
  1: "glpi.actorRoleRequester",
  2: "glpi.actorRoleObserver",
  3: "glpi.actorRoleAssign",
};
const VALIDATION_STATUS_LABEL_KEYS: Record<number, string> = {
  1: "glpi.validationStatusWaiting",
  2: "glpi.validationStatusAccepted",
  3: "glpi.validationStatusRefused",
};

export default function TicketDetailDrawer({ open, onClose, ticketID, profileID }: Props) {
  const { locale, t } = useLocale();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const queryClient = useQueryClient();

  const [followupText, setFollowupText] = useState("");
  const [followupPrivate, setFollowupPrivate] = useState(false);
  const [taskText, setTaskText] = useState("");
  const [solutionText, setSolutionText] = useState("");
  const [actorUser, setActorUser] = useState<AsyncPickerItem | null>(null);
  const [actorGroup, setActorGroup] = useState<AsyncPickerItem | null>(null);
  const [actorRole, setActorRole] = useState("1");
  const [approver, setApprover] = useState<AsyncPickerItem | null>(null);
  const [validationComment, setValidationComment] = useState("");
  const [satisfactionRating, setSatisfactionRating] = useState("5");
  const [satisfactionComment, setSatisfactionComment] = useState("");
  const [editError, setEditError] = useState<string | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["glpi-ticket-details", ticketID, profileID],
    queryFn: () => glpiAPI.ticketDetails(ticketID!, profileID!),
    enabled: open && !!ticketID && !!profileID,
    retry: false,
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ["glpi-ticket-details", ticketID, profileID] });
    queryClient.invalidateQueries({ queryKey: ["global-chamados"] });
    queryClient.invalidateQueries({ queryKey: ["profile-chamados"] });
  };

  const updateMutation = useMutation({
    mutationFn: (fields: Parameters<typeof glpiAPI.updateTicket>[2]) =>
      glpiAPI.updateTicket(ticketID!, profileID!, fields),
    onSuccess: invalidate,
    onError: (err: Error) => setEditError(err.message),
  });

  const followupMutation = useMutation({
    mutationFn: () => glpiAPI.addTicketFollowup(ticketID!, profileID!, followupText.trim(), followupPrivate),
    onSuccess: () => {
      setFollowupText("");
      setFollowupPrivate(false);
      invalidate();
    },
    onError: (err: Error) => setEditError(err.message),
  });

  const taskMutation = useMutation({
    mutationFn: () => glpiAPI.addTicketTask(ticketID!, profileID!, taskText.trim()),
    onSuccess: () => {
      setTaskText("");
      invalidate();
    },
    onError: (err: Error) => setEditError(err.message),
  });

  const solutionMutation = useMutation({
    mutationFn: () => glpiAPI.addTicketSolution(ticketID!, profileID!, solutionText.trim()),
    onSuccess: () => {
      setSolutionText("");
      invalidate();
    },
    onError: (err: Error) => setEditError(err.message),
  });

  const actorMutation = useMutation({
    mutationFn: () =>
      glpiAPI.addTicketActor(ticketID!, profileID!, {
        user_id: actorUser?.id,
        group_id: actorGroup?.id,
        role: parseInt(actorRole, 10),
      }),
    onSuccess: () => {
      setActorUser(null);
      setActorGroup(null);
      invalidate();
    },
    onError: (err: Error) => setEditError(err.message),
  });

  const removeActorMutation = useMutation({
    mutationFn: (actor: { id: number; is_group: boolean }) =>
      glpiAPI.removeTicketActor(ticketID!, actor.id, profileID!, actor.is_group),
    onSuccess: invalidate,
    onError: (err: Error) => setEditError(err.message),
  });

  const uploadDocMutation = useMutation({
    mutationFn: (file: File) => glpiAPI.uploadTicketDocument(ticketID!, profileID!, file),
    onSuccess: invalidate,
    onError: (err: Error) => setEditError(err.message),
  });

  const validationMutation = useMutation({
    mutationFn: () => glpiAPI.addTicketValidation(ticketID!, profileID!, approver!.id, validationComment.trim()),
    onSuccess: () => {
      setApprover(null);
      setValidationComment("");
      invalidate();
    },
    onError: (err: Error) => setEditError(err.message),
  });

  const answerValidationMutation = useMutation({
    mutationFn: (args: { validationID: number; status: number }) =>
      glpiAPI.answerTicketValidation(ticketID!, args.validationID, profileID!, args.status),
    onSuccess: invalidate,
    onError: (err: Error) => setEditError(err.message),
  });

  const satisfactionMutation = useMutation({
    mutationFn: () =>
      glpiAPI.submitTicketSatisfaction(
        ticketID!,
        profileID!,
        data!.satisfaction!.id,
        parseInt(satisfactionRating, 10),
        satisfactionComment.trim()
      ),
    onSuccess: invalidate,
    onError: (err: Error) => setEditError(err.message),
  });

  const userFetcher = async (q: string) => {
    if (!profileID) return [];
    const res = await glpiAPI.searchUsers(profileID, q);
    return res.users.map(
      (u) => ({ id: u.id, label: u.display || u.login } as AsyncPickerItem)
    );
  };
  const groupFetcher = async (q: string) => {
    if (!profileID) return [];
    const res = await glpiAPI.searchDropdown("Group", profileID, q);
    return res.items.map((r) => ({ id: r.id, label: r.completename || r.name } as AsyncPickerItem));
  };

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
                {t(glpiStatusLabelKey(data.ticket.status_slug))}
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

            {editError && (
              <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-xs px-3 py-2">
                {editError}
              </div>
            )}

            {/* Quick edit — status/urgency/impact/priority, mirrors GLPI's own ticket form fields */}
            {canEdit && (
              <section>
                <SectionHeading as="h3">{t("glpi.editSectionTitle")}</SectionHeading>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                  <Select
                    label={t("glpi.statusLabel")}
                    options={STATUS_VALUES.map((v) => ({ value: v, label: t(STATUS_LABEL_KEYS[v]) }))}
                    value={String(data.ticket.status)}
                    onChange={(e) => updateMutation.mutate({ status: parseInt(e.target.value, 10) })}
                  />
                  <Select
                    label={t("glpi.urgencyLabel")}
                    options={URGENCY_VALUES.map((v) => ({ value: v, label: t(URGENCY_LABEL_KEYS[v]) }))}
                    value={String(data.ticket.urgency ?? 3)}
                    onChange={(e) => updateMutation.mutate({ urgency: parseInt(e.target.value, 10) })}
                  />
                  <Select
                    label={t("glpi.impactLabel")}
                    options={IMPACT_VALUES.map((v) => ({ value: v, label: t(IMPACT_LABEL_KEYS[v]) }))}
                    value={String(data.ticket.impact ?? 3)}
                    onChange={(e) => updateMutation.mutate({ impact: parseInt(e.target.value, 10) })}
                  />
                  <Select
                    label={t("glpi.priorityLabel")}
                    options={PRIORITY_VALUES.map((v) => ({ value: v, label: t(PRIORITY_LABEL_KEYS[v]) }))}
                    value={String(data.ticket.priority ?? 3)}
                    onChange={(e) => updateMutation.mutate({ priority: parseInt(e.target.value, 10) })}
                  />
                </div>
              </section>
            )}

            {/* Actors */}
            <section>
              <SectionHeading as="h3">{t("glpi.actorsHeading")}</SectionHeading>
              {data.actors.length === 0 ? (
                <p className="text-xs text-[var(--text-muted)]">{t("glpi.noActors")}</p>
              ) : (
                <ul className="flex flex-wrap gap-1.5 mb-2">
                  {data.actors.map((a) => (
                    <li
                      key={`${a.is_group ? "g" : "u"}-${a.id}`}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-xs"
                    >
                      <span className="text-[var(--text-faint)]">{t(ACTOR_ROLE_LABEL_KEYS[a.role] ?? "glpi.actorRoleAssign")}:</span>
                      <span className="text-[var(--text-primary)]">{a.name}</span>
                      {canEdit && (
                        <button
                          type="button"
                          onClick={() => removeActorMutation.mutate({ id: a.id, is_group: a.is_group })}
                          className="w-4 h-4 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--danger)] hover:bg-[var(--bg-overlay)]"
                          aria-label={t("common.removeItem", { label: a.name })}
                        >
                          ×
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
              {canEdit && (
                <div className="flex flex-wrap items-end gap-2">
                  <div className="w-48">
                    <AsyncPicker
                      label={t("glpi.assignToUserLabel")}
                      value={actorUser?.id ?? null}
                      selectedLabel={actorUser?.label}
                      fetcher={userFetcher}
                      onChange={(it) => {
                        setActorUser(it);
                        if (it) setActorGroup(null);
                      }}
                      placeholder={t("glpi.searchUserPlaceholder")}
                    />
                  </div>
                  <div className="w-48">
                    <AsyncPicker
                      label={t("glpi.assignToGroupLabel")}
                      value={actorGroup?.id ?? null}
                      selectedLabel={actorGroup?.label}
                      fetcher={groupFetcher}
                      onChange={(it) => {
                        setActorGroup(it);
                        if (it) setActorUser(null);
                      }}
                      placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "Group" })}
                    />
                  </div>
                  <div className="w-36">
                    <Select
                      label={t("glpi.actorRoleFieldLabel")}
                      options={[1, 2, 3].map((r) => ({ value: String(r), label: t(ACTOR_ROLE_LABEL_KEYS[r]) }))}
                      value={actorRole}
                      onChange={(e) => setActorRole(e.target.value)}
                    />
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    onClick={() => actorMutation.mutate()}
                    loading={actorMutation.isPending}
                    disabled={!actorUser && !actorGroup}
                  >
                    {t("glpi.addActorButton")}
                  </Button>
                </div>
              )}
            </section>

            {/* SLA */}
            {(data.ticket.slas_id_tto || data.ticket.slas_id_ttr) && (
              <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--text-muted)]">
                {!!data.ticket.slas_id_tto && (
                  <span className="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                    {t("glpi.slaTTOLabel")}: #{data.ticket.slas_id_tto}
                  </span>
                )}
                {!!data.ticket.slas_id_ttr && (
                  <span className="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                    {t("glpi.slaTTRLabel")}: #{data.ticket.slas_id_ttr}
                  </span>
                )}
              </div>
            )}

            {/* Documents */}
            <section>
              <SectionHeading as="h3">{t("glpi.documentsHeading")}</SectionHeading>
              {data.documents.length === 0 ? (
                <p className="text-xs text-[var(--text-muted)]">{t("glpi.noDocuments")}</p>
              ) : (
                <ul className="space-y-1 mb-2">
                  {data.documents.map((doc) => (
                    <li key={doc.id}>
                      <Link
                        href={`/api/glpi/documents/${doc.doc_id}${profileID ? `?profile_id=${profileID}` : ""}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-xs text-[var(--accent)] hover:underline inline-flex items-center gap-1"
                      >
                        <Icon path={ICON_PATHS.externalLink} className="w-3 h-3" />
                        {doc.filename}
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
              {canEdit && (
                <label className="inline-flex items-center gap-2 text-xs text-[var(--accent)] hover:underline cursor-pointer">
                  {t("glpi.addDocumentButton")}
                  <input
                    type="file"
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) uploadDocMutation.mutate(file);
                      e.target.value = "";
                    }}
                  />
                </label>
              )}
            </section>

            {/* Validations */}
            <section>
              <SectionHeading as="h3">{t("glpi.validationsHeading")}</SectionHeading>
              {data.validations.length === 0 ? (
                <p className="text-xs text-[var(--text-muted)]">{t("glpi.noValidations")}</p>
              ) : (
                <ul className="space-y-1.5 mb-2">
                  {data.validations.map((v) => (
                    <li
                      key={v.id}
                      className="flex items-center justify-between gap-2 border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-2.5 py-1.5 bg-[var(--bg-elevated)] text-xs"
                    >
                      <span>
                        {t(VALIDATION_STATUS_LABEL_KEYS[v.status] ?? "glpi.validationStatusWaiting")}
                        {v.comment_submission ? ` — ${v.comment_submission}` : ""}
                      </span>
                      {v.status === 1 && canEdit && (
                        <span className="flex gap-1.5 shrink-0">
                          <Button
                            type="button"
                            size="sm"
                            variant="secondary"
                            onClick={() => answerValidationMutation.mutate({ validationID: v.id, status: 2 })}
                          >
                            {t("glpi.validationAccept")}
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="secondary"
                            onClick={() => answerValidationMutation.mutate({ validationID: v.id, status: 3 })}
                          >
                            {t("glpi.validationRefuse")}
                          </Button>
                        </span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
              {canEdit && (
                <div className="flex flex-wrap items-end gap-2">
                  <div className="w-48">
                    <AsyncPicker
                      label={t("glpi.validationApproverLabel")}
                      value={approver?.id ?? null}
                      selectedLabel={approver?.label}
                      fetcher={userFetcher}
                      onChange={setApprover}
                      placeholder={t("glpi.searchUserPlaceholder")}
                    />
                  </div>
                  <div className="flex-1 min-w-[160px]">
                    <Textarea
                      label={t("glpi.validationCommentLabel")}
                      value={validationComment}
                      onChange={(e) => setValidationComment(e.target.value)}
                      rows={1}
                    />
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    onClick={() => validationMutation.mutate()}
                    loading={validationMutation.isPending}
                    disabled={!approver}
                  >
                    {t("glpi.validationRequestButton")}
                  </Button>
                </div>
              )}
            </section>

            {/* Satisfaction */}
            {data.satisfaction && (
              <section>
                <SectionHeading as="h3">{t("glpi.satisfactionHeading")}</SectionHeading>
                {data.satisfaction.satisfaction > 0 ? (
                  <p className="text-xs text-[var(--text-secondary)]">
                    {t("glpi.satisfactionRatingValue", { rating: String(data.satisfaction.satisfaction) })}
                    {data.satisfaction.comment ? ` — ${data.satisfaction.comment}` : ""}
                  </p>
                ) : (
                  <div className="flex flex-wrap items-end gap-2">
                    <div className="w-28">
                      <Select
                        label={t("glpi.satisfactionRatingLabel")}
                        options={["1", "2", "3", "4", "5"].map((v) => ({ value: v, label: v }))}
                        value={satisfactionRating}
                        onChange={(e) => setSatisfactionRating(e.target.value)}
                      />
                    </div>
                    <div className="flex-1 min-w-[160px]">
                      <Textarea
                        label={t("glpi.satisfactionCommentLabel")}
                        value={satisfactionComment}
                        onChange={(e) => setSatisfactionComment(e.target.value)}
                        rows={1}
                      />
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => satisfactionMutation.mutate()}
                      loading={satisfactionMutation.isPending}
                    >
                      {t("glpi.satisfactionSubmitButton")}
                    </Button>
                  </div>
                )}
              </section>
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
              {canEdit && (
                <div className="mt-3 space-y-2">
                  <Textarea
                    label={t("glpi.addFollowupLabel")}
                    value={followupText}
                    onChange={(e) => setFollowupText(e.target.value)}
                    rows={3}
                    placeholder={t("glpi.addFollowupPlaceholder")}
                  />
                  <div className="flex items-center justify-between gap-2">
                    <label className="flex items-center gap-1.5 text-xs text-[var(--text-secondary)] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={followupPrivate}
                        onChange={(e) => setFollowupPrivate(e.target.checked)}
                        className="w-3.5 h-3.5 accent-[var(--accent)]"
                      />
                      {t("glpi.addFollowupPrivate")}
                    </label>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => followupMutation.mutate()}
                      loading={followupMutation.isPending}
                      disabled={!followupText.trim()}
                    >
                      {t("glpi.addFollowupSubmit")}
                    </Button>
                  </div>

                  <Textarea
                    label={t("glpi.addTaskLabel")}
                    value={taskText}
                    onChange={(e) => setTaskText(e.target.value)}
                    rows={2}
                    placeholder={t("glpi.addTaskPlaceholder")}
                  />
                  <div className="flex items-center justify-end">
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      onClick={() => taskMutation.mutate()}
                      loading={taskMutation.isPending}
                      disabled={!taskText.trim()}
                    >
                      {t("glpi.addTaskSubmit")}
                    </Button>
                  </div>

                  <Textarea
                    label={t("glpi.addSolutionLabel")}
                    value={solutionText}
                    onChange={(e) => setSolutionText(e.target.value)}
                    rows={3}
                    placeholder={t("glpi.addSolutionPlaceholder")}
                  />
                  <div className="flex items-center justify-end">
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      onClick={() => solutionMutation.mutate()}
                      loading={solutionMutation.isPending}
                      disabled={!solutionText.trim()}
                    >
                      {t("glpi.addSolutionSubmit")}
                    </Button>
                  </div>
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </Drawer>
  );
}
