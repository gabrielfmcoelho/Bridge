"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import DetailHeader from "@/components/ui/DetailHeader";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Textarea from "@/components/ui/Textarea";
import StatusAlert from "@/components/ui/StatusAlert";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { ApiError, requestsAPI } from "@/lib/api";
import {
  assetHref, assetTypeLabelKey, availableTransitions, statusColor,
  statusLabelKey, TRANSITION_ACTION_KEY, transitionVariant,
} from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { RequestStatus } from "@/lib/types";
import RequestAnswers from "@/components/requests/RequestAnswers";
import RequestTimeline from "./_components/RequestTimeline";
import TransitionModal from "./_components/TransitionModal";
import RequestEditModal from "./_components/RequestEditModal";

export default function RequestDetail({ id }: { id: number }) {
  const { t, formatDate, formatDateTime } = useLocale();
  const queryClient = useQueryClient();

  const [pendingTo, setPendingTo] = useState<RequestStatus | null>(null);
  const [editing, setEditing] = useState(false);
  const [comment, setComment] = useState("");
  const [commentError, setCommentError] = useState("");

  // Key prefixed with "requests" so the create flow's
  // invalidateQueries({queryKey:["requests"]}) refreshes this page too —
  // React Query matches by prefix.
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["requests", id],
    queryFn: () => requestsAPI.get(id),
    retry: (count, err) => !(err instanceof ApiError && err.status === 404) && count < 2,
  });

  const refresh = () => queryClient.invalidateQueries({ queryKey: ["requests"] });

  const commentMutation = useMutation({
    mutationFn: () => requestsAPI.comment(id, comment.trim()),
    onSuccess: () => {
      setComment("");
      setCommentError("");
      refresh();
    },
    onError: (err) => setCommentError(err instanceof Error ? err.message : t("requests.commentError")),
  });

  if (isLoading) {
    return (
      <PageShell>
        <div className="space-y-6">
          <Skeleton className="h-4 w-20" />
          <div className="space-y-2">
            <Skeleton className="h-7 w-80 max-w-full" />
            <Skeleton className="h-4 w-40" />
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
            <div className="lg:col-span-2 h-64 skeleton rounded-[var(--radius-lg)]" />
            <div className="h-64 skeleton rounded-[var(--radius-lg)]" />
          </div>
        </div>
      </PageShell>
    );
  }

  if (isError || !data) {
    const notFound = error instanceof ApiError && error.status === 404;
    return (
      <PageShell>
        {notFound ? (
          <EmptyState
            icon="search"
            title={t("requests.notFound")}
            description={t("requests.notFoundHint")}
            action={
              <Link href="/requests">
                <Button size="sm">{t("requests.title")}</Button>
              </Link>
            }
          />
        ) : (
          <div className="space-y-4">
            <StatusAlert variant="error">
              {(error as Error)?.message || t("requests.detailError")}
            </StatusAlert>
            <Link href="/requests" className="text-sm text-[var(--accent)] hover:underline">
              {t("requests.title")}
            </Link>
          </div>
        )}
      </PageShell>
    );
  }

  const { request, offering, events, can } = data;
  // Legal next states, already filtered to the ones the server says this
  // actor may take. The four booleans are authority; TRANSITIONS is legality.
  const moves = availableTransitions(request.status, can);

  // Only the rows this record actually has — an empty "Decided" line on a
  // request nobody has decided is noise, not information.
  const meta: { label: string; value: React.ReactNode }[] = [
    { label: t("requests.offering"), value: offering?.name ?? request.offering_name ?? "—" },
    { label: t("requests.requester"), value: request.requester_name ?? "—" },
    // Shown only when it differs from the requester: repeating the same name on
    // its own row is noise, but "call Ana, not Marcos" is the whole point.
    ...(request.contact_name && request.contact_name !== request.requester_name
      ? [{ label: t("catalog.steps.contactName"), value: request.contact_name }]
      : []),
    ...(request.contact_phone
      ? [{
          label: t("catalog.steps.contactPhone"),
          value: (
            <a href={`tel:${request.contact_phone.replace(/[^+\d]/g, "")}`} className="text-[var(--accent)] hover:underline">
              {request.contact_phone}
            </a>
          ),
        }]
      : []),
    ...(request.entidade_name ? [{ label: t("requests.entidade"), value: request.entidade_name }] : []),
    { label: t("requests.priority"), value: t(`requests.priorityLevels.${request.priority}`) },
    { label: t("requests.createdAt"), value: formatDateTime(request.created_at) },
    { label: t("requests.updatedAt"), value: formatDateTime(request.updated_at) },
    ...(request.decided_at ? [{ label: t("requests.decidedAt"), value: formatDate(request.decided_at) }] : []),
    ...(request.delivered_at ? [{ label: t("requests.deliveredAt"), value: formatDate(request.delivered_at) }] : []),
  ];

  if (request.fulfilled_asset_id && request.fulfilled_asset_type) {
    const href = assetHref(request.fulfilled_asset_type, request.fulfilled_asset_id);
    const labelKey = assetTypeLabelKey(request.fulfilled_asset_type);
    const label = `${labelKey ? t(labelKey) : request.fulfilled_asset_type} #${request.fulfilled_asset_id}`;
    meta.push({
      label: t("requests.fulfilledAsset"),
      value: href ? <Link href={href} className="text-[var(--accent)] hover:underline">{label}</Link> : label,
    });
  }

  if (request.external_source) {
    const label = request.cached_title || request.external_ref || request.external_source;
    meta.push({
      label: t("requests.externalTicket"),
      value: (
        <span className="flex flex-wrap items-center gap-1.5">
          {request.external_url ? (
            <a href={request.external_url} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] hover:underline">
              {label}
            </a>
          ) : (
            label
          )}
          {request.cached_status && <Badge color="gray">{request.cached_status}</Badge>}
        </span>
      ),
    });
  }

  return (
    <PageShell>
      <div className="space-y-5">
        <DetailHeader
          backHref="/requests"
          backLabel={t("common.back")}
          title={request.title}
          subtitle={offering?.name ?? request.offering_name}
          badges={
            <>
              <Badge color={statusColor(request.status)}>{t(statusLabelKey(request.status))}</Badge>
              <Badge color="gray">{t(`requests.priorityLevels.${request.priority}`)}</Badge>
            </>
          }
        />

        {(moves.length > 0 || can.edit) && (
          <div className="flex flex-wrap gap-2">
            {moves.map((to) => (
              <Button key={to} size="sm" variant={transitionVariant(to)} onClick={() => setPendingTo(to)}>
                {t(TRANSITION_ACTION_KEY[to])}
              </Button>
            ))}
            {can.edit && (
              <Button size="sm" variant="secondary" onClick={() => setEditing(true)}>
                {t("common.edit")}
              </Button>
            )}
          </div>
        )}

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5 items-start">
          <div className="lg:col-span-2 space-y-5">
            <section>
              <SectionHeading>{t("requests.answers")}</SectionHeading>
              <Card accent="none">
                <RequestAnswers schema={request.form_schema_snapshot} data={request.form_data} />
              </Card>
            </section>

            <section>
              <SectionHeading>{t("requests.activity")}</SectionHeading>
              <Card accent="none" className="space-y-5">
                <RequestTimeline events={events} />

                <div className="space-y-2 border-t border-[var(--border-subtle)] pt-4">
                  <Textarea
                    label={t("requests.addComment")}
                    value={comment}
                    onChange={(e) => {
                      setComment(e.target.value);
                      if (commentError) setCommentError("");
                    }}
                    rows={3}
                  />
                  {commentError && <StatusAlert variant="error">{commentError}</StatusAlert>}
                  <div className="flex justify-end">
                    <Button
                      size="sm"
                      onClick={() => commentMutation.mutate()}
                      loading={commentMutation.isPending}
                      disabled={!comment.trim() || commentMutation.isPending}
                    >
                      {t("requests.actions.comment")}
                    </Button>
                  </div>
                </div>
              </Card>
            </section>
          </div>

          <aside>
            <SectionHeading>{t("requests.details")}</SectionHeading>
            <Card accent="none">
              <dl className="space-y-3">
                {meta.map((row) => (
                  <div key={row.label} className="flex flex-wrap justify-between gap-2">
                    <dt className="text-xs font-medium text-[var(--text-muted)] tracking-wide">{row.label}</dt>
                    <dd className="text-sm text-[var(--text-primary)] text-right min-w-0">{row.value}</dd>
                  </div>
                ))}
              </dl>
            </Card>
          </aside>
        </div>

        <TransitionModal
          requestId={id}
          to={pendingTo}
          onClose={() => setPendingTo(null)}
          onDone={refresh}
        />
        <RequestEditModal
          request={editing ? request : null}
          onClose={() => setEditing(false)}
          onDone={refresh}
        />
      </div>
    </PageShell>
  );
}
