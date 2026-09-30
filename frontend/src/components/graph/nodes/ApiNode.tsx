import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

export default function ApiNode({ data }: NodeProps) {
  const { t } = useLocale();
  const ops = data.operation_count as number | undefined;
  return (
    <div className="rounded-[10px] min-w-[180px] shadow-lg cursor-pointer transition duration-200 hover:shadow-xl overflow-hidden" style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}>
      <Handle type="target" position={Position.Top} className="!bg-[var(--rose)] !w-2 !h-2" />
      <div className="h-1 bg-[var(--rose)]" />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-1">
          <Icon path={ICON_PATHS.code} className="w-3.5 h-3.5 text-[var(--rose)] shrink-0" strokeWidth={1.5} />
          <span className="text-xs font-bold text-[var(--rose)] truncate">{data.label as string}</span>
        </div>
        {ops != null && <p className="text-2xs font-mono truncate" style={{ color: "var(--text-muted)" }}>{t("atlas.apis.endpointsCount", { count: String(ops) })}</p>}
      </div>
      <Handle type="source" position={Position.Bottom} className="!bg-[var(--rose)] !w-2 !h-2" />
    </div>
  );
}
