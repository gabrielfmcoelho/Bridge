"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import { Skeleton } from "@/components/ui/Skeleton";
import Button from "@/components/ui/Button";
import Textarea from "@/components/ui/Textarea";
import SectionHeading from "@/components/ui/SectionHeading";
import { glpiSeadAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { getTimeAgo } from "@/lib/utils";

interface Props {
  open: boolean;
  onClose: () => void;
  ticketID: number | null;
}

export default function SeadTicketDetailDrawer({ open, onClose, ticketID }: Props) {
  const { locale, t } = useLocale();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const queryClient = useQueryClient();

  const [followupText, setFollowupText] = useState("");
  const [followupPrivate, setFollowupPrivate] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: ticket, isLoading, error: fetchError } = useQuery({
    queryKey: ["glpi-sead-ticket", ticketID],
    queryFn: () => glpiSeadAPI.getTicket(ticketID!),
    enabled: open && !!ticketID,
    retry: false,
  });
  const { data: timeline } = useQuery({
    queryKey: ["glpi-sead-timeline", ticketID],
    queryFn: () => glpiSeadAPI.getTimeline(ticketID!),
    enabled: open && !!ticketID,
    retry: false,
  });
  const { data: team } = useQuery({
    queryKey: ["glpi-sead-team", ticketID],
    queryFn: () => glpiSeadAPI.getTeam(ticketID!),
    enabled: open && !!ticketID,
    retry: false,
  });

  const followupMutation = useMutation({
    mutationFn: () => glpiSeadAPI.addFollowup(ticketID!, followupText.trim(), followupPrivate),
    onSuccess: () => {
      setFollowupText("");
      setFollowupPrivate(false);
      queryClient.invalidateQueries({ queryKey: ["glpi-sead-timeline", ticketID] });
    },
    onError: (err: Error) => setError(err.message),
  });

  const title = ticket ? `#${ticket.id} · ${ticket.name || t("glpi.untitled")}` : ticketID ? `#${ticketID}` : "";

  return (
    <Drawer open={open} onClose={onClose} title={title} wide>
      <div className="p-4 space-y-5">
        {isLoading && (
          <div className="space-y-3">
            <Skeleton className="h-5 w-1/3 rounded" />
            <Skeleton className="h-24 w-full rounded-[var(--radius-md)]" />
          </div>
        )}
        {fetchError && (
          <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-sm px-3 py-2">
            {(fetchError as Error).message}
          </div>
        )}
        {ticket && (
          <>
            <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--text-muted)]">
              <span className="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)]">
                {ticket.status.name}
              </span>
              {ticket.category?.name && <span>{ticket.category.name}</span>}
              {ticket.date && (
                <span title={ticket.date}>{getTimeAgo(ticket.date.replace(" ", "T"), locale)}</span>
              )}
            </div>

            {error && (
              <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-xs px-3 py-2">
                {error}
              </div>
            )}

            {team && team.team.length > 0 && (
              <section>
                <SectionHeading as="h3">{t("glpi.actorsHeading")}</SectionHeading>
                <ul className="flex flex-wrap gap-1.5">
                  {team.team.map((m) => (
                    <li
                      key={`${m.type}-${m.id}`}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-xs"
                    >
                      <span className="text-[var(--text-faint)]">{m.role}:</span>
                      <span className="text-[var(--text-primary)]">{m.name}</span>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {ticket.content && (
              <section>
                <SectionHeading as="h3">{t("common.description")}</SectionHeading>
                <div
                  className="text-sm text-[var(--text-primary)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2.5 glpi-content"
                  dangerouslySetInnerHTML={{ __html: ticket.content }}
                />
              </section>
            )}

            <section>
              <SectionHeading as="h3">{t("glpi.timelineHeading")}</SectionHeading>
              {!timeline?.items.length ? (
                <p className="text-xs text-[var(--text-muted)]">{t("glpi.noEvents")}</p>
              ) : (
                <ul className="space-y-2">
                  {timeline.items.map((ev, i) => (
                    <li
                      key={i}
                      className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2.5 bg-[var(--bg-elevated)]"
                    >
                      <div className="flex items-center justify-between gap-2 mb-1.5">
                        <span className="text-xs font-semibold text-[var(--text-secondary)]">
                          {ev.type}
                        </span>
                        {ev.item.date && (
                          <time className="text-2xs text-[var(--text-faint)] font-mono">
                            {getTimeAgo(String(ev.item.date).replace(" ", "T"), locale)}
                          </time>
                        )}
                      </div>
                      <div
                        className="text-sm text-[var(--text-primary)] glpi-content"
                        dangerouslySetInnerHTML={{
                          __html: ev.item.content ? String(ev.item.content) : `<em>${t("glpi.emptyContent")}</em>`,
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
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </Drawer>
  );
}
