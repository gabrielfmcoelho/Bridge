"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useLocale } from "@/contexts/LocaleContext";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";

interface MobileBottomNavProps {
  onOpenDrawer: () => void;
}

const navItems = [
  { href: "/", label: "nav.dashboard", icon: NAV_ICONS.LayoutDashboard },
  { href: "/issues", label: "nav.issues", icon: NAV_ICONS.ListChecks },
  { href: "__drawer__", label: "nav.menu", icon: "" },
  { href: "/hosts", label: "nav.hosts", icon: NAV_ICONS.Server },
  { href: "/services", label: "nav.services", icon: NAV_ICONS.Boxes },
];

export default function MobileBottomNav({ onOpenDrawer }: MobileBottomNavProps) {
  const pathname = usePathname();
  const { appColor } = useAppearance();
  const { t } = useLocale();

  return (
    <nav className="md:hidden fixed bottom-0 left-0 right-0 z-40 bg-[var(--bg-surface)] border-t border-[var(--border-subtle)] safe-area-bottom">
      <div className="flex items-end justify-around px-1 pt-1.5 pb-1.5">
        {navItems.map((item) => {
          if (item.href === "__drawer__") {
            // Center drawer button — special style
            return (
              <button
                key="drawer"
                onClick={onOpenDrawer}
                className="flex flex-col items-center justify-center -mt-4 relative"
              >
                <div
                  className="w-12 h-12 rounded-full flex items-center justify-center shadow-lg border-2"
                  style={{
                    backgroundColor: appColor,
                    borderColor: "var(--bg-surface)",
                    boxShadow: `0 4px 14px ${appColor}40`,
                  }}
                >
                  <Icon path={ICON_PATHS.menu} className="w-5 h-5 text-white" />
                </div>
                <span className="text-2xs font-medium mt-0.5" style={{ color: appColor }}>
                  {t(item.label)}
                </span>
              </button>
            );
          }

          const isActive = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);

          return (
            <Link
              key={item.href}
              href={item.href}
              className="flex flex-col items-center justify-center py-1 px-2 min-w-[56px] transition-colors"
            >
              <Icon path={item.icon} className="w-5 h-5" strokeWidth={isActive ? 2 : 1.5}
                style={{ color: isActive ? appColor : "var(--text-muted)" }} />
              <span
                className="text-2xs font-medium mt-0.5"
                style={{ color: isActive ? appColor : "var(--text-muted)" }}
              >
                {t(item.label)}
              </span>
              {isActive && (
                <span
                  className="absolute -top-px left-1/2 -translate-x-1/2 w-8 h-0.5 rounded-full"
                  style={{ backgroundColor: appColor }}
                />
              )}
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
