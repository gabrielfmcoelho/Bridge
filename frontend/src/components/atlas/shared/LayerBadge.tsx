import type { TableLayer } from "@/lib/lineage/indexes";

const LAYER_STYLES: Record<TableLayer, { bg: string; text: string; border: string; dot: string }> = {
  source: { bg: "bg-blue-500/10",    text: "text-blue-300",    border: "border-blue-500/30",    dot: "bg-blue-400" },
  bronze: { bg: "bg-amber-700/10",   text: "text-amber-400",   border: "border-amber-700/30",   dot: "bg-amber-500" },
  silver: { bg: "bg-slate-400/10",   text: "text-slate-300",   border: "border-slate-400/30",   dot: "bg-slate-300" },
  gold:   { bg: "bg-yellow-500/10",  text: "text-yellow-400",  border: "border-yellow-500/30",  dot: "bg-yellow-400" },
  iapep:  { bg: "bg-violet-500/10",  text: "text-violet-300",  border: "border-violet-500/30",  dot: "bg-violet-400" },
  iaspi:  { bg: "bg-pink-500/10",    text: "text-pink-300",    border: "border-pink-500/30",    dot: "bg-pink-400" },
  other:  { bg: "bg-gray-500/10",    text: "text-gray-400",    border: "border-gray-500/30",    dot: "bg-gray-400" },
};

interface Props {
  layer: TableLayer;
  /** Capitalized label override (defaults to the layer key in upper-case). */
  label?: string;
  /** Show a leading dot in the layer color. */
  dot?: boolean;
  size?: "sm" | "md";
  className?: string;
}

export default function LayerBadge({ layer, label, dot = true, size = "sm", className = "" }: Props) {
  const s = LAYER_STYLES[layer] ?? LAYER_STYLES.other;
  const sizeCls = size === "md" ? "text-xs px-2.5 py-1" : "text-[10px] px-2 py-0.5";
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-full border font-semibold uppercase tracking-[0.08em] ${s.bg} ${s.text} ${s.border} ${sizeCls} ${className}`}
    >
      {dot && <span className={`w-1.5 h-1.5 rounded-full ${s.dot} shrink-0`} />}
      {label ?? layer}
    </span>
  );
}

export function getLayerStyle(layer: TableLayer) {
  return LAYER_STYLES[layer] ?? LAYER_STYLES.other;
}
