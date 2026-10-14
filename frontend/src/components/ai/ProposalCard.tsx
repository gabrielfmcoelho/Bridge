"use client";

import Link from "next/link";
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { glpiAPI, globalIssuesAPI } from "@/lib/api";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import Button from "@/components/ui/Button";
import FormError from "@/components/ui/FormError";
import Icon from "@/components/ui/Icon";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import Textarea from "@/components/ui/Textarea";
import { chatActions, type ProposalState } from "./chatStore";

const PRIORITIES = ["low", "medium", "high", "critical"] as const;

/**
 * An action the assistant drafted. Nothing has happened yet: the user edits
 * the draft and confirms, and only then does the browser call the regular
 * create endpoint, with the user's own permissions and validation.
 */
export default function ProposalCard({ msgId, proposal: p }: { msgId: string; proposal: ProposalState }) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [title, setTitle] = useState(p.title);
  const [description, setDescription] = useState(p.description);
  const [priority, setPriority] = useState(p.priority ?? "medium");
  const resolve = (patch: Partial<ProposalState>) => chatActions.resolveProposal(msgId, p.id, patch);

  const create = useMutation({
    mutationFn: async () => {
      if (p.kind === "issue") {
        const issue = await globalIssuesAPI.create({ title, description, priority, entity_type: p.parent_type, entity_id: p.parent_id });
        void qc.invalidateQueries({ queryKey: ["issues"] });
        return { label: t("ai.proposal.issueCreated", { id: String(issue.id) }), href: "/issues" };
      }
      const r = await glpiAPI.createTicket({ profile_id: p.profile!.id, title, description, host_slug: p.host_slug || undefined });
      return { label: t("ai.proposal.chamadoCreated", { id: String(r.ticket_id) }), href: r.ticket_url };
    },
    onSuccess: (result) => resolve({ status: "done", title, description, priority, result }),
  });

  const kindLabel = t(p.kind === "issue" ? "ai.proposal.issueDraft" : "ai.proposal.chamadoDraft");
  const icon = p.kind === "issue" ? NAV_ICONS.ListChecks : NAV_ICONS.Ticket;

  if (p.status !== "pending") {
    return (
      <div className="flex items-center gap-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] px-3 py-2 text-sm">
        <Icon path={p.status === "done" ? ICON_PATHS.checkCircle : ICON_PATHS.close} className={`w-4 h-4 ${p.status === "done" ? "text-[var(--success)]" : "text-[var(--text-muted)]"}`} />
        <span className="flex-1 min-w-0 truncate text-[var(--text-secondary)]">
          {p.status === "done" ? p.result?.label : t("ai.proposal.dismissed", { title: p.title })}
        </span>
        {p.status === "done" && p.result?.href && (
          p.result.href.startsWith("/")
            ? <Link href={p.result.href} className="text-xs font-medium text-[var(--accent)] hover:underline">{t("ai.proposal.open")}</Link>
            : <a href={p.result.href} target="_blank" rel="noreferrer" className="text-xs font-medium text-[var(--accent)] hover:underline">{t("ai.proposal.open")}</a>
        )}
      </div>
    );
  }

  return (
    <section aria-label={kindLabel} className="rounded-[var(--radius-lg)] border border-[var(--accent)]/30 bg-[var(--accent)]/[0.04]">
      <header className="flex items-center gap-2 border-b border-[var(--border-subtle)] px-4 py-2.5">
        <Icon path={icon} className="w-4 h-4 text-[var(--accent)]" strokeWidth={1.5} />
        <span className="text-sm font-medium text-[var(--text-primary)]">{kindLabel}</span>
        <span className="ml-auto text-2xs text-[var(--text-muted)]">{t("ai.proposal.notCreatedYet")}</span>
      </header>
      <div className="space-y-3 px-4 py-3">
        <Input label={t("ai.proposal.title")} value={title} onChange={(e) => setTitle(e.target.value)} required />
        <Textarea label={t("ai.proposal.description")} value={description} onChange={(e) => setDescription(e.target.value)} rows={5} />
        {p.kind === "issue" ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <NativeSelect label={t("ai.proposal.priority")} value={priority} onChange={(e) => setPriority(e.target.value)}>
              {PRIORITIES.map((v) => <option key={v} value={v}>{t(`issue.${v}`)}</option>)}
            </NativeSelect>
            {p.parent && (
              <div>
                <p className="mb-1.5 text-xs font-medium text-[var(--text-secondary)]">{t("ai.proposal.parent")}</p>
                <Link href={p.parent.href} className="text-sm text-[var(--accent)] hover:underline">{p.parent.label}</Link>
              </div>
            )}
          </div>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">
            {t("ai.proposal.glpiProfile", { name: p.profile?.name ?? "–" })}
            {p.host_slug && <> · {t("ai.proposal.linkedHost", { host: p.host_slug })}</>}
          </p>
        )}
        {create.error && <FormError message={create.error.message} />}
      </div>
      <footer className="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] px-4 py-2.5">
        <Button size="sm" variant="ghost" onClick={() => resolve({ status: "dismissed" })} disabled={create.isPending}>
          {t("ai.proposal.dismiss")}
        </Button>
        <Button size="sm" onClick={() => create.mutate()} loading={create.isPending} disabled={!title.trim()}>
          {t(p.kind === "issue" ? "ai.proposal.createIssue" : "ai.proposal.openChamado")}
        </Button>
      </footer>
    </section>
  );
}
