import Icon from "./Icon";
import Spinner from "./Spinner";
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
  loading: <Spinner />,
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
