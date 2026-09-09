// Fallbacks when the situacao enum carries no colour. Semantic tokens so they
// follow the theme (the light-mode values are WCAG-tuned in globals.css).
export const SITUACAO_COLORS: Record<string, string> = {
  active: "bg-[var(--success)]/15 text-[var(--success)] border-[var(--success)]/30",
  inactive: "bg-[var(--bg-overlay)] text-[var(--text-secondary)] border-[var(--border-default)]",
  maintenance: "bg-[var(--warning)]/15 text-[var(--warning)] border-[var(--warning)]/30",
};

export const SITUACAO_DOT_COLORS: Record<string, string> = {
  active: "bg-[var(--success)]",
  inactive: "bg-[var(--text-faint)]",
  maintenance: "bg-[var(--warning)]",
};

// Card accent for an entity's situacao: the enum's own colour when it has one,
// otherwise the semantic token. Replaces the per-card hex fallbacks.
export function situacaoAccent(situacao: string | undefined, enumColor?: string): string {
  if (enumColor) return enumColor;
  return situacao === "active" ? "success" : situacao === "maintenance" ? "warning" : "muted";
}

// ponytail: `role` gates by users.role; `permission` by permission code. Permission codes are
// empty under dev:mock (mocks/seed.ts), so admin-only dev pages use `role` instead.
export type NavItem = { href: string; label: string; icon: string; permission?: string; role?: "admin" };
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
    key: "serviceCatalog",
    items: [
      { href: "/catalog", label: "nav.serviceCatalog", icon: "Boxes" },
      { href: "/requests", label: "nav.requests", icon: "ClipboardList" },
    ],
  },
  {
    key: "management",
    label: "nav.management",
    items: [
      { href: "/projects", label: "nav.projects", icon: "FolderKanban" },
      { href: "/releases", label: "nav.releases", icon: "Rocket" },
      { href: "/contacts", label: "nav.contacts", icon: "Users" },
      { href: "/trash", label: "nav.trash", icon: "Trash2" },
    ],
  },
  {
    key: "credentials",
    label: "nav.credentials",
    items: [
      { href: "/ssh-keys", label: "nav.hostCredentials", icon: "Key" },
      // Unified secrets manager (Phase 1–4). Covers service credentials,
      // host passwords/keys, env-var bundles, app logins, and share links.
      // The legacy /service-credentials route 308-redirects here.
      { href: "/secrets", label: "nav.vault", icon: "Lock" },
    ],
  },
  {
    key: "atlas",
    label: "nav.atlas",
    items: [
      { href: "/atlas/topology", label: "nav.topology", icon: "Network" },
      { href: "/atlas/lineage", label: "nav.lineage", icon: "GitBranch" },
      { href: "/atlas/catalog", label: "nav.catalog", icon: "Database" },
      { href: "/atlas/pipeline", label: "nav.pipeline", icon: "Workflow" },
      { href: "/atlas/apis", label: "nav.apis", icon: "Plug" },
    ],
  },
  {
    key: "tools",
    label: "nav.toolsSection",
    items: [
      { href: "/ssh-config", label: "nav.sshConfig", icon: "Terminal" },
      { href: "/tools", label: "nav.tools", icon: "Wrench" },
      { href: "/settings", label: "nav.settings", icon: "Settings" },
      { href: "/design-system", label: "nav.designSystem", icon: "LayoutDashboard", role: "admin" },
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
