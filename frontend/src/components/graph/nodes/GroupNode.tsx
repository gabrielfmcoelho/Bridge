import type { NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

/** Frame around services of one stack / Coolify environment / Coolify project. */
export default function GroupNode({ data }: NodeProps) {
  return (
    <div className="w-full h-full rounded-[var(--radius-lg)] border border-dashed border-[var(--border-strong)] bg-[var(--bg-surface)]/40">
      <span className="absolute left-2 top-2 inline-flex items-center gap-1 max-w-[calc(100%-1rem)] px-2 py-0.5 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-2xs font-mono text-[var(--text-secondary)]">
        <Icon path={ICON_PATHS.cube} className="w-3 h-3 shrink-0" strokeWidth={1.5} />
        <span className="truncate">{data.label as string}</span>
      </span>
    </div>
  );
}
