import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

const variantStyles: Record<string, string> = {
  success: "bg-[var(--success)]/10 border border-[var(--success)]/25 text-[var(--success)]",
  error: "bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]",
  warning: "bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-[var(--warning)]",
  info: "bg-[var(--info)]/10 border border-[var(--info)]/25 text-[var(--info)]",
  loading: "bg-[var(--info)]/10 border border-[var(--info)]/25 text-[var(--info)]",
};

const icons: Record<string, React.ReactNode> = {
  success: <Icon path={ICON_PATHS.checkCircle} />,
  error: <Icon path={ICON_PATHS.xCircle} />,
  warning: <Icon path={ICON_PATHS.alert} />,
  info: <Icon path={ICON_PATHS.infoCircle} />,
  loading: (
    <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
    </svg>
  ),
};

export default function StatusAlert({
  variant,
  children,
  className = "",
}: {
  variant: "success" | "error" | "warning" | "info" | "loading";
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-[var(--radius-md)] p-3 text-sm animate-slide-up ${variantStyles[variant]} ${className}`}>
      <div className="flex items-center gap-2">
        {icons[variant]}
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </div>
  );
}
