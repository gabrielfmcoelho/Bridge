"use client";

import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useTheme } from "@/contexts/ThemeContext";
import Avatar from "@/components/ui/Avatar";
import PillButton from "@/components/ui/PillButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { ROLE_COLORS } from "@/lib/constants";

/**
 * The profile menu's body: who is signed in, then the per-user preferences
 * (language, theme). Shared by the desktop dropdown and the mobile drawer;
 * each host renders its own logout action, since only the dropdown's closes
 * the popover.
 */
export default function ProfilePanel() {
  const { user } = useAuth();
  const { locale, setLocale, t } = useLocale();
  const { theme, toggleTheme } = useTheme();
  if (!user) return null;
  const name = user.display_name || user.username;

  const preference = (label: string, children: React.ReactNode) => (
    <div>
      <p className="text-xs font-medium text-[var(--text-muted)] mb-1.5">{label}</p>
      <div className="flex gap-1.5">{children}</div>
    </div>
  );

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2.5">
        <Avatar name={name} size="md" />
        <div className="min-w-0">
          <p className="text-sm font-medium text-[var(--text-primary)] truncate">{name}</p>
          <p className="text-xs text-[var(--text-muted)] truncate">@{user.username}</p>
        </div>
        <span className={`ml-auto shrink-0 text-2xs px-1.5 py-0.5 rounded-full border font-medium ${ROLE_COLORS[user.role] ?? ROLE_COLORS.viewer}`}>
          {user.role}
        </span>
      </div>

      {/* Language names stay in their own language, so they read the same in both. */}
      {preference(t("header.language"), (
        <>
          <PillButton active={locale === "pt-BR"} onClick={() => setLocale("pt-BR")} className="flex-1 justify-center">Português</PillButton>
          <PillButton active={locale === "en"} onClick={() => setLocale("en")} className="flex-1 justify-center">English</PillButton>
        </>
      ))}

      {preference(t("header.themeLabel"), (
        <>
          <PillButton
            active={theme === "light"}
            onClick={() => theme !== "light" && toggleTheme()}
            lead={<Icon path={ICON_PATHS.sun} className="w-3.5 h-3.5" />}
            className="flex-1 justify-center"
          >
            {t("header.themeLight")}
          </PillButton>
          <PillButton
            active={theme === "dark"}
            onClick={() => theme !== "dark" && toggleTheme()}
            lead={<Icon path={ICON_PATHS.moon} className="w-3.5 h-3.5" />}
            className="flex-1 justify-center"
          >
            {t("header.themeDark")}
          </PillButton>
        </>
      ))}
    </div>
  );
}
