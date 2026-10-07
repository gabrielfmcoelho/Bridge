"use client";

import { Fragment, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import NativeSelect from "@/components/ui/NativeSelect";
import Lozenge, { type LozengeAppearance } from "@/components/ui/Lozenge";
import RowActions from "@/components/ui/RowActions";
import Modal from "@/components/ui/Modal";
import CopyButton from "@/components/ui/CopyButton";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Icon from "@/components/ui/Icon";
import { tableClasses } from "@/components/ui/Table";
import { AccessLogPanel } from "@/components/atlas/apis/ShareBundleModal";
import ShareEditDrawer from "@/components/share/ShareEditDrawer";
import { ApiError, shareBundlesAPI } from "@/lib/api";
import { ICON_PATHS } from "@/lib/icon-paths";
import { BUNDLE_STATUSES, bundleStatus, groupItems, recipientText, type BundleStatus, type ItemGroup } from "@/lib/shareBundles";
import type { ShareBundleReveal, ShareBundleView } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";

const STATUS_TONE: Record<BundleStatus, LozengeAppearance> = {
  live: "success",
  revoked: "removed",
  archived: "default",
  expired: "moved",
  exhausted: "moved",
};

const RENEW_SECONDS = 30 * 86400;

// Links open on the configured public URL when there is one, else on this origin.
const shareOrigin = () => process.env.NEXT_PUBLIC_BASE_URL || window.location.origin;

/**
 * Every share link at a glance: what it carries, who it's for, its state and
 * use. Non-admins see their own links; admins see all of them and can read a
 * link and its passphrase back (audited) or revoke everything at once.
 */
export default function SharesPage() {
  const { t, formatDateTime } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();

  const [status, setStatus] = useState<"" | BundleStatus>("");
  const [group, setGroup] = useState<"" | ItemGroup>("");
  const [recipient, setRecipient] = useState("");
  const [openLog, setOpenLog] = useState<number | null>(null);
  const [revealed, setRevealed] = useState<(ShareBundleReveal & { title: string }) | null>(null);
  const [editing, setEditing] = useState<ShareBundleView | null>(null);

  const { data: bundles = [], isLoading, error } = useQuery({ queryKey: ["share-bundles", "all"], queryFn: shareBundlesAPI.list });
  const refresh = () => qc.invalidateQueries({ queryKey: ["share-bundles"] });
  const onError = (title: string) => (e: Error) => flag({ appearance: "error", title, description: e.message });

  const recipients = useMemo(
    () => Array.from(new Set(bundles.map(recipientText).filter(Boolean))).sort(),
    [bundles],
  );
  const rows = useMemo(() => bundles.filter((b) =>
    (!status || bundleStatus(b) === status)
    && (!group || groupItems(b.items).some((g) => g.group === group))
    && (!recipient || recipientText(b) === recipient),
  ), [bundles, status, group, recipient]);

  const renew = useMutation({
    mutationFn: (id: number) => shareBundlesAPI.renew(id, { ttl_seconds: RENEW_SECONDS }),
    onSuccess: () => { refresh(); flag({ appearance: "success", title: t("shares.renewed") }); },
    onError: onError(t("shares.renew")),
  });
  const revoke = useMutation({
    mutationFn: (id: number) => shareBundlesAPI.revoke(id),
    onSuccess: () => { refresh(); flag({ appearance: "success", title: t("shares.revoked") }); },
    onError: onError(t("shares.revoke")),
  });
  const revokeAll = useMutation({
    mutationFn: shareBundlesAPI.revokeAll,
    onSuccess: (r) => { refresh(); flag({ appearance: "success", title: t("shares.revokeAllDone", { n: String(r.revoked) }) }); },
    onError: onError(t("shares.revokeAll")),
  });

  async function reveal(b: ShareBundleView) {
    if (!(await confirm({ title: t("shares.revealConfirmTitle"), message: t("shares.revealConfirmMessage"), confirmLabel: t("shares.reveal") }))) return;
    try {
      const r = await shareBundlesAPI.reveal(b.id);
      setRevealed({ ...r, url: shareOrigin() + r.url, title: b.title });
      qc.invalidateQueries({ queryKey: ["share-bundle-access-log", b.id] });
    } catch (e) {
      const notRecoverable = e instanceof ApiError && e.status === 409;
      flag({ appearance: "error", title: t("shares.reveal"), description: notRecoverable ? t("shares.notRecoverable") : (e as Error).message });
    }
  }

  async function confirmRevoke(b: ShareBundleView) {
    if (await confirm({ title: t("shares.revokeConfirmTitle", { title: b.title || `#${b.id}` }), danger: true, confirmLabel: t("shares.revoke") })) revoke.mutate(b.id);
  }

  async function confirmRevokeAll() {
    const word = t("shares.revokeAllWord");
    if (await confirm({ title: t("shares.revokeAllConfirmTitle"), message: t("shares.revokeAllConfirmMessage"), danger: true, requireText: word, confirmLabel: t("shares.revokeAll") })) revokeAll.mutate();
  }

  const groupLabel = (g: ItemGroup) => t(`shares.group.${g}`);

  return (
    <PageShell>
      <PageHeader
        title={t("shares.title")}
        description={t(isAdmin ? "shares.descriptionAdmin" : "shares.description")}
        actions={isAdmin ? (
          <Button variant="danger" size="sm" loading={revokeAll.isPending} onClick={confirmRevokeAll}>{t("shares.revokeAll")}</Button>
        ) : undefined}
        controlsKey="shares"
        controlsBadge={[status, group, recipient].filter(Boolean).length}
        controls={
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 max-w-3xl">
            <NativeSelect label={t("shares.filterStatus")} value={status} onChange={(e) => setStatus(e.target.value as "" | BundleStatus)}>
              <option value="">{t("common.all")}</option>
              {BUNDLE_STATUSES.map((s) => <option key={s} value={s}>{t(`shares.status.${s}`)}</option>)}
            </NativeSelect>
            <NativeSelect label={t("shares.filterType")} value={group} onChange={(e) => setGroup(e.target.value as "" | ItemGroup)}>
              <option value="">{t("common.all")}</option>
              {(["api", "wiki", "secret"] as const).map((g) => <option key={g} value={g}>{groupLabel(g)}</option>)}
            </NativeSelect>
            <NativeSelect label={t("shares.filterRecipient")} value={recipient} onChange={(e) => setRecipient(e.target.value)}>
              <option value="">{t("common.all")}</option>
              {recipients.map((r) => <option key={r} value={r}>{r}</option>)}
            </NativeSelect>
          </div>
        }
      />

      {error ? (
        <StatusAlert variant="error">{(error as Error).message}</StatusAlert>
      ) : isLoading ? (
        <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>
      ) : rows.length === 0 ? (
        <EmptyState icon="key" title={t("shares.empty")} description={t("shares.emptyHint")} />
      ) : (
        <div className={tableClasses.wrapper}>
          <table className={tableClasses.table}>
            <thead>
              <tr className={tableClasses.headRow}>
                <th className={tableClasses.th}>{t("shares.col.title")}</th>
                <th className={tableClasses.th}>{t("shares.col.recipient")}</th>
                <th className={tableClasses.th}>{t("shares.col.items")}</th>
                <th className={tableClasses.th}>{t("shares.col.createdBy")}</th>
                <th className={tableClasses.th}>{t("shares.col.created")}</th>
                <th className={tableClasses.th}>{t("shares.col.expires")}</th>
                <th className={tableClasses.th}>{t("shares.col.status")}</th>
                <th className={tableClasses.th}>{t("shares.col.views")}</th>
                <th className={tableClasses.th}>{t("shares.col.lastAccess")}</th>
                <th className={tableClasses.th}><span className="sr-only">{t("common.actions")}</span></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((b, i) => {
                const st = bundleStatus(b);
                return (
                  <Fragment key={b.id}>
                    <tr className={`${tableClasses.row} ${i % 2 ? tableClasses.rowAlt : ""}`}>
                      <td className={tableClasses.td}>
                        <div className="flex items-center gap-1.5 font-medium text-[var(--text-primary)]">
                          {b.title || `#${b.id}`}
                          {b.has_passphrase && (
                            <span title={t("shares.hasPassphrase")} aria-label={t("shares.hasPassphrase")}>
                              <Icon path={ICON_PATHS.lock} className="w-3.5 h-3.5 text-[var(--text-muted)]" />
                            </span>
                          )}
                        </div>
                      </td>
                      <td className={tableClasses.td}>{recipientText(b) || <span className="text-[var(--text-faint)]">—</span>}</td>
                      <td className={`${tableClasses.td} text-xs`}>
                        {groupItems(b.items).map((g) => (
                          <div key={g.group}>
                            <span className="text-[var(--text-muted)]">{groupLabel(g.group)}:</span>{" "}
                            <span className="text-[var(--text-secondary)]">{g.labels.join(", ")}</span>
                          </div>
                        ))}
                      </td>
                      <td className={tableClasses.td}>{b.created_by_name || "—"}</td>
                      <td className={`${tableClasses.td} whitespace-nowrap`}>{formatDateTime(b.created_at)}</td>
                      <td className={`${tableClasses.td} whitespace-nowrap`}>{b.expires_at ? formatDateTime(b.expires_at) : t("shares.never")}</td>
                      <td className={tableClasses.td}><Lozenge appearance={STATUS_TONE[st]}>{t(`shares.status.${st}`)}</Lozenge></td>
                      <td className={tableClasses.td}>{b.view_count}{b.max_views != null ? `/${b.max_views}` : ""}</td>
                      <td className={`${tableClasses.td} whitespace-nowrap`}>{b.last_access_at ? formatDateTime(b.last_access_at) : "—"}</td>
                      <td className={tableClasses.td}>
                        <RowActions
                          name={b.title || `#${b.id}`}
                          actions={[
                            { label: t("common.edit"), icon: ICON_PATHS.edit, onClick: () => setEditing(b) },
                            { label: t("shares.reveal"), icon: ICON_PATHS.eye, onClick: () => reveal(b), hidden: !isAdmin || !b.recoverable },
                            { label: t("shares.renew"), icon: ICON_PATHS.refresh, onClick: () => renew.mutate(b.id) },
                            { label: openLog === b.id ? t("shares.hideLog") : t("shares.accessLog"), icon: ICON_PATHS.clock, onClick: () => setOpenLog((c) => (c === b.id ? null : b.id)) },
                            { label: t("shares.revoke"), icon: ICON_PATHS.xCircle, onClick: () => confirmRevoke(b), danger: true, hidden: st === "revoked" },
                          ]}
                        />
                      </td>
                    </tr>
                    {openLog === b.id && (
                      <tr>
                        <td colSpan={10} className={tableClasses.td}><AccessLogPanel bundleId={b.id} /></td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <ShareEditDrawer bundle={editing} onClose={() => setEditing(null)} />

      <Modal open={revealed != null} onClose={() => setRevealed(null)} title={t("shares.revealTitle", { title: revealed?.title || "" })}>
        {revealed && (
          <div className="space-y-4">
            <div className="flex items-end gap-2">
              <Input label={t("shares.link")} value={revealed.url} readOnly className="font-mono text-xs" />
              <CopyButton value={revealed.url} label={t("common.copy")} copiedLabel={t("common.copied")} />
            </div>
            {revealed.passphrase ? (
              <div className="flex items-end gap-2">
                <Input label={t("shares.passphrase")} value={revealed.passphrase} readOnly className="font-mono text-xs" />
                <CopyButton value={revealed.passphrase} label={t("common.copy")} copiedLabel={t("common.copied")} />
              </div>
            ) : (
              <p className="text-sm text-[var(--text-muted)]">{t("shares.noPassphrase")}</p>
            )}
            <p className="text-xs text-[var(--text-muted)]">{t("shares.revealAudited")}</p>
            <div className="flex justify-end">
              <Button variant="secondary" onClick={() => setRevealed(null)}>{t("common.close")}</Button>
            </div>
          </div>
        )}
      </Modal>
    </PageShell>
  );
}
