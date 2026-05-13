import { useLocale } from "@/contexts/LocaleContext";
import type { TableRole } from "@/lib/atlas/types";

const STYLES: Record<TableRole, { bg: string; text: string; border: string }> = {
  source: { bg: "bg-cyan-500/10",    text: "text-cyan-300",    border: "border-cyan-500/30" },
  built:  { bg: "bg-emerald-500/10", text: "text-emerald-300", border: "border-emerald-500/30" },
};

interface Props {
  role: TableRole;
  size?: "sm" | "md";
  className?: string;
}

export default function RoleBadge({ role, size = "sm", className = "" }: Props) {
  const { t } = useLocale();
  const s = STYLES[role];
  const sizeCls = size === "md" ? "text-xs px-2.5 py-1" : "text-[10px] px-2 py-0.5";
  const label = role === "source" ? t("atlas.catalog.role.source") : t("atlas.catalog.role.built");
  return (
    <span className={`inline-flex items-center rounded-full border font-medium tracking-wide ${s.bg} ${s.text} ${s.border} ${sizeCls} ${className}`}>
      {label}
    </span>
  );
}
