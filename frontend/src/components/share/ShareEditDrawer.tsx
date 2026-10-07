"use client";

import { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiCatalogAPI, outlineAPI, secretsAPI, shareBundlesAPI } from "@/lib/api";
import type { ShareBundleView } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import NativeSelect from "@/components/ui/NativeSelect";
import RadioGroup from "@/components/ui/RadioGroup";
import SectionHeading from "@/components/ui/SectionHeading";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import Icon from "@/components/ui/Icon";
import RecipientField, { type Recipient } from "@/components/share/RecipientField";
import { WikiPickerNodes } from "@/components/atlas/apis/ShareBundleModal";
import { ICON_PATHS } from "@/lib/icon-paths";

type ItemType = "secret" | "api_doc" | "wiki_doc" | "wiki_collection";
type Selector = { mode: string; op_keys?: string[]; tags?: string[] };
/** One item as the edit holds it; label is for display only. */
type Item = { type: ItemType; ref_id?: number; ref_key?: string; selector?: Selector; label: string };

const itemKey = (i: Item) => `${i.type}:${i.ref_id ?? 0}:${i.ref_key ?? ""}`;

// Validity choices: keep the current expiry, a fresh window, or none.
const EXPIRY_DAYS = [1, 7, 30, 90];

/**
 * Edit a share link in place — the URL never changes. Details (title,
 * description, recipient, passphrase), validity (expiry, view limit) and
 * contents (secrets, APIs, wiki pages) are saved through their own endpoints,
 * only the parts that changed.
 */
export default function ShareEditDrawer({ bundle, onClose }: { bundle: ShareBundleView | null; onClose: () => void }) {
  // Keyed by the link: opening another one starts a fresh form.
  return bundle ? <EditForm key={bundle.id} bundle={bundle} onClose={onClose} /> : null;
}

function EditForm({ bundle, onClose }: { bundle: ShareBundleView; onClose: () => void }) {
  const { t, formatDateTime } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const open = true;

  const [title, setTitle] = useState(bundle.title ?? "");
  const [description, setDescription] = useState(bundle.description ?? "");
  const [recipient, setRecipient] = useState<Recipient>({ contactId: bundle.recipient_contact_id ?? null, label: bundle.recipient_label ?? "" });
  const [passMode, setPassMode] = useState<"keep" | "set" | "remove">("keep");
  const [passphrase, setPassphrase] = useState("");
  const [expiry, setExpiry] = useState("keep"); // "keep" | "never" | "<days>"
  const [maxViews, setMaxViews] = useState(bundle.max_views != null ? String(bundle.max_views) : "");
  const [items, setItems] = useState<Item[]>(() =>
    bundle.items.map((i) => ({
      type: i.type,
      ref_id: i.ref_id || undefined,
      ref_key: i.ref_key || undefined,
      selector: i.selector ? (JSON.parse(i.selector) as Selector) : undefined,
      label: i.label,
    })),
  );
  const [addSecret, setAddSecret] = useState("");
  const [addApi, setAddApi] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const { data: secrets = [] } = useQuery({ queryKey: ["secrets", "shareable"], queryFn: () => secretsAPI.list(), enabled: open });
  const { data: apis = [] } = useQuery({ queryKey: ["api-catalog", "share-edit"], queryFn: () => apiCatalogAPI.list(), enabled: open });
  const { data: wikiTree } = useQuery({ queryKey: ["wiki-tree", "share-picker"], queryFn: outlineAPI.commonWikiTree, enabled: open, retry: false });
  const wikiSections = useMemo(() => wikiTree?.sections ?? [], [wikiTree]);

  const has = useMemo(() => new Set(items.map(itemKey)), [items]);
  const wikiDocs = useMemo(() => new Set(items.filter((i) => i.type === "wiki_doc").map((i) => i.ref_key!)), [items]);
  const wikiCols = useMemo(() => new Set(items.filter((i) => i.type === "wiki_collection").map((i) => i.ref_key!)), [items]);
  // Wiki titles for items added in this edit (existing ones carry their label).
  const wikiTitle = useMemo(() => {
    const m = new Map<string, string>();
    type Node = { id: string; title: string; children?: Node[] };
    const walk = (ns: Node[]) => ns.forEach((n) => { m.set(n.id, n.title); walk(n.children ?? []); });
    for (const s of wikiSections) {
      if (s.collection) m.set(s.collection_id, s.collection.name);
      walk((s.nodes ?? []) as Node[]);
    }
    return m;
  }, [wikiSections]);

  const remove = (i: Item) => setItems((cur) => cur.filter((x) => itemKey(x) !== itemKey(i)));
  const add = (i: Item) => setItems((cur) => (has.has(itemKey(i)) ? cur : [...cur, i]));
  const toggleWiki = (type: "wiki_doc" | "wiki_collection", id: string) => {
    const i: Item = { type, ref_key: id, label: wikiTitle.get(id) ?? id };
    if (has.has(itemKey(i))) remove(i);
    else add(i);
  };

  const groupLabel = (type: ItemType) =>
    t(type === "secret" ? "shares.group.secret" : type === "api_doc" ? "shares.group.api" : "shares.group.wiki");

  async function save() {
    setError("");
    if (items.length === 0) return setError(t("shares.edit.needItem"));
    if (passMode === "set" && passphrase.trim() === "") return setError(t("shares.edit.needPassphrase"));
    const mv = maxViews.trim() === "" ? null : Number(maxViews);
    if (mv !== null && (!Number.isInteger(mv) || mv < 1)) return setError(t("shares.edit.badMaxViews"));

    // Only the parts that changed go out.
    const details: Record<string, unknown> = {};
    if (title !== (bundle.title ?? "")) details.title = title;
    if (description !== (bundle.description ?? "")) details.description = description;
    if ((recipient.contactId ?? 0) !== (bundle.recipient_contact_id ?? 0)) details.recipient_contact_id = recipient.contactId ?? 0;
    if (recipient.label !== (bundle.recipient_label ?? "")) details.recipient_label = recipient.label;
    if (passMode === "set") details.passphrase = passphrase;
    if (passMode === "remove") details.passphrase = "";

    const before = bundle.items.map((i) => `${i.type}:${i.ref_id || 0}:${i.ref_key ?? ""}`).sort().join("|");
    const itemsChanged = before !== items.map(itemKey).sort().join("|");

    const viewsChanged = mv !== (bundle.max_views ?? null);
    let ttl: number | undefined;
    if (expiry === "never") ttl = -1;
    else if (expiry !== "keep") ttl = Number(expiry) * 86400;
    else if (viewsChanged) {
      // Renew always resets the expiry: keep the current one by sending what's left of it.
      ttl = bundle.expires_at ? Math.max(60, Math.round((Date.parse(bundle.expires_at) - Date.now()) / 1000)) : -1;
    }

    setSaving(true);
    try {
      if (Object.keys(details).length > 0) await shareBundlesAPI.updateDetails(bundle.id, details);
      if (itemsChanged) {
        await shareBundlesAPI.updateItems(
          bundle.id,
          items.map((i) => ({ type: i.type, ref_id: i.ref_id, ref_key: i.ref_key, selector: i.selector })),
        );
      }
      if (ttl !== undefined) await shareBundlesAPI.renew(bundle.id, { ttl_seconds: ttl, ...(viewsChanged ? { max_views: mv } : {}) });
      qc.invalidateQueries({ queryKey: ["share-bundles"] });
      flag({ appearance: "success", title: t("shares.edit.saved") });
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  const revived = !!(bundle.revoked_at || bundle.deleted_at);

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("shares.edit.title")}
      subHeader={<span className="text-xs text-[var(--text-muted)]">{t("shares.edit.sameLink")}</span>}
      wide
      footer={
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose}>{t("common.cancel")}</Button>
          <Button onClick={save} loading={saving}>{t("common.save")}</Button>
        </div>
      }
    >
      <div className="space-y-6">
          <section className="space-y-3">
            <SectionHeading variant="label" as="h3">{t("shares.edit.details")}</SectionHeading>
            <Input label={t("shares.col.title")} value={title} onChange={(e) => setTitle(e.target.value)} />
            <Textarea label={t("shares.edit.description")} value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
            <RecipientField value={recipient} onChange={setRecipient} enabled={open} />
            <RadioGroup
              label={t("shares.passphrase")}
              name="share-passphrase"
              value={passMode}
              onChange={(v) => setPassMode(v as "keep" | "set" | "remove")}
              options={[
                { value: "keep", label: t(bundle.has_passphrase ? "shares.edit.passKeep" : "shares.edit.passNone") },
                { value: "set", label: t(bundle.has_passphrase ? "shares.edit.passChange" : "shares.edit.passSet") },
                ...(bundle.has_passphrase ? [{ value: "remove", label: t("shares.edit.passRemove") }] : []),
              ]}
            />
            {passMode === "set" && (
              <Input type="password" autoComplete="new-password" label={t("shares.edit.newPassphrase")} value={passphrase} onChange={(e) => setPassphrase(e.target.value)} />
            )}
          </section>

          <section className="space-y-3">
            <SectionHeading variant="label" as="h3">{t("shares.edit.validity")}</SectionHeading>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <NativeSelect label={t("shares.col.expires")} value={expiry} onChange={(e) => setExpiry(e.target.value)}>
                <option value="keep">
                  {t("shares.edit.expiryKeep", { when: bundle.expires_at ? formatDateTime(bundle.expires_at) : t("shares.never") })}
                </option>
                {EXPIRY_DAYS.map((d) => (
                  <option key={d} value={String(d)}>{d === 1 ? t("shares.edit.expiryDay") : t("shares.edit.expiryDays", { n: String(d) })}</option>
                ))}
                <option value="never">{t("shares.never")}</option>
              </NativeSelect>
              <Input
                type="number"
                min={1}
                label={t("shares.edit.maxViews")}
                hint={t("shares.edit.maxViewsHint")}
                value={maxViews}
                onChange={(e) => setMaxViews(e.target.value)}
              />
            </div>
            {revived && <StatusAlert variant="warning">{t("shares.edit.reactivates")}</StatusAlert>}
          </section>

          <section className="space-y-3">
            <SectionHeading variant="label" as="h3" count={items.length}>{t("shares.edit.contents")}</SectionHeading>
            <ul className="space-y-1">
              {items.map((i) => (
                <li key={itemKey(i)} className="flex items-center gap-2 text-sm rounded-[var(--radius-md)] border border-[var(--border-subtle)] px-2 py-1.5">
                  <span className="text-2xs px-1.5 py-0.5 rounded border border-[var(--border-default)] bg-[var(--bg-overlay)] text-[var(--text-muted)] shrink-0">
                    {groupLabel(i.type)}
                  </span>
                  <span className="truncate text-[var(--text-secondary)]">{i.label}</span>
                  {i.type === "api_doc" && i.selector && i.selector.mode !== "all" && (
                    <span className="text-2xs text-[var(--text-faint)] shrink-0">{t("shares.edit.partialApi")}</span>
                  )}
                  <button
                    type="button"
                    onClick={() => remove(i)}
                    aria-label={t("shares.edit.removeItem", { name: i.label })}
                    className="ml-auto rounded p-1 text-[var(--text-muted)] hover:text-[var(--danger)] hover:bg-[var(--bg-elevated)]"
                  >
                    <Icon path={ICON_PATHS.close} className="w-3.5 h-3.5" />
                  </button>
                </li>
              ))}
            </ul>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 items-end">
              <div className="flex items-end gap-2">
                <div className="flex-1 min-w-0">
                  <NativeSelect label={t("shares.edit.addSecret")} value={addSecret} onChange={(e) => setAddSecret(e.target.value)}>
                    <option value="">—</option>
                    {secrets
                      .filter((s) => !has.has(`secret:${s.id}:`))
                      .map((s) => <option key={s.id} value={s.id}>{`${s.name} (${s.type})`}</option>)}
                  </NativeSelect>
                </div>
                <Button
                  variant="secondary"
                  disabled={!addSecret}
                  onClick={() => {
                    const s = secrets.find((x) => String(x.id) === addSecret);
                    if (s) add({ type: "secret", ref_id: s.id, label: s.name });
                    setAddSecret("");
                  }}
                >
                  {t("common.add")}
                </Button>
              </div>
              <div className="flex items-end gap-2">
                <div className="flex-1 min-w-0">
                  <NativeSelect label={t("shares.edit.addApi")} value={addApi} onChange={(e) => setAddApi(e.target.value)}>
                    <option value="">—</option>
                    {apis
                      .filter((a) => !has.has(`api_doc:${a.id}:`))
                      .map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
                  </NativeSelect>
                </div>
                <Button
                  variant="secondary"
                  disabled={!addApi}
                  onClick={() => {
                    const a = apis.find((x) => String(x.id) === addApi);
                    if (a) add({ type: "api_doc", ref_id: a.id, selector: { mode: "all" }, label: a.name });
                    setAddApi("");
                  }}
                >
                  {t("common.add")}
                </Button>
              </div>
            </div>
            <p className="text-xs text-[var(--text-muted)]">{t("shares.edit.apiWhole")}</p>

            {wikiSections.length > 0 && (
              <div>
                <p className="text-sm mb-1.5 text-[var(--text-secondary)]">{t("shares.edit.addWiki")}</p>
                <div className="max-h-56 overflow-y-auto space-y-2 border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2">
                  {wikiSections.map((section) => (
                    <div key={section.collection_id}>
                      <label className="flex items-center gap-2 text-xs font-semibold text-[var(--text-secondary)]">
                        <input type="checkbox" checked={wikiCols.has(section.collection_id)} onChange={() => toggleWiki("wiki_collection", section.collection_id)} />
                        <span className="truncate">{section.collection?.name ?? section.collection_id}</span>
                        <span className="text-3xs font-normal text-[var(--text-faint)]">{t("share.wikiCollection")}</span>
                      </label>
                      {!wikiCols.has(section.collection_id) && (section.nodes?.length ?? 0) > 0 && (
                        <div className="pl-4 mt-0.5">
                          <WikiPickerNodes nodes={section.nodes ?? []} selected={wikiDocs} onToggle={(id) => toggleWiki("wiki_doc", id)} />
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            )}
          </section>

          {error && <FormError message={error} />}
      </div>
    </Drawer>
  );
}
