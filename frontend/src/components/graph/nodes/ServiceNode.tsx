import { Handle, Position, type NodeProps } from "@xyflow/react";

export default function ServiceNode({ data }: NodeProps) {
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition-all duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-purple-500 !w-2 !h-2" />
      <div className="h-1 bg-gradient-to-r from-purple-500 to-purple-400" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <svg className="w-3.5 h-3.5 text-purple-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          <span className="text-xs font-bold text-purple-400 truncate">{data.label as string}</span>
        </div>
        {(data.technology_stack as string) && (
          <p className="text-[10px] truncate" style={{ color: "var(--text-muted)" }}>{data.technology_stack as string}</p>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-purple-500 !w-2 !h-2" />
    </div>
  );
}
