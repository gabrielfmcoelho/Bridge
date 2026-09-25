"use client";

import { useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import { type SectionNavGroup } from "@/components/ui/SectionNav";
import IntegrationsTab from "./IntegrationsTab";
import PermissionsTab from "./PermissionsTab";
import RoleMappingsTab from "./RoleMappingsTab";
import EntidadesTab from "./EntidadesTab";
import OfferingsTab from "./OfferingsTab";
import EnumsTab from "./EnumsTab";
import UsersTab from "./UsersTab";
import AppearanceTab from "./AppearanceTab";
import ImportTab from "./ImportTab";
import BackupTab from "./BackupTab";

type Tab = "enums" | "users" | "entidades" | "offerings" | "appearance" | "import" | "backup" | "integrations" | "permissions" | "role-mappings";

export default function SettingsPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const isAdmin = user?.role === "admin";
  // ponytail: role gate until Phase B ships the catalog.manage permission;
  // then this becomes user?.permissions?.includes("catalog.manage").
  const canManageCatalog = isAdmin || user?.role === "editor";
  const [activeTab, setActiveTab] = useState<Tab>("enums");

  // Grouped like a side nav (ADS): ten flat tabs don't fit a phone and don't
  // say how the sections relate.
  const groups: SectionNavGroup<Tab>[] = [
    { title: t("settings.groups.general"), items: [
      { key: "enums", label: t("settings.enums") },
      ...(isAdmin ? [{ key: "appearance" as Tab, label: t("settings.appearance") }] : []),
    ] },
    { title: t("settings.groups.access"), items: isAdmin ? [
      { key: "users", label: t("settings.users") },
      { key: "entidades", label: t("entidades.title") },
      { key: "permissions", label: t("settings.permissions.tabLabel") },
      { key: "role-mappings", label: t("settings.roleMappings.tabLabel") },
    ] : [] },
    { title: t("settings.groups.catalog"), items: canManageCatalog ? [{ key: "offerings", label: t("settings.offerings.title") }] : [] },
    { title: t("settings.groups.integrations"), items: isAdmin ? [{ key: "integrations", label: t("settings.integrationsTabLabel") }] : [] },
    { title: t("settings.groups.data"), items: isAdmin ? [
      { key: "import", label: t("settings.import") },
      { key: "backup", label: t("settings.backup") },
    ] : [] },
  ];

  return (
    <PageShell>
      {/* Side tabs: PageHeader draws the grouped list beside the panel. */}
      <PageHeader
        title={t("settings.title")}
        tabs={{
          idBase: "settings",
          label: t("settings.title"),
          variant: "side",
          active: activeTab,
          onChange: (k) => setActiveTab(k as Tab),
          items: groups.flatMap((g) => g.items.map((i) => ({ ...i, group: g.title }))),
          panelClassName: "animate-fade-in",
        }}
      >
        {activeTab === "enums" && <EnumsTab />}
        {activeTab === "users" && isAdmin && <UsersTab />}
        {activeTab === "entidades" && isAdmin && <EntidadesTab />}
        {activeTab === "offerings" && canManageCatalog && <OfferingsTab />}
        {activeTab === "appearance" && isAdmin && <AppearanceTab />}
        {activeTab === "import" && isAdmin && <ImportTab />}
        {activeTab === "backup" && isAdmin && <BackupTab />}
        {activeTab === "integrations" && isAdmin && <IntegrationsTab />}
        {activeTab === "permissions" && isAdmin && <PermissionsTab />}
        {activeTab === "role-mappings" && isAdmin && <RoleMappingsTab />}
      </PageHeader>
    </PageShell>
  );
}
