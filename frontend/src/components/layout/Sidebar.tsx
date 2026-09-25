"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { dashboardAPI } from "@/lib/api";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import { usePathname } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useAuth } from "@/contexts/AuthContext";
import { NAV_SECTIONS, canSeeNavItem, type NavItem, type NavSection } from "@/lib/constants";
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
      items: section.items.filter((item) => canSeeNavItem(item, permissions, role)),
    }))
    .filter((section) => section.items.length > 0);
}

export default function Sidebar({ collapsed, mobileOpen, onCloseMobile }: SidebarProps) {
  const pathname = usePathname();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const { user } = useAuth();
  const sections = filterSections(NAV_SECTIONS, user?.permissions ?? [], user?.role);
  // Collapsed, the rail opens while the pointer (or keyboard focus) is on it,
  // floating over the page so the content underneath doesn't reflow.
  const [peek, setPeek] = useState(false);
  const expanded = !collapsed || peek;
  // Hover intent (ADS navigation-system timings): open after 500ms on the rail,
  // close 400ms after leaving, so sweeping the pointer past the edge — or
  // briefly overshooting while inside — doesn't flash the panel.
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const schedule = (open: boolean, ms: number) => {
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setPeek(open), ms);
  };
  useEffect(() => () => clearTimeout(timer.current), []);

  // Live counts after a label (ADS elemAfter). Same query key as the
  // dashboard, so it is usually already cached.
  const { data: stats } = useQuery({ queryKey: ["dashboard"], queryFn: dashboardAPI.get, staleTime: 60_000 });
  const counts: Record<NonNullable<NavItem["count"]>, { n: number; label: string }> = {
    openIssues: { n: stats?.open_issues ?? 0, label: t("host.openIssuesTitle", { count: String(stats?.open_issues ?? 0) }) },
  };
  const [closedSections, setClosedSections] = useLocalStorage<Record<string, boolean>>("sidebar_closed_sections", {});
  const closed = (sec: NavSection) =>
    !!sec.collapsible && !!closedSections[sec.key] && !sec.items.some((i) => isActiveHref(i.href));
  const isActiveHref = (href: string) => (href === "/" ? pathname === "/" : pathname.startsWith(href));

  // Labels stay mounted and fade, so opening animates as one motion: the width
  // grows (overflow clipped) while the text fades in. Icons never move — they
  // sit at the same x in the 64px rail and the 240px panel.
  const fade = `whitespace-nowrap transition-opacity duration-150 ${expanded ? "opacity-100" : "opacity-0"}`;
  const itemClass = "group flex items-center gap-2.5 px-3 py-2 rounded-[var(--radius-md)] text-sm font-medium";

  const sidebarContent = (
    <aside
      onMouseEnter={() => collapsed && schedule(true, 500)}
      onMouseLeave={() => schedule(false, 400)}
      // Keyboard focus opens at once: there is no accidental sweep to filter.
      onFocus={() => { if (collapsed) { clearTimeout(timer.current); setPeek(true); } }}
      onBlur={(e) => !e.currentTarget.contains(e.relatedTarget) && schedule(false, 0)}
      className={`bg-[var(--bg-surface)] border-r border-[var(--border-subtle)] flex flex-col h-full overflow-hidden transition-[width] duration-200 ease-out ${
        expanded ? "w-60" : "w-16"
      } ${collapsed ? "absolute inset-y-0 left-0 z-40" : ""} ${peek ? "shadow-[var(--elevate-hi)]" : ""}`}
    >
      {/* Branding — matches header h-13 */}
      <div className="h-13 px-4 flex items-center border-b border-[var(--border-subtle)] shrink-0">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="w-8 h-8 rounded-[var(--radius-md)] flex items-center justify-center overflow-hidden shrink-0"
            style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
            ) : (
              <Icon path={ICON_PATHS.prompt} />
            )}
          </div>
          <div className={`min-w-0 ${fade}`} aria-hidden={!expanded}>
            <h1 className="text-sm font-bold text-[var(--text-primary)] truncate font-display">
              {appName}
            </h1>
            <p className="text-2xs text-[var(--text-muted)] leading-tight">{t("app.subtitle")}</p>
          </div>
        </div>
      </div>

      {/* Navigation by sections */}
      <nav className="flex-1 p-2.5 overflow-y-auto overflow-x-hidden">
        {sections.map((section, sIdx) => (
          <div key={section.key}>
            {sIdx > 0 && <Divider className="my-3" />}
            {/* The name folds away with the rail (0fr ↔ 1fr row), so the groups
                slide together instead of jumping. */}
            {section.label && (
              <div className={`grid transition-[grid-template-rows] duration-200 ease-out ${expanded ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}>
                {section.collapsible ? (
                  <button
                    type="button"
                    onClick={() => setClosedSections((c) => ({ ...c, [section.key]: !c[section.key] }))}
                    aria-expanded={!closed(section)}
                    aria-controls={`nav-section-${section.key}`}
                    tabIndex={expanded ? 0 : -1}
                    className={`overflow-hidden flex w-full items-center justify-between px-3 text-2xs font-semibold text-[var(--text-muted)] hover:text-[var(--text-secondary)] ${fade}`}
                  >
                    <span className="block pb-1.5">{t(section.label)}</span>
                    <Icon path={ICON_PATHS.chevronRight} className={`mb-1.5 w-3 h-3 transition-transform duration-150 ${closed(section) ? "" : "rotate-90"}`} />
                  </button>
                ) : (
                  <p className={`overflow-hidden px-3 text-2xs font-semibold text-[var(--text-muted)] ${fade}`} aria-hidden={!expanded}>
                    <span className="block pb-1.5">{t(section.label)}</span>
                  </p>
                )}
              </div>
            )}
            {/* A folded section keeps its items in the collapsed rail (icons are the
                only way in there) and never hides the page you are on. */}
            <div
              id={`nav-section-${section.key}`}
              className={`grid transition-[grid-template-rows] duration-200 ease-out ${expanded && closed(section) ? "grid-rows-[0fr]" : "grid-rows-[1fr]"}`}
            >
            <div className="space-y-0.5 overflow-hidden">
              {section.items.map((item) => {
                const icon = <Icon path={NAV_ICONS[item.icon] || NAV_ICONS.Server} className="w-[18px] h-[18px] shrink-0" strokeWidth={1.5} />;
                const label = <span className={fade}>{t(item.label)}</span>;
                if (item.href === "#") {
                  return (
                    <span
                      key={item.label}
                      title={t("common.comingSoon")}
                      aria-label={expanded ? undefined : t(item.label)}
                      className={`${itemClass} opacity-40 cursor-not-allowed text-[var(--text-muted)]`}
                    >
                      {icon}
                      {label}
                    </span>
                  );
                }
                const isActive = isActiveHref(item.href);
                const count = item.count && counts[item.count].n > 0 ? counts[item.count] : null;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={onCloseMobile}
                    aria-label={expanded ? undefined : count ? `${t(item.label)}, ${count.label}` : t(item.label)}
                    aria-current={isActive ? "page" : undefined}
                    className={`${itemClass} transition duration-150 relative ${
                      !isActive ? "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]" : ""
                    }`}
                    // Active is the accent bar + accent icon and label; no fill.
                    style={isActive ? { color: appColor } : undefined}
                  >
                    {isActive && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full" style={{ backgroundColor: appColor }} />
                    )}
                    <span className="relative flex shrink-0">
                      {icon}
                      {/* Rail: a dot says "something here"; the number shows once open. */}
                      {count && !expanded && <span aria-hidden className="absolute -right-1 -top-1 w-2 h-2 rounded-full bg-[var(--accent)] ring-2 ring-[var(--bg-surface)]" />}
                    </span>
                    {label}
                    {count && (
                      <span className={`ml-auto rounded-full bg-[var(--bg-overlay)] px-1.5 text-2xs font-mono font-semibold text-[var(--text-secondary)] ${fade}`}>
                        <span aria-hidden>{count.n}</span>
                        <span className="sr-only">{count.label}</span>
                      </span>
                    )}
                  </Link>
                );
              })}
            </div>
            </div>
          </div>
        ))}
      </nav>

      <div className={`p-3 border-t border-[var(--border-subtle)] shrink-0 ${fade}`} aria-hidden={!expanded}>
        <p className="text-2xs text-[var(--text-muted)] text-center font-mono">v0.1.0</p>
      </div>
    </aside>
  );

  return (
    <>
      {/* Desktop sidebar */}
      {/* Collapsed, this keeps the rail's 64px in the layout while the aside
          is positioned over it (and over the page when it peeks open). */}
      <div className={`hidden md:block shrink-0 relative ${collapsed ? "w-16" : ""}`}>
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
                <p className="px-1 pb-1.5 text-2xs font-semibold text-[var(--text-muted)]">
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
