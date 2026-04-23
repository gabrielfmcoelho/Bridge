"use client";

import { useState, useRef, useEffect } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useTheme } from "@/contexts/ThemeContext";
import { useRouter, usePathname } from "next/navigation";
import Drawer from "@/components/ui/Drawer";
import AiChatDrawer from "@/components/ai/AiChatDrawer";

const roleColors: Record<string, string> = {
  admin: "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]",
  editor: "bg-purple-500/10 text-purple-400/70 border-purple-500/15",
  viewer: "bg-[var(--bg-overlay)] text-[var(--text-faint)] border-[var(--border-subtle)]",
};

interface HeaderProps {
  onToggleCollapse: () => void;
  collapsed: boolean;
}

export default function Header({ onToggleCollapse, collapsed }: HeaderProps) {
  const { user, logout } = useAuth();
  const { locale, setLocale, t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const { theme, toggleTheme } = useTheme();
  const router = useRouter();
  const pathname = usePathname();
  // Route → title mapping for the global header. Extend as other pages want
  // to surface their name next to the collapse button.
  const headerTitle =
    pathname === "/wiki" || pathname?.startsWith("/wiki/") ? "Wiki" : "";
  const [userDrawer, setUserDrawer] = useState(false);
  const [userMenu, setUserMenu] = useState(false);
  const [aiChat, setAiChat] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const hasAiPermission = user?.permissions?.includes("ai.use") ?? false;

  const handleLogout = async () => {
    setUserDrawer(false);
    setUserMenu(false);
    await logout();
    router.push("/login");
  };

  // Close desktop dropdown on outside click
  useEffect(() => {
    if (!userMenu) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setUserMenu(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [userMenu]);

  return (
    <header className="h-13 bg-[var(--bg-surface)] border-b border-[var(--border-subtle)] flex items-center justify-between px-3 md:px-5 gap-2">
      {/* Left side */}
      <div className="flex items-center gap-1">
        {/* Mobile app branding */}
        <div className="md:hidden flex items-center gap-2">
          <div className="w-7 h-7 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden shrink-0"
            style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
            ) : (
              <svg className="w-3.5 h-3.5" style={{ color: appColor }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
              </svg>
            )}
          </div>
          <span className="text-sm font-bold text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-display)" }}>
            {appName}
          </span>
        </div>
        {/* Desktop collapse toggle */}
        <button
          onClick={onToggleCollapse}
          className="hidden md:flex w-8 h-8 items-center justify-center rounded-[var(--radius-md)] text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)] transition-colors"
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          <svg className={`w-4 h-4 transition-transform duration-200 ${collapsed ? "rotate-180" : ""}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
          </svg>
        </button>
        {headerTitle && (
          <h1
            className="hidden md:block ml-2 text-sm font-semibold text-[var(--text-primary)] truncate"
            style={{ fontFamily: "var(--font-display)" }}
          >
            {headerTitle}
          </h1>
        )}
      </div>

      {/* AI Chat Drawer */}
      <AiChatDrawer open={aiChat} onClose={() => setAiChat(false)} />

      {/* Right side — Desktop */}
      <div className="hidden md:flex items-center gap-3">
        {/* AI chat trigger */}
        {hasAiPermission && (
          <button
            onClick={() => setAiChat(true)}
            className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--accent)] transition-colors"
            title="AI Assistant"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 00-2.455 2.456z" />
            </svg>
          </button>
        )}
        {/* Locale toggle */}
        <div className="flex h-8 rounded-[var(--radius-md)] border border-[var(--border-default)] overflow-hidden">
          <button
            onClick={() => setLocale("en")}
            className={`px-3 text-[11px] font-medium transition-all duration-150 ${
              locale === "en"
                ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            }`}
          >
            EN
          </button>
          <button
            onClick={() => setLocale("pt-BR")}
            className={`px-3 text-[11px] font-medium border-l border-[var(--border-default)] transition-all duration-150 ${
              locale === "pt-BR"
                ? "bg-[var(--accent-muted)] text-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
            }`}
          >
            PT
          </button>
        </div>

        {/* Theme toggle — sized to match the locale pair and the user avatar */}
        <button
          onClick={toggleTheme}
          aria-label={t("header.toggleTheme")}
          title={theme === "dark" ? t("header.themeLight") : t("header.themeDark")}
          className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-muted)] hover:text-[var(--accent)] hover:bg-[var(--bg-elevated)] transition-colors"
        >
          {theme === "dark" ? (
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2m0 14v2M5.05 5.05l1.41 1.41M17.54 17.54l1.41 1.41M3 12h2m14 0h2M5.05 18.95l1.41-1.41M17.54 6.46l1.41-1.41" />
              <circle cx="12" cy="12" r="4" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
          ) : (
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M21 12.79A9 9 0 1111.21 3a7 7 0 009.79 9.79z" />
            </svg>
          )}
        </button>

        {/* Desktop user avatar + dropdown */}
        {user && (
          <div className="relative" ref={menuRef}>
            <button
              onClick={() => setUserMenu(!userMenu)}
              className="w-8 h-8 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-xs font-semibold text-[var(--text-secondary)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-overlay)] transition-all"
            >
              {(user.display_name || user.username).charAt(0).toUpperCase()}
            </button>

            {userMenu && (
              <div className="absolute right-0 top-full mt-2 w-56 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-lg)] shadow-[var(--shadow-lg)] overflow-hidden z-50 animate-fade-in">
                {/* User info */}
                <div className="px-4 py-3 border-b border-[var(--border-subtle)]">
                  <div className="flex items-center gap-2.5">
                    <div className="w-9 h-9 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-sm font-semibold text-[var(--text-secondary)]">
                      {(user.display_name || user.username).charAt(0).toUpperCase()}
                    </div>
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-[var(--text-primary)] truncate">
                        {user.display_name || user.username}
                      </p>
                      <p className="text-[11px] text-[var(--text-faint)] mt-0.5">@{user.username}</p>
                      <span className={`inline-block mt-1 text-[9px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[user.role] || roleColors.viewer}`}>
                        {user.role}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Logout */}
                <div className="p-1.5">
                  <button
                    onClick={handleLogout}
                    className="w-full flex items-center gap-2 px-3 py-2 text-sm rounded-[var(--radius-md)] text-red-400 hover:bg-red-500/10 transition-colors"
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
                    </svg>
                    {t("auth.logout")}
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Right side — Mobile: avatar button opens drawer */}
      {user && (
        <button
          onClick={() => setUserDrawer(true)}
          className="md:hidden ml-auto w-8 h-8 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-xs font-semibold text-[var(--text-secondary)]"
        >
          {(user.display_name || user.username).charAt(0).toUpperCase()}
        </button>
      )}

      {/* Mobile user drawer */}
      <div className="md:hidden">
        <Drawer open={userDrawer} onClose={() => setUserDrawer(false)}>
          {user && (
            <div className="space-y-4">
              {/* User info */}
              <div className="flex items-center gap-3 pb-4 border-b border-[var(--border-subtle)]">
                <div className="w-12 h-12 rounded-full bg-[var(--bg-elevated)] border border-[var(--border-default)] flex items-center justify-center text-lg font-semibold text-[var(--text-secondary)]">
                  {(user.display_name || user.username).charAt(0).toUpperCase()}
                </div>
                <div>
                  <p className="font-semibold text-[var(--text-primary)]">{user.display_name || user.username}</p>
                  <p className="text-xs text-[var(--text-muted)]">@{user.username}</p>
                  <span className={`inline-block mt-1 text-[10px] px-1.5 py-0.5 rounded-full border font-medium ${roleColors[user.role] || roleColors.viewer}`}>
                    {user.role}
                  </span>
                </div>
              </div>

              {/* Language */}
              <div>
                <p className="text-xs text-[var(--text-faint)] mb-2 font-medium uppercase tracking-wider">Language</p>
                <div className="flex gap-2">
                  <button
                    onClick={() => setLocale("en")}
                    className={`flex-1 py-2.5 rounded-[var(--radius-md)] text-sm font-medium transition-all ${
                      locale === "en"
                        ? "bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border border-[var(--border-subtle)]"
                    }`}
                  >
                    English
                  </button>
                  <button
                    onClick={() => setLocale("pt-BR")}
                    className={`flex-1 py-2.5 rounded-[var(--radius-md)] text-sm font-medium transition-all ${
                      locale === "pt-BR"
                        ? "bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border border-[var(--border-subtle)]"
                    }`}
                  >
                    Português
                  </button>
                </div>
              </div>

              {/* Theme */}
              <div>
                <p className="text-xs text-[var(--text-faint)] mb-2 font-medium uppercase tracking-wider">{t("header.themeLabel")}</p>
                <div className="flex gap-2">
                  <button
                    onClick={() => theme === "light" && toggleTheme()}
                    className={`flex-1 py-2.5 rounded-[var(--radius-md)] text-sm font-medium transition-all flex items-center justify-center gap-2 ${
                      theme === "dark"
                        ? "bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border border-[var(--border-subtle)]"
                    }`}
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M21 12.79A9 9 0 1111.21 3a7 7 0 009.79 9.79z" />
                    </svg>
                    {t("header.themeDark")}
                  </button>
                  <button
                    onClick={() => theme === "dark" && toggleTheme()}
                    className={`flex-1 py-2.5 rounded-[var(--radius-md)] text-sm font-medium transition-all flex items-center justify-center gap-2 ${
                      theme === "light"
                        ? "bg-[var(--accent-muted)] text-[var(--accent)] border border-[var(--accent)]/20"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border border-[var(--border-subtle)]"
                    }`}
                  >
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2m0 14v2M5.05 5.05l1.41 1.41M17.54 17.54l1.41 1.41M3 12h2m14 0h2M5.05 18.95l1.41-1.41M17.54 6.46l1.41-1.41" />
                      <circle cx="12" cy="12" r="4" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                    {t("header.themeLight")}
                  </button>
                </div>
              </div>

              {/* Logout */}
              <button
                onClick={handleLogout}
                className="w-full py-2.5 rounded-[var(--radius-md)] text-sm font-medium bg-red-500/10 text-red-400 border border-red-500/20 hover:bg-red-500/20 transition-all"
              >
                {t("auth.logout")}
              </button>
            </div>
          )}
        </Drawer>
      </div>
    </header>
  );
}
