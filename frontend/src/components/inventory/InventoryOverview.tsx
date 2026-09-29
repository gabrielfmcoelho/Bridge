import type { ReactNode } from "react";

/**
 * A listing page's Visão geral: indicators as a compact grid — the sticky
 * left column on desktop, a strip above the listing below lg — and the
 * listing in the rest. Hosts, DNS and Serviços share it.
 */
export default function InventoryOverview({ kpis, children }: { kpis: ReactNode; children: ReactNode }) {
  return (
    <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,3fr)] gap-6 max-lg:space-y-6">
      <aside className="lg:sticky lg:top-0 lg:self-start">{kpis}</aside>
      <div className="min-w-0">{children}</div>
    </div>
  );
}
