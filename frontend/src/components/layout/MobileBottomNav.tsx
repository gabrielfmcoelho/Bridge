"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAppearance } from "@/contexts/AppearanceContext";

interface MobileBottomNavProps {
  onOpenDrawer: () => void;
}

const navItems = [
  { href: "/", label: "Painel", icon: "M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" },
  { href: "/issues", label: "Issues", icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" },
  { href: "__drawer__", label: "Menu", icon: "" },
  { href: "/hosts", label: "Hosts", icon: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2zm2-7a1 1 0 100 2 1 1 0 000-2zm0 10a1 1 0 100 2 1 1 0 000-2z" },
  { href: "/services", label: "Services", icon: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" },
];

export default function MobileBottomNav({ onOpenDrawer }: MobileBottomNavProps) {
  const pathname = usePathname();
  const { appColor } = useAppearance();

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
                  <svg className="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
                  </svg>
                </div>
                <span className="text-[10px] font-medium mt-0.5" style={{ color: appColor }}>
                  {item.label}
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
              <svg
                className="w-5 h-5"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={isActive ? 2 : 1.5}
                style={{ color: isActive ? appColor : "var(--text-muted)" }}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d={item.icon} />
              </svg>
              <span
                className="text-[10px] font-medium mt-0.5"
                style={{ color: isActive ? appColor : "var(--text-faint)" }}
              >
                {item.label}
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
