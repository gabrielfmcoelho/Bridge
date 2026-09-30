"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { contactsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import { useOpenOnParam } from "@/hooks/useOpenOnParam";
import { useLocalStorage } from "@/hooks/useLocalStorage";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import ListToolbar from "@/components/ui/ListToolbar";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import { tableClasses } from "@/components/ui/Table";
import RowActions from "@/components/ui/RowActions";
import PillButton from "@/components/ui/PillButton";
import SectionHeading from "@/components/ui/SectionHeading";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import TrashButton from "@/components/inventory/TrashDrawer";
import ContactForm from "@/components/contacts/ContactForm";
import ContactCard from "@/components/contacts/ContactCard";
import ContactDetailDrawer, { usageSummary } from "@/components/contacts/ContactDetailDrawer";
import { ICON_PATHS } from "@/lib/icon-paths";
import { formatPhone } from "@/lib/phone";
import type { Contact } from "@/lib/types";

type Col = "name" | "phone" | "email" | "area" | "use" | "actions";
type Filters = { kind: "" | "internal" | "external"; area: string; use: "" | "with" | "without" };
const emptyFilters: Filters = { kind: "", area: "", use: "" };
const totalUses = (c: Contact) => Object.values(c.usage ?? {}).reduce((a, b) => a + (b ?? 0), 0);

/**
 * Contatos: the people assets name as responsáveis. The list says what each
 * one is responsável for; the detail drawer lists those assets.
 */
export default function ContactsPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const confirm = useConfirm();
  const flag = useFlag();
  const qc = useQueryClient();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const isAdmin = user?.role === "admin";

  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [showFilters, setShowFilters] = useState(false);
  const [viewMode, setViewMode] = useLocalStorage<"table" | "cards">("contacts.view", "table");
  const [detail, setDetail] = useState<Contact | null>(null);
  const [form, setForm] = useState<{ contact: Contact | null } | null>(null);
  const [subHeader, setSubHeader] = useState<ReactNode>(null);
  const [footer, setFooter] = useState<ReactNode>(null);
  useOpenOnParam("new", () => setForm({ contact: null }), canEdit);

  const { data: contacts = [], isLoading } = useQuery({ queryKey: ["contacts"], queryFn: contactsAPI.list });
  const areas = useMemo(() => [...new Set(contacts.map((c) => c.entity).filter(Boolean))].sort(), [contacts]);
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  const visible = useMemo(() => {
    const q = search.trim().toLowerCase();
    return contacts.filter((c) => {
      if (filters.kind === "external" && !c.is_external) return false;
      if (filters.kind === "internal" && c.is_external) return false;
      if (filters.area && c.entity !== filters.area) return false;
      if (filters.use === "with" && totalUses(c) === 0) return false;
      if (filters.use === "without" && totalUses(c) > 0) return false;
      if (!q) return true;
      const digits = q.replace(/\D/g, "");
      return [c.name, c.role, c.entity, c.email ?? ""].some((v) => v.toLowerCase().includes(q)) || (!!digits && c.phone.includes(digits));
    });
  }, [contacts, filters, search]);

  const del = useMutation({
    mutationFn: (id: number) => contactsAPI.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["contacts"] });
      qc.invalidateQueries({ queryKey: ["contacts-trash"] });
      setDetail(null);
    },
    onError: (e) => flag({ appearance: "error", title: t("common.delete"), description: e instanceof Error ? e.message : t("form.saveFailed") }),
  });
  // Deleting says what the contact is taken off of.
  const remove = async (c: Contact) => {
    const use = usageSummary(c.usage, t);
    if (await confirm({
      title: t("confirm.deleteTitle", { name: `"${c.name}"` }),
      message: use ? t("contact.deleteImpact", { name: c.name, use }) : t("contact.deleteNoImpact"),
      danger: true, confirmLabel: t("common.delete"),
    })) del.mutate(c.id);
  };
  const actions = (c: Contact) => [
    { label: t("contact.open"), icon: ICON_PATHS.eye, onClick: () => setDetail(c) },
    { label: t("common.edit"), icon: ICON_PATHS.pencil, onClick: () => setForm({ contact: c }), hidden: !canEdit },
    { label: t("common.delete"), icon: ICON_PATHS.trashOutline, danger: true, onClick: () => remove(c), hidden: !isAdmin },
  ];

  const pills = <K extends keyof Filters>(label: string, key: K, values: Filters[K][], labelOf: (v: Filters[K]) => string) => (
    <section className="space-y-2">
      <SectionHeading as="h3" className="!mb-0">{label}</SectionHeading>
      <div className="flex flex-wrap gap-1.5">
        {values.map((v) => (
          <PillButton key={v || "all"} size="sm" active={filters[key] === v} onClick={() => setFilters((f) => ({ ...f, [key]: v }))}>
            {v ? labelOf(v) : t("common.all")}
          </PillButton>
        ))}
      </div>
    </section>
  );

  const nameCell = (c: Contact) => (
    <span className="min-w-0">
      <span className="flex items-center gap-2">
        <span className="font-medium text-[var(--text-primary)] truncate">{c.name}</span>
        {c.is_external && <Badge color="warning">{t("responsavel.external")}</Badge>}
      </span>
      {c.role && <span className="block text-xs text-[var(--text-muted)] truncate">{c.role}</span>}
    </span>
  );

  return (
    <PageShell>
      <PageHeader
        title={t("nav.contacts")}
        description={t("contact.pageDescription")}
        addLabel={canEdit ? t("contact.create") : undefined}
        onAdd={canEdit ? () => setForm({ contact: null }) : undefined}
        controlsKey="contacts"
        controlsBadge={activeFilterCount}
        controls={
          <ListToolbar
            search={search}
            onSearchChange={setSearch}
            onFilterClick={() => setShowFilters(true)}
            activeFilterCount={activeFilterCount}
            searchPlaceholder={t("contact.searchPlaceholder")}
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            actions={
              <TrashButton title={t("trash.title", { module: t("nav.contacts") })} source={{
                key: ["contacts-trash"], fetch: contactsAPI.trash, restore: contactsAPI.restore, invalidate: [["contacts"]],
                label: (c) => c.name, meta: (c) => [c.role, c.entity].filter(Boolean).join(" · ") || undefined,
              }} />
            }
          />
        }
      />

      {isLoading ? (
        <div className="space-y-3">{[0, 1, 2].map((i) => <SkeletonCard key={i} />)}</div>
      ) : visible.length === 0 ? (
        search || activeFilterCount ? (
          <EmptyState icon="search" title={t("common.noResults")} description={t("contact.searchEmptyHint")}
            action={<Button size="sm" variant="secondary" onClick={() => { setSearch(""); setFilters(emptyFilters); }}>{t("vault.clearFilters")}</Button>} />
        ) : (
          <EmptyState icon="search" title={t("contact.emptyTitle")} description={t("contact.emptyHint")}
            action={canEdit ? <Button size="sm" onClick={() => setForm({ contact: null })}>{t("contact.create")}</Button> : undefined} />
        )
      ) : (
        <>
          <div className={viewMode === "table" ? "hidden md:block" : "hidden"}>
            <SortableTable<Col>
              columns={[
                { key: "name", label: t("responsavel.name") },
                { key: "phone", label: t("responsavel.phone") },
                { key: "email", label: t("contact.email") },
                { key: "area", label: t("contact.area") },
                { key: "use", label: t("contact.responsibleFor") },
                { key: "actions", label: "", sortable: false },
              ]}
              defaultSort="name"
            >
              {(key, dir) => sortRows(visible, key, dir, {
                name: (a, b) => a.name.localeCompare(b.name, locale),
                phone: (a, b) => a.phone.localeCompare(b.phone),
                email: (a, b) => (a.email ?? "").localeCompare(b.email ?? ""),
                area: (a, b) => a.entity.localeCompare(b.entity, locale),
                use: (a, b) => totalUses(a) - totalUses(b),
                actions: () => 0,
              }).map((c) => (
                <tr key={c.id} className={`${tableClasses.row} border-t border-[var(--border-subtle)] cursor-pointer`} onClick={() => setDetail(c)}>
                  <td className={tableClasses.td}>{nameCell(c)}</td>
                  <td className={`${tableClasses.td} font-mono text-[var(--text-secondary)] whitespace-nowrap`}>{c.phone ? formatPhone(c.phone) : "–"}</td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)] max-w-[14rem] truncate`}>{c.email || "–"}</td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)]`}>{c.entity || "–"}</td>
                  <td className={`${tableClasses.td} text-[var(--text-secondary)] whitespace-nowrap`}>{usageSummary(c.usage, t) || <span className="text-[var(--text-muted)]">{t("contact.noUseShort")}</span>}</td>
                  <td className={`${tableClasses.td} text-right`} onClick={(e) => e.stopPropagation()}>
                    <RowActions name={c.name} actions={actions(c)} />
                  </td>
                </tr>
              ))}
            </SortableTable>
          </div>
          <div className={`grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 ${viewMode === "table" ? "md:hidden" : ""}`}>
            {[...visible].sort((a, b) => a.name.localeCompare(b.name, locale)).map((c) => (
              <ContactCard key={c.id} contact={c} onOpen={() => setDetail(c)} actions={actions(c)} />
            ))}
          </div>
        </>
      )}

      <Drawer open={showFilters} onClose={() => setShowFilters(false)} title={t("common.filter")}
        footer={activeFilterCount ? <Button variant="secondary" className="w-full" onClick={() => setFilters(emptyFilters)}>{t("vault.clearFilters")}</Button> : undefined}>
        <div className="space-y-6">
          {pills(t("contact.filterKind"), "kind", ["", "internal", "external"], (v) => t(`contact.kind.${v}`))}
          {pills(t("contact.filterUse"), "use", ["", "with", "without"], (v) => t(`contact.useFilter.${v}`))}
          {areas.length > 0 && pills(t("contact.area"), "area", ["", ...areas], (v) => v)}
        </div>
      </Drawer>

      <ContactDetailDrawer contact={detail} onClose={() => setDetail(null)}
        onEdit={canEdit ? (c) => { setDetail(null); setForm({ contact: c }); } : undefined}
        onDelete={isAdmin ? remove : undefined} />

      <Drawer open={!!form} onClose={() => setForm(null)} subHeader={subHeader} footer={footer}
        title={form?.contact ? t("form.editTitle", { name: form.contact.name }) : t("contact.create")}>
        {form && (
          <ContactForm initial={form.contact} onCancel={() => setForm(null)}
            onSubHeaderChange={setSubHeader} onFooterChange={setFooter}
            onSaved={() => { setForm(null); qc.invalidateQueries({ queryKey: ["contacts"] }); }} />
        )}
      </Drawer>
    </PageShell>
  );
}
