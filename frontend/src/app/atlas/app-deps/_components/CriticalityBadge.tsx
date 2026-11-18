import Lozenge, { type LozengeAppearance } from "@/components/ui/Lozenge";
import { useLocale } from "@/contexts/LocaleContext";
import { criticalityKey } from "@/lib/appDeps";

const APPEARANCE: Record<string, LozengeAppearance> = { high: "removed", medium: "moved", low: "success" };

/** A catalog criticality as a state lozenge; a value outside the governance list shows as-is. */
export default function CriticalityBadge({ value }: { value?: string }) {
  const { t } = useLocale();
  if (!value) return <span className="text-[var(--text-faint)]">–</span>;
  const key = criticalityKey(value);
  return <Lozenge appearance={APPEARANCE[value] ?? "default"}>{key ? t(key) : value}</Lozenge>;
}
