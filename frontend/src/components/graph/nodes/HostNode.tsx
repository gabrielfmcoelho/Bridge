import { Handle, Position, type NodeProps } from "@xyflow/react";
import { useSituacao } from "@/hooks/useSituacao";
import { SITUACAO_DOT_COLORS } from "@/lib/constants";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { HostToggle } from "@/lib/topology";

export default function HostNode({ data }: NodeProps) {
  const { roleOf } = useSituacao();
  const { t } = useLocale();
  const status = (data.status as string) || "active";
  const toggle = data.toggle as HostToggle | undefined;
  const onToggle = data.onToggle as (() => void) | undefined;
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--cyan)] !w-2 !h-2" />
      {/* Color bar */}
      <div className="h-1 bg-gradient-to-r from-[var(--cyan)] to-[var(--cyan)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={ICON_PATHS.serverRack} className="w-3.5 h-3.5 text-[var(--cyan)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--cyan)] truncate font-mono">{data.label as string}</span>
          <span className={`w-2 h-2 rounded-full shrink-0 ${SITUACAO_DOT_COLORS[roleOf(status)] || "bg-[var(--text-faint)]"}`} />
        </div>
        {(data.hostname as string) && (
          <p className="text-2xs truncate font-mono" style={{ color: "var(--text-muted)" }}>{data.hostname as string}</p>
        )}
        {toggle && onToggle && (
          <button
            type="button"
            // The card navigates to the host on click; the toggle must not.
            onClick={(e) => { e.stopPropagation(); onToggle(); }}
            aria-expanded={toggle.expanded}
            className="nodrag nopan mt-1.5 inline-flex items-center gap-1 px-1.5 py-0.5 rounded-[var(--radius-sm)] text-2xs font-medium text-[var(--cyan)] bg-[var(--cyan)]/10 hover:bg-[var(--cyan)]/20 transition-colors"
          >
            <Icon path={toggle.expanded ? ICON_PATHS.chevronDown : ICON_PATHS.chevronRight} className="w-3 h-3" />
            {toggle.expanded ? t("topology.collapseServices") : t("topology.moreServices", { count: String(toggle.count) })}
          </button>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--cyan)] !w-2 !h-2" />
    </div>
  );
}
