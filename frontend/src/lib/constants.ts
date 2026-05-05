export const SITUACAO_COLORS: Record<string, string> = {
  active: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
  inactive: "bg-gray-500/15 text-gray-400 border-gray-500/30",
  maintenance: "bg-yellow-500/15 text-yellow-400 border-yellow-500/30",
};

export const SITUACAO_DOT_COLORS: Record<string, string> = {
  active: "bg-emerald-400",
  inactive: "bg-gray-400",
  maintenance: "bg-yellow-400",
};

export type NavItem = { href: string; label: string; icon: string; permission?: string };
export type NavSection = { key: string; label?: string; items: NavItem[] };

export const NAV_SECTIONS: NavSection[] = [
  {
    key: "main",
    items: [
      { href: "/", label: "nav.dashboard", icon: "LayoutDashboard" },
      { href: "/issues", label: "nav.issues", icon: "ClipboardList" },
      { href: "/wiki", label: "nav.wiki", icon: "Book" },
      { href: "/chamados", label: "nav.chamados", icon: "Ticket" },
    ],
  },
  {
    key: "inventory",
    label: "nav.inventory",
    items: [
      { href: "/hosts", label: "nav.hosts", icon: "Server" },
      { href: "/dns", label: "nav.dns", icon: "Globe" },
      { href: "/services", label: "nav.services", icon: "Boxes" },
    ],
  },
  {
    key: "management",
    label: "nav.management",
    items: [
      { href: "/projects", label: "nav.projects", icon: "FolderKanban" },
      { href: "/releases", label: "nav.releases", icon: "Rocket" },
      { href: "/contacts", label: "nav.contacts", icon: "Users" },
    ],
  },
  {
    key: "credentials",
    label: "nav.credentials",
    items: [
      { href: "/ssh-keys", label: "nav.hostCredentials", icon: "Key" },
      { href: "/service-credentials", label: "nav.serviceCredentials", icon: "Lock" },
    ],
  },
  {
    key: "tools",
    label: "nav.toolsSection",
    items: [
      { href: "/topology", label: "nav.topology", icon: "Network" },
      { href: "/ssh-config", label: "nav.sshConfig", icon: "Terminal" },
      { href: "/tools", label: "nav.tools", icon: "Wrench" },
      { href: "/settings", label: "nav.settings", icon: "Settings" },
    ],
  },
];

// Flat list for mobile drawer and backward compat
export const NAV_ITEMS: NavItem[] = NAV_SECTIONS.flatMap(s => s.items);

/** Semantic colors for entity-link indicators on inventory cards. */
export const ENTITY_INDICATOR_COLORS = {
  hosts: "cyan",
  dns: "emerald",
  services: "amber",
  projects: "violet",
  containers: "sky",
  processes: "violet",
  dependencies: "amber",
  alerts: "amber",
  issues: "purple",
  chamados: "orange",
} as const;
