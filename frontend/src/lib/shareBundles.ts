import type { ShareBundleItemView, ShareBundleView } from "./types";

export type BundleStatus = "live" | "revoked" | "archived" | "expired" | "exhausted";
export const BUNDLE_STATUSES: BundleStatus[] = ["live", "revoked", "archived", "expired", "exhausted"];

/** Why a link does or doesn't open, most decisive reason first (revoked beats expired). */
export function bundleStatus(
  b: Pick<ShareBundleView, "revoked_at" | "deleted_at" | "expires_at" | "max_views" | "view_count">,
  now: number = Date.now(),
): BundleStatus {
  if (b.revoked_at) return "revoked";
  if (b.deleted_at) return "archived";
  if (b.expires_at && new Date(b.expires_at).getTime() < now) return "expired";
  if (b.max_views != null && b.view_count >= b.max_views) return "exhausted";
  return "live";
}

export type ItemGroup = "api" | "wiki" | "secret";

/** An item's group on the shares page: API docs, wiki pages/collections, secrets. */
export const itemGroup = (it: Pick<ShareBundleItemView, "type">): ItemGroup =>
  it.type === "api_doc" ? "api" : it.type === "secret" ? "secret" : "wiki";

/** Item labels by group, in a fixed order; empty groups are left out. */
export function groupItems(items: ShareBundleItemView[]): { group: ItemGroup; labels: string[] }[] {
  return (["api", "wiki", "secret"] as const)
    .map((group) => ({ group, labels: items.filter((it) => itemGroup(it) === group).map((it) => it.label) }))
    .filter((g) => g.labels.length > 0);
}

/** Who the link is for, as one line: contact name and/or free text. */
export function recipientText(b: Pick<ShareBundleView, "recipient_name" | "recipient_label">): string {
  return [b.recipient_name, b.recipient_label].filter(Boolean).join(" · ");
}
