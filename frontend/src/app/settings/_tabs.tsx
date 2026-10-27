import type { ComponentType } from "react";
import type { User } from "@/lib/types";
import IntegrationsTab from "./IntegrationsTab";
import GitLabReposTab from "./_integrations/GitLabReposTab";
import PermissionsTab from "./PermissionsTab";
import RoleMappingsTab from "./RoleMappingsTab";
import EntidadesTab from "./EntidadesTab";
import OfferingsTab from "./OfferingsTab";
import EnumsTab from "./EnumsTab";
import UsersTab from "./UsersTab";
import AppearanceTab from "./AppearanceTab";
import ImportTab from "./ImportTab";
import BackupTab from "./BackupTab";
import SSHCustomOpsTab from "./SSHCustomOpsTab";
import ApiTokensPanel from "@/components/tokens/ApiTokensPanel";

/** One Settings section: its URL (/settings/<key>), its label and side-nav
 *  group (i18n keys), who sees it and what it renders. Order = nav order. */
export interface SettingsTab {
  key: string;
  label: string;
  group: string;
  minRole?: "editor" | "admin";
  Component: ComponentType;
}

export const SETTINGS_TABS: SettingsTab[] = [
  { key: "enums", label: "settings.enums", group: "settings.groups.general", Component: EnumsTab },
  { key: "appearance", label: "settings.appearance", group: "settings.groups.general", minRole: "admin", Component: AppearanceTab },
  { key: "users", label: "settings.users", group: "settings.groups.access", minRole: "admin", Component: UsersTab },
  { key: "entidades", label: "entidades.title", group: "settings.groups.access", minRole: "admin", Component: EntidadesTab },
  { key: "permissions", label: "settings.permissions.tabLabel", group: "settings.groups.access", minRole: "admin", Component: PermissionsTab },
  { key: "role-mappings", label: "settings.roleMappings.tabLabel", group: "settings.groups.access", minRole: "admin", Component: RoleMappingsTab },
  // Every role manages its own tokens.
  { key: "api-tokens", label: "settings.apiTokens.tabLabel", group: "settings.groups.access", Component: ApiTokensPanel },
  // ponytail: role gate until the catalog.manage permission ships.
  { key: "offerings", label: "settings.offerings.title", group: "settings.groups.catalog", minRole: "editor", Component: OfferingsTab },
  { key: "ssh-operations", label: "settings.sshOps.title", group: "settings.groups.hosts", minRole: "admin", Component: SSHCustomOpsTab },
  { key: "integrations", label: "settings.integrationsTabLabel", group: "settings.groups.integrations", minRole: "admin", Component: IntegrationsTab },
  { key: "gitlab-repos", label: "settings.gitlabRepos.tabLabel", group: "settings.groups.integrations", minRole: "admin", Component: GitLabReposTab },
  { key: "import", label: "settings.import", group: "settings.groups.data", minRole: "admin", Component: ImportTab },
  { key: "backup", label: "settings.backup", group: "settings.groups.data", minRole: "admin", Component: BackupTab },
];

export const DEFAULT_SETTINGS_TAB = "enums";

const RANK: Record<User["role"], number> = { viewer: 0, editor: 1, admin: 2 };

export function canSeeTab(tab: SettingsTab, role: User["role"] | undefined): boolean {
  return !tab.minRole || (!!role && RANK[role] >= RANK[tab.minRole]);
}
