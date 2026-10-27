"use client";

import type { ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import { DEFAULT_SETTINGS_TAB, SETTINGS_TABS, canSeeTab } from "./_tabs";

/** Settings: the grouped side nav stays put while each section is its own
 *  page (/settings/<key>), so a section can be linked, bookmarked and reloaded. */
export default function SettingsLayout({ children }: { children: ReactNode }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const active = pathname.split("/")[2] || DEFAULT_SETTINGS_TAB;
  const items = SETTINGS_TABS.filter((tab) => canSeeTab(tab, user?.role))
    .map((tab) => ({ key: tab.key, label: t(tab.label), group: t(tab.group) }));

  return (
    <PageShell>
      {/* Side tabs: PageHeader draws the grouped list beside the panel. */}
      <PageHeader
        title={t("settings.title")}
        tabs={{
          idBase: "settings",
          label: t("settings.title"),
          variant: "side",
          active,
          onChange: (k) => router.push(`/settings/${k}`),
          items,
          panelClassName: "animate-fade-in",
        }}
      >
        {children}
      </PageHeader>
    </PageShell>
  );
}
