"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { contactsAPI, shareBundlesAPI } from "@/lib/api";
import type { ShareSendResult } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import Modal from "@/components/ui/Modal";
import TagInput from "@/components/ui/TagInput";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import StatusAlert from "@/components/ui/StatusAlert";

export interface SendTarget {
  id: number;
  title: string;
  has_passphrase: boolean;
  /** The recipient contact; its email is the default address. */
  recipient_contact_id?: number | null;
  /** That contact's email when already known (the /shares list carries it). */
  recipient_email?: string;
}

/** ShareSendForm in a modal — the /shares page's "send by email" action. */
export default function ShareSendModal({ target, onClose }: { target: SendTarget | null; onClose: () => void }) {
  const { t } = useLocale();
  return (
    <Modal open={target != null} onClose={onClose} title={t("shares.send.title", { title: target ? target.title || `#${target.id}` : "" })}>
      {/* Keyed so each opening starts from that bundle's default address. */}
      {target && <ShareSendForm key={target.id} target={target} onDone={onClose} onCancel={onClose} />}
    </Modal>
  );
}

/**
 * Email a share link: the server mails link + passphrase together, one message
 * per address. Starts with the recipient contact's email; any contact (as
 * "Name <email>") or a typed address can be added. Every send is audited.
 */
export function ShareSendForm({ target, onDone, onCancel }: { target: SendTarget; onDone: () => void; onCancel: () => void }) {
  const { t } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const { data: contacts = [] } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const suggestions = contacts.filter((c) => c.email).map((c) => `${c.name} <${c.email}>`);
  const defaultEmail = target.recipient_email || contacts.find((c) => c.id === target.recipient_contact_id)?.email;
  // null = untouched: follow the default, which may arrive with the contacts.
  const [edited, setEdited] = useState<string[] | null>(null);
  const emails = edited ?? (defaultEmail ? [defaultEmail] : []);
  const [failed, setFailed] = useState<ShareSendResult[]>([]);

  const send = useMutation({
    mutationFn: () => shareBundlesAPI.send(target.id, emails),
    onSuccess: ({ results }) => {
      qc.invalidateQueries({ queryKey: ["share-bundle-access-log", target.id] });
      const bad = results.filter((r) => !r.sent);
      const ok = results.length - bad.length;
      if (ok > 0) flag({ appearance: "success", title: t("shares.send.sentTitle"), description: t("shares.send.sentCount", { count: String(ok) }) });
      setFailed(bad);
      if (bad.length === 0) onDone();
      // Keep only what failed, so sending again retries just those.
      else setEdited(bad.map((r) => r.email));
    },
  });

  return (
    <div className="space-y-4">
      <p className="text-sm text-[var(--text-secondary)]">{t("shares.send.hint")}</p>
      <TagInput
        label={t("shares.send.to")}
        tags={emails}
        onChange={setEdited}
        suggestions={suggestions}
        placeholder={t("shares.send.toPlaceholder")}
      />
      {!target.has_passphrase && <StatusAlert variant="warning">{t("shares.send.noPassphrase")}</StatusAlert>}
      {failed.length > 0 && (
        <StatusAlert variant="error">
          <ul className="space-y-0.5">
            {failed.map((r) => <li key={r.email}><span className="font-mono">{r.email}</span>: {r.error}</li>)}
          </ul>
        </StatusAlert>
      )}
      {send.isError && <FormError message={send.error.message} />}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" type="button" onClick={onCancel}>{t("common.cancel")}</Button>
        <Button type="button" onClick={() => send.mutate()} loading={send.isPending} disabled={emails.length === 0}>
          {t("shares.send.submit")}
        </Button>
      </div>
    </div>
  );
}

/**
 * The "link created" step of the share modals: the URL to copy, the
 * shown-once warning, and sending it by email right there.
 */
export function ShareLinkCreated({ url, target, onClose }: { url: string; target: SendTarget | null; onClose: () => void }) {
  const { t } = useLocale();
  const [sending, setSending] = useState(false);
  return (
    <div className="space-y-3">
      <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.linkCreated")}</p>
      <div className="flex gap-2">
        <Input value={url} readOnly className="font-mono text-xs" />
        <CopyButton value={url} variant="primary" label={t("atlas.apis.copyLink")} copiedLabel={t("atlas.apis.copied")} />
      </div>
      <p className="text-xs text-[var(--warning)]">⚠ {t("atlas.apis.tokenOnce")}</p>
      {sending && target ? (
        <div className="border-t border-[var(--border-subtle)] pt-3">
          <ShareSendForm target={target} onDone={onClose} onCancel={() => setSending(false)} />
        </div>
      ) : (
        <div className="flex justify-end gap-2 pt-2">
          {target && (
            <Button variant="secondary" type="button" onClick={() => setSending(true)}>
              {t("shares.send.action")}
            </Button>
          )}
          <Button variant="secondary" type="button" onClick={onClose}>
            {t("common.close")}
          </Button>
        </div>
      )}
    </div>
  );
}
