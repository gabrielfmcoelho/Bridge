import { Handle, Position, type NodeProps } from "@xyflow/react";
import { SITUACAO_DOT_COLORS } from "@/lib/constants";

export default function ProjectNode({ data }: NodeProps) {
  const status = (data.status as string) || "active";
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition-all duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-amber-500 !w-2 !h-2" />
      <div className="h-1 bg-gradient-to-r from-amber-500 to-amber-400" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2">
          <svg className="w-3.5 h-3.5 text-amber-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
          </svg>
          <span className="text-xs font-bold text-amber-400 truncate">{data.label as string}</span>
          <span className={`w-2 h-2 rounded-full shrink-0 ${SITUACAO_DOT_COLORS[status] || "bg-gray-400"}`} />
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-amber-500 !w-2 !h-2" />
    </div>
  );
}
