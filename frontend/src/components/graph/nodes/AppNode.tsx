import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { NAV_ICONS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

/** An application (app-deps CI) in the topology graph; opens its dependency page. */
export default function AppNode({ data }: NodeProps) {
  const { t } = useLocale();
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--text-secondary)] !w-2 !h-2" />
      <div className="h-1 bg-[var(--text-secondary)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={NAV_ICONS.Workflow} className="w-3.5 h-3.5 text-[var(--text-secondary)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold font-mono text-[var(--text-primary)] truncate">{data.label as string}</span>
        </div>
        <p className="text-2xs truncate" style={{ color: "var(--text-muted)" }}>{t("appDeps.graphNode")}</p>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--text-secondary)] !w-2 !h-2" />
    </div>
  );
}
