"use client";

import { useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import Drawer from "@/components/ui/Drawer";
import DropdownMenu, { DropdownMenuItem } from "@/components/ui/DropdownMenu";
import { requestOpen } from "@/hooks/useOpenOnParam";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";
import Avatar from "@/components/ui/Avatar";
import Divider from "@/components/ui/Divider";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import AiChatDrawer from "@/components/ai/AiChatDrawer";
import Breadcrumbs from "./Breadcrumbs";
import CommandPalette from "./CommandPalette";
import ProfilePanel from "./ProfilePanel";

// "Create" menu: each list page opens its form on ?new=1 (useOpenOnParam).
const CREATE_ITEMS = [
  { href: "/hosts", label: "create.host", icon: NAV_ICONS.Server },
  { href: "/dns", label: "create.dns", icon: NAV_ICONS.Globe },
  { href: "/services", label: "create.service", icon: NAV_ICONS.Boxes },
  { href: "/projects", label: "create.project", icon: NAV_ICONS.FolderKanban },
  { href: "/contacts", label: "create.contact", icon: NAV_ICONS.Users },
];

interface HeaderProps {
  onToggleCollapse: () => void;
  collapsed: boolean;
}

export default function Header({ onToggleCollapse, collapsed }: HeaderProps) {
  const { user, logout } = useAuth();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const router = useRouter();
  const pathname = usePathname();
  const [userDrawer, setUserDrawer] = useState(false);
  const [aiChat, setAiChat] = useState(false);
  const hasAiPermission = user?.permissions?.includes("ai.use") ?? false;
  const name = user ? user.display_name || user.username : "";
  const canEdit = user?.role === "admin" || user?.role === "editor";

  // Already on the page: it stays mounted on a same-path push, so ask it directly.
  const create = (href: string) => (pathname === href ? requestOpen("new") : router.push(`${href}?new=1`));

  const handleLogout = async () => {
    setUserDrawer(false);
    await logout();
    router.push("/login");
  };

  return (
    <header className="h-13 bg-[var(--bg-surface)] border-b border-[var(--border-subtle)] flex items-center justify-between px-3 md:px-5 gap-2">
      {/* Left side */}
      {/* Left takes the flexible space and truncates, so a deep breadcrumb
          never pushes the tools on the right. */}
      <div className="flex-1 flex items-center gap-1 min-w-0 overflow-hidden">
        {/* Mobile app branding */}
        <div className="md:hidden flex items-center gap-2">
          <div className="w-7 h-7 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden shrink-0"
            style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
            ) : (
              <Icon path={ICON_PATHS.prompt} className="w-3.5 h-3.5" style={{ color: appColor }} />
            )}
          </div>
          <span className="text-sm font-bold text-[var(--text-primary)] truncate font-display">{appName}</span>
        </div>
        <IconButton
          onClick={onToggleCollapse}
          className="max-md:hidden"
          label={collapsed ? t("header.expandSidebar") : t("header.collapseSidebar")}
        >
          <Icon path={ICON_PATHS.chevronsLeft} className={`w-4 h-4 transition-transform duration-200 ${collapsed ? "rotate-180" : ""}`} />
        </IconButton>
        <Breadcrumbs />
      </div>

      <AiChatDrawer open={aiChat} onClose={() => setAiChat(false)} />

      {/* Right, fixed order: search · Assistente · Criar · profile. */}
      <div className="shrink-0 flex items-center gap-2">
        <CommandPalette />
        {hasAiPermission && (
          <>
            <Button size="sm" variant="secondary" onClick={() => setAiChat(true)} className="max-md:hidden">
              <Icon path={ICON_PATHS.sparkles} className="w-4 h-4" />
              {t("header.assistant")}
            </Button>
            <IconButton onClick={() => setAiChat(true)} label={t("header.assistant")} className="md:hidden">
              <Icon path={ICON_PATHS.sparkles} className="w-4 h-4" />
            </IconButton>
          </>
        )}
      </div>

      {/* Right side — Desktop. Language and theme live in the profile menu. */}
      <div className="hidden md:flex items-center gap-2 shrink-0">
        {canEdit && (
          <DropdownMenu
            className="w-44"
            trigger={
              <Button size="sm">
                <Icon path={ICON_PATHS.plus} className="w-4 h-4" />
                {t("common.create")}
                <Icon path={ICON_PATHS.chevronDown} className="w-3.5 h-3.5" />
              </Button>
            }
          >
            <div className="py-1">
              {CREATE_ITEMS.map((item) => (
                <DropdownMenuItem key={item.href} onClick={() => create(item.href)} elemBefore={<Icon path={item.icon} className="w-4 h-4" strokeWidth={1.5} />}>
                  {t(item.label)}
                </DropdownMenuItem>
              ))}
            </div>
          </DropdownMenu>
        )}
        {user && (
          <DropdownMenu
            className="w-72"
            trigger={
              <button
                type="button"
                aria-label={t("header.profileMenu")}
                className="rounded-full transition duration-150 hover:brightness-110 active:scale-[0.95]"
              >
                <Avatar name={name} />
              </button>
            }
          >
            <div className="p-4">
              <ProfilePanel />
            </div>
            <Divider />
            <div className="p-1.5">
              <DropdownMenuItem danger onClick={handleLogout} className="rounded-[var(--radius-md)] py-2" elemBefore={<Icon path={ICON_PATHS.logout} className="w-4 h-4" />}>
                {t("auth.logout")}
              </DropdownMenuItem>
            </div>
          </DropdownMenu>
        )}
      </div>

      {/* Right side — Mobile: the same panel in a drawer */}
      {user && (
        <button
          type="button"
          onClick={() => setUserDrawer(true)}
          aria-label={t("header.profileMenu")}
          className="md:hidden rounded-full"
        >
          <Avatar name={name} />
        </button>
      )}
      <div className="md:hidden">
        <Drawer
          open={userDrawer}
          onClose={() => setUserDrawer(false)}
          title={t("header.profileMenu")}
          footer={
            <Button variant="danger" className="w-full" onClick={handleLogout}>
              <Icon path={ICON_PATHS.logout} className="w-4 h-4" />
              {t("auth.logout")}
            </Button>
          }
        >
          <ProfilePanel />
        </Drawer>
      </div>
    </header>
  );
}
