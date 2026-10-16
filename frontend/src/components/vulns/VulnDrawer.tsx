"use client";

import { useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Badge from "@/components/ui/Badge";
import Lozenge from "@/components/ui/Lozenge";
import Field from "@/components/ui/Field";
import SectionHeading from "@/components/ui/SectionHeading";
import NativeSelect from "@/components/ui/NativeSelect";
import Textarea from "@/components/ui/Textarea";
import Button from "@/components/ui/Button";
import FormError from "@/components/ui/FormError";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import { Skeleton } from "@/components/ui/Skeleton";
import StatusAlert from "@/components/ui/StatusAlert";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import { dnsAPI, hostsAPI, projectsAPI, servicesAPI, vulnAPI } from "@/lib/api";
import { VULN_SEVERITY_BADGE, VULN_STATUS_LOZENGE } from "@/lib/alert-colors";
import { linkifyParts, nvdURL, safeHref } from "@/lib/vulns";
import type { VulnEntityType, VulnLink, VulnStatus } from "@/lib/types";

const STATUSES: VulnStatus[] = ["open", "accepted", "false_positive", "resolved"];
const ENTITY_TYPES: VulnEntityType[] = ["host", "dns", "service", "project"];

function entityHref(l: VulnLink): string {
  switch (l.entity_type) {
    case "host": return `/hosts/${l.slug}`;
    case "dns": return `/dns/${l.entity_id}`;
    case "service": return `/services/${l.entity_id}`;
    case "project": return `/projects/${l.entity_id}`;
  }
}

const pickerFetchers: Record<VulnEntityType, (q: string) => Promise<AsyncPickerItem[]>> = {
  host: (q) => hostsAPI.listPaginated({ search: q, per_page: "20" }).then((r) => r.data.map((h) => ({ id: h.id, label: h.nickname, secondary: h.hostname }))),
  dns: (q) => dnsAPI.listPaginated({ search: q, per_page: "20" }).then((r) => r.data.map((d) => ({ id: d.id, label: d.domain }))),
  service: (q) => servicesAPI.listPaginated({ search: q, per_page: "20" }).then((r) => r.data.map((s) => ({ id: s.id, label: s.nickname }))),
  project: (q) => projectsAPI.listPaginated({ search: q, per_page: "20" }).then((r) => r.data.map((p) => ({ id: p.id, label: p.name }))),
};

/** Scanner text with only its http(s) URLs as links. Payloads stay text. */
function LinkText({ text, mono }: { text: string; mono?: boolean }) {
  return (
    <p className={`text-sm text-[var(--text-secondary)] whitespace-pre-wrap break-all ${mono ? "font-mono text-xs" : ""}`}>
      {linkifyParts(text).map((p, i) =>
        p.href ? (
          <a key={i} href={p.href} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] hover:underline">{p.text}</a>
        ) : (
          <span key={i}>{p.text}</span>
        ),
      )}
    </p>
  );
}

/** One finding: everything the scanner said, its links, and its status. */
export default function VulnDrawer({ id, onClose }: { id: number | null; onClose: () => void }) {
  const { t, formatDateTime } = useLocale();
  const { user } = useAuth();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const queryClient = useQueryClient();

  const { data: f, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "detail", id],
    queryFn: () => vulnAPI.get(id!),
    enabled: id != null,
  });

  const [linkType, setLinkType] = useState<VulnEntityType>("host");
  const [formError, setFormError] = useState("");

  const refresh = () => {
    setFormError("");
    return queryClient.invalidateQueries({ queryKey: ["vulnerabilities"] });
  };
  const onFail = (e: Error) => setFormError(e.message);

  const addLink = useMutation({
    mutationFn: (entityID: number) => vulnAPI.addLink(id!, { entity_type: linkType, entity_id: entityID }),
    onSuccess: refresh,
    onError: onFail,
  });
  const removeLink = useMutation({
    mutationFn: (l: VulnLink) => vulnAPI.removeLink(id!, { entity_type: l.entity_type, entity_id: l.entity_id }),
    onSuccess: refresh,
    onError: onFail,
  });

  const cveHref = f ? nvdURL(f.cve) : null;
  const refHref = f ? safeHref(f.reference) : null;

  return (
    <Drawer
      open={id != null}
      onClose={onClose}
      wide
      title={f?.title ?? t("vuln.finding")}
      subHeader={
        f && (
          <div className="flex flex-wrap items-center gap-2">
            <Badge color={VULN_SEVERITY_BADGE[f.severity]}>{t(`vuln.severityLevel.${f.severity}`)}</Badge>
            <Lozenge appearance={VULN_STATUS_LOZENGE[f.status]}>{t(`vuln.status.${f.status}`)}</Lozenge>
            {f.known_exploited && <Badge color="danger">{t("vuln.knownExploited")}</Badge>}
          </div>
        )
      }
    >
      {isLoading ? (
        <div className="space-y-3"><Skeleton className="h-5 w-2/3" /><Skeleton className="h-24 w-full" /></div>
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>
      ) : f ? (
        <div className="space-y-6">
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("vuln.target")} value={f.target || "—"} mono wrap />
            <Field label={t("vuln.vm")} value={f.vm_hint || "—"} mono />
            <Field label={t("vuln.cve")} value={f.cve || "—"} mono href={cveHref ?? undefined} />
            <Field label={t("vuln.cvss")} value={f.cvss != null ? String(f.cvss) : "—"} />
            <Field label={t("vuln.cwe")} value={f.cwe || "—"} mono />
            <Field label={t("vuln.scanner")} value={`${f.scanner || "—"} · ${f.source.toUpperCase()}`} />
            <Field label={t("vuln.firstSeen")} value={formatDateTime(f.first_seen_at)} />
            <Field label={t("vuln.lastSeen")} value={formatDateTime(f.last_seen_at)} />
            {f.resolved_at && <Field label={t("vuln.resolvedAt")} value={formatDateTime(f.resolved_at)} />}
          </div>

          {f.risk && (
            <section className="space-y-1.5">
              <SectionHeading>{t("vuln.risk")}</SectionHeading>
              <LinkText text={f.risk} />
            </section>
          )}
          {f.remediation && (
            <section className="space-y-1.5">
              <SectionHeading>{t("vuln.remediation")}</SectionHeading>
              <LinkText text={f.remediation} />
            </section>
          )}
          {f.evidence && (
            <section className="space-y-1.5">
              <SectionHeading>{t("vuln.evidence")}</SectionHeading>
              <LinkText text={f.evidence} mono />
            </section>
          )}
          {f.reference && (
            <section className="space-y-1.5">
              <SectionHeading>{t("vuln.reference")}</SectionHeading>
              {refHref ? <LinkText text={f.reference} /> : <p className="text-sm text-[var(--text-secondary)] break-all">{f.reference}</p>}
            </section>
          )}

          <section className="space-y-2">
            <SectionHeading count={f.links?.length ?? 0}>{t("vuln.links")}</SectionHeading>
            {(f.links ?? []).length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{t("vuln.unmatchedHint")}</p>
            ) : (
              <ul className="space-y-1">
                {(f.links ?? []).map((l) => (
                  <li key={`${l.entity_type}-${l.entity_id}`} className="flex items-center justify-between gap-2 text-sm">
                    <span className="min-w-0 truncate">
                      <span className="text-[var(--text-muted)]">{t(`vuln.entity.${l.entity_type}`)} · </span>
                      <Link href={entityHref(l)} className="text-[var(--accent)] hover:underline">{l.name}</Link>
                      <span className="text-xs text-[var(--text-faint)]"> · {t(`vuln.origin.${l.origin}`)}</span>
                    </span>
                    {canEdit && (
                      <Button size="sm" variant="ghost" onClick={() => removeLink.mutate(l)} disabled={removeLink.isPending}>
                        {t("common.remove")}
                      </Button>
                    )}
                  </li>
                ))}
              </ul>
            )}
            {canEdit && (
              <div className="flex items-end gap-2">
                <NativeSelect value={linkType} onChange={(e) => setLinkType(e.target.value as VulnEntityType)} aria-label={t("vuln.linkType")}>
                  {ENTITY_TYPES.map((et) => <option key={et} value={et}>{t(`vuln.entity.${et}`)}</option>)}
                </NativeSelect>
                <div className="flex-1">
                  <AsyncPicker
                    key={linkType}
                    value={null}
                    fetcher={pickerFetchers[linkType]}
                    placeholder={t("vuln.addLink")}
                    onChange={(item) => item && addLink.mutate(item.id)}
                  />
                </div>
              </div>
            )}
          </section>

          {formError && <FormError message={formError} />}
          {canEdit && (
            // Keyed on the saved values: a reload after saving starts a fresh form.
            <TriageForm key={`${f.id}|${f.status}|${f.status_note}`} id={f.id} status={f.status} note={f.status_note} onSaved={refresh} />
          )}
          {!canEdit && f.status_note && (
            <section className="space-y-1.5">
              <SectionHeading>{t("vuln.note")}</SectionHeading>
              <p className="text-sm text-[var(--text-secondary)] whitespace-pre-wrap">{f.status_note}</p>
            </section>
          )}
        </div>
      ) : null}
    </Drawer>
  );
}

/** Status + note of one finding. Initial values come from props; the
 *  parent remounts it (key) when the saved values change. */
function TriageForm({ id, status: initialStatus, note: initialNote, onSaved }: { id: number; status: VulnStatus; note: string; onSaved: () => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const [status, setStatus] = useState(initialStatus);
  const [note, setNote] = useState(initialNote);
  const [error, setError] = useState("");
  const save = useMutation({
    mutationFn: () => vulnAPI.update(id, { status, status_note: note }),
    onSuccess: () => {
      onSaved();
      flag({ appearance: "success", title: t("vuln.saved") });
    },
    onError: (e: Error) => setError(e.message),
  });
  const dirty = status !== initialStatus || note !== initialNote;
  return (
    <section className="space-y-3">
      <SectionHeading>{t("vuln.triage")}</SectionHeading>
      <NativeSelect label={t("common.status")} value={status} onChange={(e) => setStatus(e.target.value as VulnStatus)}>
        {STATUSES.map((s) => <option key={s} value={s}>{t(`vuln.status.${s}`)}</option>)}
      </NativeSelect>
      <Textarea label={t("vuln.note")} hint={t("vuln.noteHint")} value={note} onChange={(e) => setNote(e.target.value)} rows={3} />
      {error && <FormError message={error} />}
      <Button onClick={() => save.mutate()} loading={save.isPending} disabled={!dirty}>{t("common.save")}</Button>
    </section>
  );
}
