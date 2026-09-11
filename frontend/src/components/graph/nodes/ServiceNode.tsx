import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function ServiceNode({ data }: NodeProps) {
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--accent)] !w-2 !h-2" />
      <div className="h-1 bg-gradient-to-r from-[var(--accent)] to-[var(--accent)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={ICON_PATHS.folder} className="w-3.5 h-3.5 text-[var(--accent)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--accent)] truncate">{data.label as string}</span>
        </div>
        {(data.technology_stack as string) && (
          <p className="text-2xs truncate" style={{ color: "var(--text-muted)" }}>{data.technology_stack as string}</p>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--accent)] !w-2 !h-2" />
    </div>
  );
}
