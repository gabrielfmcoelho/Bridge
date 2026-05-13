"use client";

import { useMemo, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import type { AtlasIndexes, AtlasFilters, TableRecord } from "@/lib/atlas/types";
import type { TableLayer } from "@/lib/lineage/indexes";
import { getLayerStyle } from "../shared/LayerBadge";

interface Props {
  indexes: AtlasIndexes;
  tables: TableRecord[];      // already filtered
  selectedId: string | null;
  onSelect: (id: string) => void;
  filters: AtlasFilters;
  onFiltersChange: (next: Partial<AtlasFilters>) => void;
  /** When true, table rows are hidden — only the namespace/layer skeleton is shown
   *  (useful when the center pane already shows cards). Clicking a layer node
   *  applies a layer filter. */
  scaffoldOnly?: boolean;
}

interface NamespaceBucket {
  ns: string;
  total: number;
  byLayer: Map<TableLayer, TableRecord[]>;
}

const LAYER_ORDER: TableLayer[] = ["source", "bronze", "silver", "gold", "iapep", "iaspi", "other"];

export default function DomainTree({ indexes, tables, selectedId, onSelect, filters, onFiltersChange, scaffoldOnly }: Props) {
  const { t } = useLocale();

  const buckets = useMemo<NamespaceBucket[]>(() => {
    const m = new Map<string, NamespaceBucket>();
    for (const tb of tables) {
      let b = m.get(tb.namespace);
      if (!b) {
        b = { ns: tb.namespace, total: 0, byLayer: new Map() };
        m.set(tb.namespace, b);
      }
      b.total += 1;
      const arr = b.byLayer.get(tb.layer) ?? [];
      arr.push(tb);
      b.byLayer.set(tb.layer, arr);
    }
    return indexes.namespaces.map(ns => m.get(ns)).filter((b): b is NamespaceBucket => Boolean(b));
  }, [indexes, tables]);

  // Default expansion: all namespaces open, layers collapsed.
  const [expandedNs, setExpandedNs] = useState<Set<string>>(() => new Set(indexes.namespaces));
  const [expandedLayer, setExpandedLayer] = useState<Set<string>>(new Set());

  function toggleNs(ns: string) {
    setExpandedNs(prev => {
      const next = new Set(prev);
      if (next.has(ns)) next.delete(ns);
      else next.add(ns);
      return next;
    });
  }
  function toggleLayer(key: string) {
    setExpandedLayer(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }
  function setLayerFilter(layer: TableLayer) {
    const current = filters.layers;
    const next = current.includes(layer)
      ? current.filter(l => l !== layer)
      : [...current, layer];
    onFiltersChange({ layers: next });
  }

  return (
    <div className="flex flex-col gap-1 text-sm">
      {buckets.length === 0 ? (
        <p className="text-xs text-[var(--text-muted)] italic px-2 py-3">
          {t("atlas.catalog.empty.title")}
        </p>
      ) : (
        buckets.map(bucket => {
          const nsOpen = expandedNs.has(bucket.ns);
          return (
            <div key={bucket.ns} className="flex flex-col">
              <button
                onClick={() => toggleNs(bucket.ns)}
                className="group flex items-center gap-1.5 px-2 py-1.5 rounded-[var(--radius-sm)] hover:bg-[var(--bg-elevated)] transition-colors"
              >
                <Chevron open={nsOpen} />
                <span className="text-[11px] font-semibold uppercase tracking-[0.14em] text-[var(--text-primary)]">
                  {bucket.ns}
                </span>
                <span className="ml-auto text-[10px] tabular-nums text-[var(--text-faint)]">{bucket.total}</span>
              </button>
              {nsOpen && (
                <div className="flex flex-col gap-0.5 pl-3 border-l border-[var(--border-subtle)] ml-3 mt-0.5">
                  {LAYER_ORDER.filter(l => bucket.byLayer.has(l)).map(layer => {
                    const items = bucket.byLayer.get(layer)!;
                    const key = `${bucket.ns}:${layer}`;
                    const layerOpen = expandedLayer.has(key);
                    const s = getLayerStyle(layer);
                    const filterActive = filters.layers.includes(layer);
                    return (
                      <div key={key} className="flex flex-col">
                        <div
                          className={`flex items-center gap-1.5 px-2 py-1 rounded-[var(--radius-sm)] hover:bg-[var(--bg-elevated)] transition-colors ${
                            filterActive ? "bg-[var(--accent-muted)]" : ""
                          }`}
                        >
                          <button onClick={() => toggleLayer(key)} className="shrink-0">
                            <Chevron open={layerOpen} />
                          </button>
                          <button
                            onClick={() => setLayerFilter(layer)}
                            className="flex items-center gap-1.5 flex-1 min-w-0 text-left"
                            title={`Filter by ${layer}`}
                          >
                            <span className={`w-1.5 h-1.5 rounded-full ${s.dot} shrink-0`} />
                            <span className={`text-[11px] font-mono ${filterActive ? "text-[var(--accent)]" : "text-[var(--text-secondary)]"} group-hover:text-[var(--text-primary)]`}>
                              {layer}
                            </span>
                            <span className="ml-auto text-[10px] tabular-nums text-[var(--text-faint)]">{items.length}</span>
                          </button>
                        </div>
                        {layerOpen && !scaffoldOnly && (
                          <ul className="flex flex-col pl-4 mt-0.5 border-l border-[var(--border-subtle)] ml-2">
                            {items.map(tb => {
                              const isSel = tb.node.id === selectedId;
                              return (
                                <li key={tb.node.id}>
                                  <button
                                    onClick={() => onSelect(tb.node.id)}
                                    className={`w-full text-left flex items-center gap-1.5 px-2 py-1 rounded-[var(--radius-sm)] transition-colors ${
                                      isSel
                                        ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                                        : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)]"
                                    }`}
                                    title={tb.node.id}
                                  >
                                    <span className="text-[11px] font-mono truncate">{tb.node.label}</span>
                                    {tb.hasWarning && (
                                      <span className="w-1 h-1 rounded-full bg-red-500 shrink-0" />
                                    )}
                                  </button>
                                </li>
                              );
                            })}
                          </ul>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })
      )}
    </div>
  );
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      className={`w-3 h-3 text-[var(--text-faint)] transition-transform ${open ? "rotate-90" : ""}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
    </svg>
  );
}
