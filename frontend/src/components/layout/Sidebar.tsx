"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useAuth } from "@/contexts/AuthContext";
import { NAV_SECTIONS, type NavSection } from "@/lib/constants";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import Divider from "@/components/ui/Divider";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";



interface SidebarProps {
  collapsed: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

// Filter nav sections by user permissions/role — items without a permission or role field are always visible.
function filterSections(sections: NavSection[], permissions: string[], role?: string): NavSection[] {
  return sections
    .map((section) => ({
      ...section,
      items: section.items.filter(
        (item) =>
          (!item.permission || permissions.includes(item.permission)) &&
          (!item.role || item.role === role)
      ),
    }))
    .filter((section) => section.items.length > 0);
}

export default function Sidebar({ collapsed, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const { user } = useAuth();
  const sections = filterSections(NAV_SECTIONS, user?.permissions ?? [], user?.role);

  const sidebarContent = (
    <aside className={`bg-[var(--bg-surface)] border-r border-[var(--border-subtle)] flex flex-col h-full transition-[width] duration-200 ${collapsed ? "w-16" : "w-60"}`}>
      {/* Branding — matches header h-13 */}
      <div className="h-13 px-4 flex items-center border-b border-[var(--border-subtle)]">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-[var(--radius-md)] flex items-center justify-center overflow-hidden shrink-0"
            style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
            ) : (
              <Icon path={ICON_PATHS.prompt} />
            )}
          </div>
          {!collapsed && (
            <div className="min-w-0">
              <h1 className="text-sm font-bold text-[var(--text-primary)] truncate font-display">
                {appName}
              </h1>
              <p className="text-2xs text-[var(--text-muted)] leading-tight">{t("app.subtitle")}</p>
            </div>
          )}
        </div>
      </div>

      {/* Navigation by sections */}
      <nav className="flex-1 p-2.5 overflow-y-auto">
        {sections.map((section, sIdx) => (
          <div key={section.key}>
            {sIdx > 0 && <Divider className="my-3" />}
            {section.label && !collapsed && (
              <p className="px-3 py-1 text-2xs font-semibold text-[var(--text-faint)] uppercase tracking-widest">
                {t(section.label)}
              </p>
            )}
            <div className="space-y-0.5">
              {section.items.map((item) => {
                if (item.href === "#") {
                  return (
                    <span
                      key={item.label}
                      title={collapsed ? t(item.label) : t("common.comingSoon")}
                      className={`group flex items-center gap-2.5 px-3 py-2 rounded-[var(--radius-md)] text-sm font-medium opacity-40 cursor-not-allowed ${
                        collapsed ? "justify-center" : ""
                      } text-[var(--text-muted)]`}
                    >
                      <Icon path={NAV_ICONS[item.icon] || NAV_ICONS.Server} className="w-[18px] h-[18px] shrink-0" strokeWidth={1.5} />
                      {!collapsed && t(item.label)}
                    </span>
                  );
                }
                const isActive = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={onCloseMobile}
                    title={collapsed ? t(item.label) : undefined}
                    className={`group flex items-center gap-2.5 px-3 py-2 rounded-[var(--radius-md)] text-sm font-medium transition duration-150 relative ${
                      collapsed ? "justify-center" : ""
                    } ${!isActive ? "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]" : ""}`}
                    style={isActive ? { backgroundColor: `${appColor}18`, color: appColor } : undefined}
                  >
                    {isActive && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full" style={{ backgroundColor: appColor }} />
                    )}
                    <Icon path={NAV_ICONS[item.icon] || NAV_ICONS.Server} className="w-[18px] h-[18px] shrink-0" strokeWidth={1.5} />
                    {!collapsed && t(item.label)}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {!collapsed && (
        <div className="p-3 border-t border-[var(--border-subtle)]">
          <p className="text-2xs text-[var(--text-faint)] text-center font-mono">
            v0.1.0
          </p>
        </div>
      )}
    </aside>
  );

  return (
    <>
      {/* Desktop sidebar */}
      <div className="hidden md:block shrink-0">
        {sidebarContent}
      </div>

      {/* Mobile bottom drawer */}
      <MobileDrawer open={mobileOpen} onClose={onCloseMobile} pathname={pathname} appColor={appColor} t={t} sections={sections} />
    </>
  );
}

function MobileDrawer({
  open, onClose, pathname, appColor, t, sections,
}: {
  open: boolean;
  onClose: () => void;
  pathname: string;
  appColor: string;
  t: (key: string) => string;
  sections: NavSection[];
}) {
  return (
    <div className="md:hidden">
      <Drawer
        open={open}
        onClose={onClose}
        title={t("nav.modules")}
        footer={
          <Button size="sm" className="w-full" onClick={onClose}>
            {t("common.close")}
          </Button>
        }
      >
        <nav className="space-y-4">
          {sections.map((section) => (
            <div key={section.key}>
              {section.label && (
                <p className="px-1 pb-1.5 text-2xs font-semibold text-[var(--text-faint)] uppercase tracking-widest">
                  {t(section.label)}
                </p>
              )}
              <div className="grid grid-cols-3 gap-2">
                {section.items.map((item) => {
                  if (item.href === "#") {
                    return (
                      <span
                        key={item.label}
                        className="flex flex-col items-center gap-1.5 p-3 rounded-[var(--radius-md)] text-xs font-medium opacity-40 text-[var(--text-muted)]"
                      >
                        <Icon path={NAV_ICONS[item.icon] || NAV_ICONS.Server} className="w-[18px] h-[18px] shrink-0" strokeWidth={1.5} />
                        {t(item.label)}
                      </span>
                    );
                  }
                  const isActive = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={onClose}
                      className={`flex flex-col items-center gap-1.5 p-3 rounded-[var(--radius-md)] text-xs font-medium transition ${
                        isActive ? "text-white" : "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
                      }`}
                      style={isActive ? { backgroundColor: appColor, color: "#fff" } : undefined}
                    >
                      <Icon path={NAV_ICONS[item.icon] || NAV_ICONS.Server} className="w-[18px] h-[18px] shrink-0" strokeWidth={1.5} />
                      {t(item.label)}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </Drawer>
    </div>
  );
}
