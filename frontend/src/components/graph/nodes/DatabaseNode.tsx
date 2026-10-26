import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

export default function DatabaseNode({ data }: NodeProps) {
  const { t } = useLocale();
  const engine = data.engine as string | undefined;
  const dbName = data.db_name as string | undefined;
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--info)] !w-2 !h-2" />
      <div className="h-1 bg-[var(--info)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={ICON_PATHS.database} className="w-3.5 h-3.5 text-[var(--info)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--info)] truncate">{data.label as string}</span>
        </div>
        <p className="text-2xs font-mono truncate" style={{ color: "var(--text-muted)" }}>
          {[engine && t(`atlas.databases.engine.${engine}`), dbName].filter(Boolean).join(" · ")}
        </p>
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--info)] !w-2 !h-2" />
    </div>
  );
}
