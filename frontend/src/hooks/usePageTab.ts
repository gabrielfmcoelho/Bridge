import { useEffect, useState } from "react";

/** A listing page's Visão geral / Dashboard tab, kept in ?tab= so a
 *  dashboard link survives reload (replaceState, no navigation). */
export function usePageTab() {
  const [tab, setTab] = useState<"overview" | "dashboard">("overview");
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- reads the URL once after hydration
    if (new URLSearchParams(window.location.search).get("tab") === "dashboard") setTab("dashboard");
  }, []);
  const select = (k: string) => {
    const next = k === "dashboard" ? "dashboard" : "overview";
    setTab(next);
    const url = new URL(window.location.href);
    if (next === "dashboard") url.searchParams.set("tab", "dashboard");
    else url.searchParams.delete("tab");
    window.history.replaceState(window.history.state, "", url.pathname + url.search);
  };
  return [tab, select] as const;
}
