"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useQueries } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { hostsAPI, dnsAPI, servicesAPI, projectsAPI, contactsAPI } from "@/lib/api";
import { NAV_ITEMS, canSeeNavItem } from "@/lib/constants";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import { searchEntries, type SearchEntry, type SearchKind } from "@/lib/commandSearch";
import Modal from "@/components/ui/Modal";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";

const GROUP_LABEL: Record<SearchKind, string> = {
  page: "search.goTo",
  host: "nav.hosts",
  dns: "nav.dns",
  service: "nav.services",
  project: "nav.projects",
  contact: "nav.contacts",
};

const KIND_ICON: Record<Exclude<SearchKind, "page">, string> = {
  host: NAV_ICONS.Server,
  dns: NAV_ICONS.Globe,
  service: NAV_ICONS.Boxes,
  project: NAV_ICONS.FolderKanban,
  contact: NAV_ICONS.Users,
};

// Global search: Ctrl/⌘+K anywhere, or the trigger in the header's middle zone.
export default function CommandPalette() {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const listId = useId();
  const [open, setOpen] = useState(false);
  // Sources are fetched on first open, then kept (react-query cache).
  const [opened, setOpened] = useState(false);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);

  const show = () => { setOpen(true); setOpened(true); };
  const close = () => { setOpen(false); setQuery(""); setActive(0); };

  // Only Ctrl/⌘+K is handled, so it works from inside inputs too.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.altKey && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
        setOpened(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const staleTime = 60_000;
  const [hosts, dns, services, projects, contacts] = useQueries({
    queries: [
      // Distinct key: the hosts page's ["hosts", search, filters, sort] carries filter params.
      { queryKey: ["hosts", "palette"], queryFn: () => hostsAPI.list(), enabled: opened, staleTime },
      { queryKey: ["dns"], queryFn: dnsAPI.list, enabled: opened, staleTime },
      { queryKey: ["services"], queryFn: servicesAPI.list, enabled: opened, staleTime },
      { queryKey: ["projects"], queryFn: projectsAPI.list, enabled: opened, staleTime },
      { queryKey: ["contacts"], queryFn: contactsAPI.list, enabled: opened, staleTime },
    ],
  });
  const loading = [hosts, dns, services, projects, contacts].some((q) => q.isLoading);

  const entries = useMemo<SearchEntry[]>(() => {
    const perms = user?.permissions ?? [];
    const pages: SearchEntry[] = NAV_ITEMS
      .filter((i) => canSeeNavItem(i, perms, user?.role))
      .map((i) => ({ kind: "page", key: `page-${i.href}`, label: t(i.label), href: i.href, icon: NAV_ICONS[i.icon] }));
    return [
      ...pages,
      ...(hosts.data ?? []).map((h): SearchEntry => ({
        kind: "host", key: `host-${h.id}`, label: h.nickname, sub: h.oficial_slug, mono: true,
        href: `/hosts/${h.oficial_slug}`, terms: [h.hostname],
      })),
      ...(dns.data ?? []).map((d): SearchEntry => ({ kind: "dns", key: `dns-${d.id}`, label: d.domain, mono: true, href: `/dns/${d.id}` })),
      ...(services.data ?? []).map((s): SearchEntry => ({ kind: "service", key: `service-${s.id}`, label: s.nickname, href: `/services/${s.id}` })),
      ...(projects.data ?? []).map((p): SearchEntry => ({ kind: "project", key: `project-${p.id}`, label: p.name, href: `/projects/${p.id}` })),
      ...(contacts.data ?? []).map((c): SearchEntry => ({
        kind: "contact", key: `contact-${c.id}`, label: c.name, sub: c.entity || c.role || undefined, href: "/contacts",
      })),
    ];
  }, [t, user, hosts.data, dns.data, services.data, projects.data, contacts.data]);

  const groups = useMemo(() => searchEntries(entries, query), [entries, query]);
  const flat = groups.flatMap((g) => g.items);
  const current = flat[Math.min(active, flat.length - 1)];
  const optionId = (e: SearchEntry) => `${listId}-${e.key}`;

  useEffect(() => {
    if (current) document.getElementById(optionId(current))?.scrollIntoView({ block: "nearest" });
  });

  const go = (e: SearchEntry | undefined) => {
    if (!e) return;
    close();
    router.push(e.href);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!flat.length) return;
      const i = Math.min(active, flat.length - 1) + (e.key === "ArrowDown" ? 1 : -1);
      setActive((i + flat.length) % flat.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      go(current);
    }
  };

  // Mac shows ⌘; decided client-side only, hence suppressHydrationWarning on the hint.
  const shortcut = typeof navigator !== "undefined" && /Mac|iP(hone|ad)/.test(navigator.platform) ? "⌘K" : "Ctrl K";

  return (
    <>
      <button
        type="button"
        onClick={show}
        aria-keyshortcuts="Control+K Meta+K"
        className="hidden md:flex w-56 lg:w-64 items-center gap-2 h-8 px-3 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-base)] text-xs text-[var(--text-muted)] hover:border-[var(--border-strong)] hover:text-[var(--text-secondary)] transition duration-150"
      >
        <Icon path={ICON_PATHS.search} className="w-4 h-4 shrink-0" />
        <span className="flex-1 text-left truncate">{t("common.search")}</span>
        <kbd suppressHydrationWarning className="font-mono text-2xs px-1.5 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]">
          {shortcut}
        </kbd>
      </button>
      <IconButton onClick={show} className="md:hidden" label={t("search.open")}>
        <Icon path={ICON_PATHS.search} className="w-4 h-4" />
      </IconButton>

      <Modal open={open} onClose={close} size="md">
        <div className="flex items-center gap-2 border-b border-[var(--border-subtle)] pb-3 -mt-1">
          <Icon path={ICON_PATHS.search} className="w-4 h-4 shrink-0 text-[var(--text-muted)]" />
          <input
            autoFocus
            role="combobox"
            aria-expanded={flat.length > 0}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={current ? optionId(current) : undefined}
            aria-label={t("search.inputLabel")}
            placeholder={t("search.inputLabel")}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setActive(0); }}
            onKeyDown={onKeyDown}
            className="flex-1 min-w-0 bg-transparent text-base outline-none placeholder:text-[var(--text-muted)]"
          />
        </div>

        <div id={listId} role="listbox" aria-label={t("search.inputLabel")} className="mt-2 max-h-[60vh] overflow-y-auto">
          {groups.map((g) => (
            <div key={g.kind} role="group" aria-label={t(GROUP_LABEL[g.kind])} className="py-1">
              <p role="presentation" className="px-2 pt-1.5 pb-1 text-xs font-semibold text-[var(--text-muted)]">{t(GROUP_LABEL[g.kind])}</p>
              {g.items.map((e) => {
                const selected = e === current;
                return (
                  <div
                    key={e.key}
                    id={optionId(e)}
                    role="option"
                    aria-selected={selected}
                    onMouseMove={() => !selected && setActive(flat.indexOf(e))}
                    onClick={() => go(e)}
                    className={`flex items-center gap-2.5 px-2 py-1.5 rounded-[var(--radius-md)] cursor-pointer text-sm transition-colors ${
                      selected ? "bg-[var(--accent)]/10 text-[var(--text-primary)]" : "text-[var(--text-secondary)]"
                    }`}
                  >
                    <Icon
                      path={e.icon ?? (e.kind === "page" ? NAV_ICONS.Server : KIND_ICON[e.kind])}
                      className={`w-4 h-4 shrink-0 ${selected ? "text-[var(--accent)]" : "text-[var(--text-muted)]"}`}
                      strokeWidth={1.5}
                    />
                    <span className={`min-w-0 truncate ${e.mono && !e.sub ? "font-mono" : ""}`}>{e.label}</span>
                    {e.sub && <span className={`min-w-0 truncate text-xs text-[var(--text-muted)] ${e.mono ? "font-mono" : ""}`}>{e.sub}</span>}
                    {selected && <Icon path={ICON_PATHS.arrowRightAlt} className="w-3.5 h-3.5 ml-auto shrink-0 text-[var(--text-muted)]" />}
                  </div>
                );
              })}
            </div>
          ))}
          {flat.length === 0 && (
            <p className="px-2 py-6 text-center text-sm text-[var(--text-muted)]">
              {loading ? t("common.loading") : t("common.noResults")}
            </p>
          )}
        </div>
      </Modal>
    </>
  );
}
