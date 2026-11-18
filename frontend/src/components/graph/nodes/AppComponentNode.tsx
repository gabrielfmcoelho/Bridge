import { Handle, Position, type NodeProps } from "@xyflow/react";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import { criticalityColor, criticalityKey } from "@/lib/appDeps";

/** A component of a CI (app dependencies diagram): name, technology · endpoint and its
 *  criticality as a coloured top rule plus its name (never colour alone). */
export default function AppComponentNode({ data }: NodeProps) {
  const { t } = useLocale();
  const crit = data.criticality as string | undefined;
  const color = criticalityColor(crit);
  const key = criticalityKey(crit);
  const meta = [data.technology as string | undefined, data.endpoint as string | undefined].filter(Boolean).join(" · ");
  const clickable = !!data.catalogued && !data.focus;
  return (
    <div
      title={(data.description as string | undefined) || undefined}
      className={`w-[200px] rounded-[10px] shadow-lg overflow-hidden transition duration-200 ${clickable ? "cursor-pointer hover:shadow-xl" : ""}`}
      style={{ background: "var(--bg-surface)", border: "1px solid var(--border-default)" }}
    >
      <Handle type="target" position={Position.Left} className="!w-2 !h-2" style={{ background: color }} />
      <div className="h-1" style={{ background: color }} />
      <div className="px-3 py-2">
        <div className="flex items-center gap-2 mb-0.5">
          <Icon path={data.data ? ICON_PATHS.database : ICON_PATHS.cube} className="w-3.5 h-3.5 shrink-0 text-[var(--text-secondary)]" strokeWidth={1.5} />
          <span className="text-xs font-bold font-mono truncate text-[var(--text-primary)]">{data.label as string}</span>
        </div>
        <p className="text-2xs font-mono truncate text-[var(--text-muted)]">{meta || "–"}</p>
        {crit && <p className="text-2xs truncate text-[var(--text-muted)]">{key ? t(key) : crit}</p>}
      </div>
      <Handle type="source" position={Position.Right} className="!w-2 !h-2" style={{ background: color }} />
    </div>
  );
}
