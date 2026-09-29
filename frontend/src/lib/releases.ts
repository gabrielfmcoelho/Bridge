import type { Release } from "./types";

export const RELEASE_STATUSES = ["pending", "ongoing", "ready", "live", "canceled"] as const;

/** Status → design token, for the timeline dots and labels. */
export const RELEASE_TONE: Record<string, string> = {
  pending: "var(--text-muted)",
  ongoing: "var(--info)",
  ready: "var(--warning)",
  live: "var(--success)",
  canceled: "var(--danger)",
};

export interface ReleaseLanes<R extends Release = Release> {
  /** Not live yet: nearest target first, undated last. */
  planned: R[];
  /** Live: most recent first. */
  achieved: R[];
  canceled: R[];
}

/** Splits a project's releases into the timeline lanes. */
export function splitReleases<R extends Release>(releases: R[]): ReleaseLanes<R> {
  const planned = releases.filter((r) => r.status !== "live" && r.status !== "canceled");
  const achieved = releases.filter((r) => r.status === "live");
  const canceled = releases.filter((r) => r.status === "canceled");
  // ISO dates (YYYY-MM-DD) compare as plain strings (not localeCompare, which
  // ignores punctuation); "~" sorts an undated release after any date.
  const cmp = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);
  planned.sort((a, b) => cmp(a.target_date || "~", b.target_date || "~"));
  achieved.sort((a, b) => cmp(b.live_date || b.updated_at, a.live_date || a.updated_at));
  return { planned, achieved, canceled };
}

/** A planned release whose target date has passed. */
export function isOverdue(r: Release, today = new Date().toISOString().slice(0, 10)): boolean {
  return r.status !== "live" && r.status !== "canceled" && !!r.target_date && r.target_date < today;
}
