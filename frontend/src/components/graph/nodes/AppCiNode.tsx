import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/** A CI itself, inside its frame: the end of a link that names no component of it. */
export default function AppCiNode({ data }: NodeProps) {
  const clickable = !!data.catalogued && !data.focus;
  return (
    <div
      className={`w-[200px] rounded-[10px] px-3 py-2 shadow-lg transition duration-200 ${clickable ? "cursor-pointer hover:shadow-xl" : ""}`}
      style={{ background: "var(--bg-elevated)", border: "1px solid var(--border-strong)" }}
    >
      <Handle type="target" position={Position.Left} className="!w-2 !h-2 !bg-[var(--accent)]" />
      <div className="flex items-center gap-2">
        <Icon path={ICON_PATHS.serverStack} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" strokeWidth={1.5} />
        <span className="text-xs font-bold font-mono truncate text-[var(--accent)]">{data.label as string}</span>
      </div>
      <Handle type="source" position={Position.Right} className="!w-2 !h-2 !bg-[var(--accent)]" />
    </div>
  );
}
