"use client";

import type { ReactNode } from "react";
import {
  CartesianGrid,
  Line,
  LineChart as RLineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

/** Categorical slots in fixed order (globals.css). Color follows the entity:
 *  give each series the same slot every render, never by rank. */
export const CHART_COLORS = ["var(--chart-1)", "var(--chart-2)", "var(--chart-3)", "var(--chart-4)", "var(--chart-5)"];
export const CHART_OTHER = "var(--chart-other)";

export interface LineSeries {
  key: string;
  label: string;
  color: string;
}

interface LineChartProps {
  /** One row per x value: { [xKey]: number, [series.key]: number }. */
  data: Record<string, number>[];
  xKey: string;
  series: LineSeries[];
  formatX: (v: number) => string;
  formatY: (v: number) => string;
  /** Longer x text for the tooltip header (defaults to formatX). */
  formatTooltipX?: (v: number) => string;
  height?: number;
  ariaLabel: string;
}

// Line chart in the shadcn chart pattern on recharts: 2px lines, recessive
// grid/axes on text tokens, crosshair + tooltip on hover, legend from 2 series.
// One y scale only — a second measure is a second chart (or a toggle).
export default function LineChart({ data, xKey, series, formatX, formatY, formatTooltipX, height = 180, ariaLabel }: LineChartProps) {
  return (
    <div role="img" aria-label={ariaLabel}>
      {series.length > 1 && (
        <ul className="flex flex-wrap gap-x-4 gap-y-1 mb-2 text-xs text-[var(--text-secondary)]">
          {series.map((s) => (
            <li key={s.key} className="inline-flex items-center gap-1.5 min-w-0">
              <span className="w-3 h-0.5 rounded-full shrink-0" style={{ background: s.color }} />
              <span className="truncate max-w-[16rem]">{s.label}</span>
            </li>
          ))}
        </ul>
      )}
      <ResponsiveContainer width="100%" height={height}>
        <RLineChart data={data} margin={{ top: 6, right: 8, bottom: 0, left: 0 }}>
          <CartesianGrid vertical={false} stroke="var(--border-subtle)" />
          <XAxis
            dataKey={xKey}
            type="number"
            domain={["dataMin", "dataMax"]}
            scale="time"
            tickFormatter={formatX}
            tick={{ fill: "var(--text-muted)", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            minTickGap={24}
          />
          <YAxis
            tickFormatter={formatY}
            tick={{ fill: "var(--text-muted)", fontSize: 11 }}
            axisLine={false}
            tickLine={false}
            width={52}
            allowDecimals={false}
          />
          <Tooltip
            cursor={{ stroke: "var(--border-default)", strokeWidth: 1 }}
            content={({ active, label, payload }) =>
              active && payload?.length ? (
                <ChartTooltip title={(formatTooltipX ?? formatX)(Number(label))}>
                  {series.map((s) => {
                    const v = payload.find((p) => p.dataKey === s.key)?.value;
                    return v === undefined ? null : (
                      <div key={s.key} className="flex items-center gap-2">
                        <span className="w-2 h-2 rounded-full shrink-0" style={{ background: s.color }} />
                        <span className="truncate max-w-[14rem] text-[var(--text-secondary)]">{s.label}</span>
                        <span className="ml-auto pl-3 font-mono tabular-nums text-[var(--text-primary)]">{formatY(Number(v))}</span>
                      </div>
                    );
                  })}
                </ChartTooltip>
              ) : null
            }
          />
          {series.map((s) => (
            <Line
              key={s.key}
              dataKey={s.key}
              type="monotone"
              stroke={s.color}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4, stroke: "var(--bg-surface)", strokeWidth: 2 }}
              isAnimationActive={false}
            />
          ))}
        </RLineChart>
      </ResponsiveContainer>
    </div>
  );
}

function ChartTooltip({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] px-3 py-2 text-xs shadow-lg space-y-1 min-w-[10rem]">
      <p className="text-[var(--text-muted)]">{title}</p>
      {children}
    </div>
  );
}
