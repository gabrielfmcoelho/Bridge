"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useAuth } from "@/contexts/AuthContext";
import { NAV_SECTIONS, type NavSection } from "@/lib/constants";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";

const icons: Record<string, string> = {
  LayoutDashboard: "M4 5a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1V5zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1V5zM4 15a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1H5a1 1 0 01-1-1v-4zm10 0a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z",
  Server: "M5 3h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2zm0 10h14a2 2 0 012 2v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4a2 2 0 012-2zm2-7a1 1 0 100 2 1 1 0 000-2zm0 10a1 1 0 100 2 1 1 0 000-2z",
  Globe: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z",
  FolderKanban: "M2 6a2 2 0 012-2h5l2 2h9a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V6z",
  Boxes: "M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z",
  Network: "M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5",
  Rocket: "M9.315 7.584C12.195 3.883 16.615 1.5 21.75 1.5a.75.75 0 01.75.75c0 5.136-2.383 9.555-6.084 12.436A6.75 6.75 0 019.75 22.5a.75.75 0 01-.75-.75v-4.131A15.838 15.838 0 016.382 20H2.25a.75.75 0 01-.75-.75v-4.131a6.75 6.75 0 017.815-6.535z",
  Wrench: "M11.42 15.17L17.25 21A2.652 2.652 0 0021 17.25l-5.877-5.877M11.42 15.17l2.496-3.03c.317-.384.74-.626 1.208-.766M11.42 15.17l-4.655 5.653a2.548 2.548 0 11-3.586-3.586l6.837-5.63m5.108-.233c.55-.164 1.163-.188 1.743-.14a4.5 4.5 0 004.486-6.336l-3.276 3.277a3.004 3.004 0 01-2.25-2.25l3.276-3.276a4.5 4.5 0 00-6.336 4.486c.049.58.025 1.193-.14 1.743",
  Terminal: "M4 17l6-6-6-6m8 14h8",
  Key: "M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z",
  Users: "M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z",
  Lock: "M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z",
  ClipboardList: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4",
  Book: "M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253",
  Ticket: "M15 5v2m0 4v2m0 4v2M5 5a2 2 0 00-2 2v3a2 2 0 110 4v3a2 2 0 002 2h14a2 2 0 002-2v-3a2 2 0 110-4V7a2 2 0 00-2-2H5z",
  Settings: "M12 15a3 3 0 100-6 3 3 0 000 6zm7.43-2.56a1.04 1.04 0 00.2-1.1l-.68-1.17a1.04 1.04 0 00-1-.56h-.3a7 7 0 00-.72-.42l-.1-.29a1.04 1.04 0 00-.62-.84l-1.17-.68a1.04 1.04 0 00-1.1.2l-.22.21a7 7 0 00-.84 0l-.22-.21a1.04 1.04 0 00-1.1-.2l-1.17.68a1.04 1.04 0 00-.62.84l-.1.3a7 7 0 00-.72.41h-.3a1.04 1.04 0 00-1 .56l-.68 1.17a1.04 1.04 0 00.2 1.1l.21.22a7 7 0 000 .84l-.21.22a1.04 1.04 0 00-.2 1.1l.68 1.17c.17.3.5.5.84.56h.3c.13.15.27.29.42.42l.1.3c.06.34.26.62.56.78l1.17.68c.35.2.78.16 1.1-.2l.22-.21a7 7 0 00.84 0l.22.21c.32.36.75.4 1.1.2l1.17-.68c.3-.16.5-.44.56-.78l.1-.3c.15-.13.29-.27.42-.42h.3c.34-.06.67-.26.84-.56l.68-1.17a1.04 1.04 0 00-.2-1.1l-.21-.22a7 7 0 000-.84l.21-.22z",
};

function SvgIcon({ name }: { name: string }) {
  const d = icons[name] || icons.Server;
  return (
    <svg className="w-[18px] h-[18px] shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
      <path strokeLinecap="round" strokeLinejoin="round" d={d} />
    </svg>
  );
}

interface SidebarProps {
  collapsed: boolean;
  mobileOpen: boolean;
  onCloseMobile: () => void;
}

// Filter nav sections by user permissions — items without a permission field are always visible.
function filterSections(sections: NavSection[], permissions: string[]): NavSection[] {
  return sections
    .map((section) => ({
      ...section,
      items: section.items.filter(
        (item) => !item.permission || permissions.includes(item.permission)
      ),
    }))
    .filter((section) => section.items.length > 0);
}

export default function Sidebar({ collapsed, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const { user } = useAuth();
  const sections = filterSections(NAV_SECTIONS, user?.permissions ?? []);

  const sidebarContent = (
    <aside className={`bg-[var(--bg-surface)] border-r border-[var(--border-subtle)] flex flex-col h-full transition-all duration-200 ${collapsed ? "w-16" : "w-60"}`}>
      {/* Branding — matches header h-13 */}
      <div className="h-13 px-4 flex items-center border-b border-[var(--border-subtle)]">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-[var(--radius-md)] flex items-center justify-center overflow-hidden shrink-0"
            style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
            ) : (
              <svg className="w-4 h-4" style={{ color: appColor }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
              </svg>
            )}
          </div>
          {!collapsed && (
            <div className="min-w-0">
              <h1 className="text-sm font-bold text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-display)" }}>
                {appName}
              </h1>
              <p className="text-[10px] text-[var(--text-muted)] leading-tight">{t("app.subtitle")}</p>
            </div>
          )}
        </div>
      </div>

      {/* Navigation by sections */}
      <nav className="flex-1 p-2.5 overflow-y-auto">
        {sections.map((section, sIdx) => (
          <div key={section.key}>
            {sIdx > 0 && <div className="my-3 border-t border-[var(--border-subtle)]" />}
            {section.label && !collapsed && (
              <p className="px-3 py-1 text-[10px] font-semibold text-[var(--text-faint)] uppercase tracking-widest">
                {t(section.label)}
              </p>
            )}
            <div className="space-y-0.5">
              {section.items.map((item) => {
                if (item.href === "#") {
                  return (
                    <span
                      key={item.label}
                      title={collapsed ? t(item.label) : "Coming soon"}
                      className={`group flex items-center gap-2.5 px-3 py-2 rounded-[var(--radius-md)] text-[13px] font-medium opacity-40 cursor-not-allowed ${
                        collapsed ? "justify-center" : ""
                      } text-[var(--text-muted)]`}
                    >
                      <SvgIcon name={item.icon} />
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
                    className={`group flex items-center gap-2.5 px-3 py-2 rounded-[var(--radius-md)] text-[13px] font-medium transition-all duration-150 relative ${
                      collapsed ? "justify-center" : ""
                    } ${!isActive ? "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]" : ""}`}
                    style={isActive ? { backgroundColor: `${appColor}18`, color: appColor } : undefined}
                  >
                    {isActive && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full" style={{ backgroundColor: appColor }} />
                    )}
                    <SvgIcon name={item.icon} />
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
          <p className="text-[10px] text-[var(--text-faint)] text-center" style={{ fontFamily: "var(--font-mono)" }}>
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
            Fechar
          </Button>
        }
      >
        <nav className="space-y-4">
          {sections.map((section) => (
            <div key={section.key}>
              {section.label && (
                <p className="px-1 pb-1.5 text-[10px] font-semibold text-[var(--text-faint)] uppercase tracking-widest">
                  {t(section.label)}
                </p>
              )}
              <div className="grid grid-cols-3 gap-2">
                {section.items.map((item) => {
                  if (item.href === "#") {
                    return (
                      <span
                        key={item.label}
                        className="flex flex-col items-center gap-1.5 p-3 rounded-[var(--radius-md)] text-[11px] font-medium opacity-40 text-[var(--text-muted)]"
                      >
                        <SvgIcon name={item.icon} />
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
                      className={`flex flex-col items-center gap-1.5 p-3 rounded-[var(--radius-md)] text-[11px] font-medium transition-all ${
                        isActive ? "text-white" : "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
                      }`}
                      style={isActive ? { backgroundColor: appColor, color: "#fff" } : undefined}
                    >
                      <SvgIcon name={item.icon} />
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
