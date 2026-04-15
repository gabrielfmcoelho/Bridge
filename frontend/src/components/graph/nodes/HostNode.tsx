import { Handle, Position, type NodeProps } from "@xyflow/react";
import { SITUACAO_DOT_COLORS } from "@/lib/constants";

export default function HostNode({ data }: NodeProps) {
  const status = (data.status as string) || "active";
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition-all duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-cyan-500 !w-2 !h-2" />
      {/* Color bar */}
      <div className="h-1 bg-gradient-to-r from-cyan-500 to-cyan-400" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <svg className="w-3.5 h-3.5 text-cyan-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2z" />
          </svg>
          <span className="text-xs font-bold text-cyan-400 truncate" style={{ fontFamily: "var(--font-mono)" }}>{data.label as string}</span>
          <span className={`w-2 h-2 rounded-full shrink-0 ${SITUACAO_DOT_COLORS[status] || "bg-gray-400"}`} />
        </div>
        {(data.hostname as string) && (
          <p className="text-[10px] truncate" style={{ fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>{data.hostname as string}</p>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-cyan-500 !w-2 !h-2" />
    </div>
  );
}
