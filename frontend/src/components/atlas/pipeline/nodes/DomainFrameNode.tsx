"use client";

import { memo, type CSSProperties } from "react";
import type { NodeProps } from "@xyflow/react";

interface Data {
  ns: string;
}

const DOMAIN_TINTS: Record<string, { ring: string; bg: CSSProperties["background"]; label: string }> = {
  servidores: { ring: "rgba(34,211,238,0.30)", bg: "linear-gradient(135deg, rgba(34,211,238,0.04) 0%, rgba(34,211,238,0) 70%)", label: "text-[var(--cyan)]" },
  sei:        { ring: "rgba(167,139,250,0.30)", bg: "linear-gradient(135deg, rgba(167,139,250,0.04) 0%, rgba(167,139,250,0) 70%)", label: "text-[var(--purple)]" },
  mercado:    { ring: "rgba(251,191,36,0.30)", bg: "linear-gradient(135deg, rgba(251,191,36,0.04) 0%, rgba(251,191,36,0) 70%)", label: "text-[var(--warning)]" },
  shared:     { ring: "rgba(148,163,184,0.30)", bg: "linear-gradient(135deg, rgba(148,163,184,0.04) 0%, rgba(148,163,184,0) 70%)", label: "text-[var(--text-muted)]" },
};

function DomainFrameNodeImpl({ data }: NodeProps) {
  const d = data as unknown as Data;
  const tint = DOMAIN_TINTS[d.ns] ?? DOMAIN_TINTS.shared;
  return (
    <div
      className="relative w-full h-full rounded-[var(--radius-lg)] border-2 border-dashed pointer-events-none"
      style={{ borderColor: tint.ring, background: tint.bg }}
    >
      <span
        className={`absolute top-2 left-3 text-xs font-semibold ${tint.label} font-display`}
      >
        {d.ns}
      </span>
    </div>
  );
}

export default memo(DomainFrameNodeImpl);
