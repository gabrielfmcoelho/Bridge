import { Handle, Position, type NodeProps } from "@xyflow/react";

export default function DnsNode({ data }: NodeProps) {
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition-all duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-emerald-500 !w-2 !h-2" />
      <div className="h-1 bg-gradient-to-r from-emerald-500 to-emerald-400" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2">
          <svg className="w-3.5 h-3.5 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" />
          </svg>
          <span className="text-xs font-bold text-emerald-400 truncate" style={{ fontFamily: "var(--font-mono)" }}>{data.label as string}</span>
          {(data.has_https as boolean) && (
            <svg className="w-3 h-3 text-emerald-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
            </svg>
          )}
        </div>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-emerald-500 !w-2 !h-2" />
    </div>
  );
}
