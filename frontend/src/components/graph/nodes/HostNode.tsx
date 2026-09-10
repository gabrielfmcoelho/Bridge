import { Handle, Position, type NodeProps } from "@xyflow/react";
import { SITUACAO_DOT_COLORS } from "@/lib/constants";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function HostNode({ data }: NodeProps) {
  const status = (data.status as string) || "active";
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--cyan)] !w-2 !h-2" />
      {/* Color bar */}
      <div className="h-1 bg-gradient-to-r from-[var(--cyan)] to-[var(--cyan)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={ICON_PATHS.serverRack} className="w-3.5 h-3.5 text-[var(--cyan)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--cyan)] truncate font-mono">{data.label as string}</span>
          <span className={`w-2 h-2 rounded-full shrink-0 ${SITUACAO_DOT_COLORS[status] || "bg-[var(--text-faint)]"}`} />
        </div>
        {(data.hostname as string) && (
          <p className="text-2xs truncate font-mono" style={{ color: "var(--text-muted)" }}>{data.hostname as string}</p>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--cyan)] !w-2 !h-2" />
    </div>
  );
}
