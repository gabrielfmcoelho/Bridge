"use client";

import { Section, Specimen } from "./Section";
import LineChart, { CHART_COLORS, CHART_OTHER } from "@/components/ui/LineChart";

// Two days of hourly points: three keys plus "outros", the shape the API
// "Requisições" tab draws with "Por chave".
const HOUR = 3_600_000;
const START = Date.UTC(2026, 9, 5, 0);
const ROWS = Array.from({ length: 48 }, (_, i) => ({
  t: START + i * HOUR,
  a: Math.round(40 + 30 * Math.sin(i / 4) + (i % 7)),
  b: Math.round(20 + 10 * Math.cos(i / 5)),
  c: (i * 3) % 17,
  other: 4 + (i % 3),
}));

export default function ChartsSection() {
  return (
    <Section id="charts" title="Charts">
      <Specimen title="LineChart" source="components/ui/LineChart.tsx" alsoIn={["components/telemetry/RequestsTab.tsx"]} wide>
        <LineChart
          data={ROWS}
          xKey="t"
          series={[
            { key: "a", label: "front · Gabriel Coelho", color: CHART_COLORS[0] },
            { key: "b", label: "vp-api", color: CHART_COLORS[1] },
            { key: "c", label: "integracao-pge", color: CHART_COLORS[2] },
            { key: "other", label: "outros", color: CHART_OTHER },
          ]}
          formatX={(v) => new Date(v).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" })}
          formatY={(v) => v.toLocaleString("pt-BR")}
          ariaLabel="Requisições por chave"
        />
        <p className="mt-2 text-xs text-[var(--text-muted)]">
          Series colors are <code>--chart-1…5</code> in fixed order (validated for color-blindness in both themes), <code>--chart-other</code> folds the rest.
          Never status colors. One y scale; a second measure is a toggle or a second chart.
        </p>
      </Specimen>
    </Section>
  );
}
