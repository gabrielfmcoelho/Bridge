import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import { criticalityColor, criticalityKey } from "@/lib/appDeps";

/** A CI with no catalog in Bridge (external system, or a consumer): a pill with the
 *  traits its link carries inline. */
export default function AppExternalNode({ data }: NodeProps) {
  const { t } = useLocale();
  const crit = data.criticality as string | undefined;
  const key = criticalityKey(crit);
  const meta = [data.technology as string | undefined, data.endpoint as string | undefined, crit && (key ? t(key) : crit)]
    .filter(Boolean).join(" · ");
  return (
    <div
      title={(data.description as string | undefined) || undefined}
      className="w-[200px] rounded-full px-4 py-2 border border-dashed"
      style={{ background: "var(--bg-surface)", borderColor: criticalityColor(crit) }}
    >
      <Handle type="target" position={Position.Left} className="!w-2 !h-2 !bg-[var(--text-muted)]" />
      <div className="flex items-center gap-2">
        <Icon path={data.data ? ICON_PATHS.database : ICON_PATHS.globe} className="w-3.5 h-3.5 shrink-0 text-[var(--text-secondary)]" strokeWidth={1.5} />
        <span className="text-xs font-semibold font-mono truncate text-[var(--text-primary)]">{data.label as string}</span>
      </div>
      {meta && <p className="text-2xs font-mono truncate text-[var(--text-muted)] pl-5">{meta}</p>}
      <Handle type="source" position={Position.Right} className="!w-2 !h-2 !bg-[var(--text-muted)]" />
    </div>
  );
}
