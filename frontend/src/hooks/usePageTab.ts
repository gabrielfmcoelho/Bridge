import { useEffect, useState } from "react";

/** A listing page's Visão geral / Dashboard tab (plus any `extra` tabs the
 *  page has), kept in ?tab= so a link survives reload (replaceState, no
 *  navigation). */
export function usePageTab<E extends string = never>(extra: readonly E[] = []) {
  type Tab = "overview" | "dashboard" | E;
  const known = (k: string | null): k is Tab => k === "dashboard" || (extra as readonly string[]).includes(k ?? "");
  const [tab, setTab] = useState<Tab>("overview");
  useEffect(() => {
    // Reads the URL once after hydration.
    const k = new URLSearchParams(window.location.search).get("tab");
    if (known(k)) setTab(k);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once, after hydration
  }, []);
  const select = (k: string) => {
    const next: Tab = known(k) ? k : "overview";
    setTab(next);
    const url = new URL(window.location.href);
    if (next !== "overview") url.searchParams.set("tab", next);
    else url.searchParams.delete("tab");
    window.history.replaceState(window.history.state, "", url.pathname + url.search);
  };
  return [tab, select] as const;
}
