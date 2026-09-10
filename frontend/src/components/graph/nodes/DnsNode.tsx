import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function DnsNode({ data }: NodeProps) {
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--success)] !w-2 !h-2" />
      <div className="h-1 bg-gradient-to-r from-[var(--success)] to-[var(--success)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2">
          <Icon path={ICON_PATHS.circle} className="w-3.5 h-3.5 text-[var(--success)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--success)] truncate font-mono">{data.label as string}</span>
          {(data.has_https as boolean) && (
            <Icon path={ICON_PATHS.lock} className="w-3 h-3 text-[var(--success)] shrink-0" />
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--success)] !w-2 !h-2" />
    </div>
  );
}
