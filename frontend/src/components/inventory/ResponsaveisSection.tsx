"use client";

import { useState } from "react";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import SortableTable, { sortRows } from "@/components/ui/SortableTable";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";
import EmptyState from "@/components/ui/EmptyState";
import type { EntityResponsavel } from "@/lib/types";

interface ResponsaveisSectionProps {
  responsaveis: EntityResponsavel[];
  t: (key: string) => string;
  emptyTitle?: string;
  emptyDescription?: string;
}

function WhatsAppButton({ phone, name }: { phone: string; name: string }) {
  return (
    <a
      href={`https://wa.me/${phone.replace(/\D/g, "")}`}
      target="_blank"
      rel="noopener noreferrer"
      title={`WhatsApp ${name}`}
      className="w-7 h-7 rounded-full flex items-center justify-center bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25 transition-colors"
      onClick={(e) => e.stopPropagation()}
    >
      <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor">
        <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413z" />
      </svg>
    </a>
  );
}

function getInitials(name: string) {
  return name.split(" ").map(p => p[0]).slice(0, 2).join("").toUpperCase();
}

export default function ResponsaveisSection({
  responsaveis,
  t,
  emptyTitle,
  emptyDescription,
}: ResponsaveisSectionProps) {
  const [respView, setRespView] = useState<"cards" | "table">("cards");
  const items = responsaveis ?? [];

  return (
    <>
      <SectionHeading actions={
        <ViewToggle
          value={respView}
          onChange={(v) => setRespView(v as "cards" | "table")}
          options={[
            { key: "cards", label: t("common.cards"), icon: VIEW_ICONS.cards },
            { key: "table", label: t("common.table"), icon: VIEW_ICONS.table },
          ]}
        />
      }>
        {t("host.responsaveis")}
      </SectionHeading>

      {items.length === 0 ? (
        <EmptyState
          icon="search"
          title={emptyTitle || t("host.noResponsaveis")}
          description={emptyDescription || t("host.noResponsaveisDesc") || "No responsible people registered."}
          compact
        />
      ) : respView === "cards" ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-2">
          {items.map((r, i) => (
            <Card key={r.id ?? i} hover={false} className="!p-3">
              <div className="flex items-center gap-2.5 mb-2">
                <div className={`w-8 h-8 rounded-full flex items-center justify-center shrink-0 text-xs font-bold ${r.is_main ? "bg-cyan-500/15 text-cyan-400" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"}`}>
                  {getInitials(r.name)}
                </div>
                <div className="min-w-0 flex-1">
                  <span className="text-sm font-medium text-[var(--text-primary)] truncate flex items-center gap-1">
                    {r.is_main && (
                      <svg className="w-3.5 h-3.5 text-amber-400 shrink-0" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                      </svg>
                    )}
                    {r.name}
                  </span>
                  <span className="text-xs text-[var(--text-muted)] truncate block">{r.role || "--"}</span>
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                  {r.is_externo && <Badge color="amber">{t("responsavel.external")}</Badge>}
                  {r.phone && <WhatsAppButton phone={r.phone} name={r.name} />}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3 text-xs">
                <div>
                  <span className="text-[var(--text-faint)] block mb-0.5">{t("host.phone")}</span>
                  <span className="text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{r.phone || "--"}</span>
                </div>
                <div>
                  <span className="text-[var(--text-faint)] block mb-0.5">{t("host.entity")}</span>
                  <span className="text-[var(--text-muted)]">{r.entity || "--"}</span>
                </div>
              </div>
            </Card>
          ))}
        </div>
      ) : (
        <SortableTable
          columns={[
            { key: "name" as const, label: t("common.name") },
            { key: "phone" as const, label: t("host.phone") },
            { key: "role" as const, label: t("host.role") },
            { key: "entity" as const, label: t("host.entity") },
            { key: "type" as const, label: t("common.type") },
            { key: "actions" as const, label: "" },
          ]}
          defaultSort="name"
        >
          {(sk, sd) => {
            const sorted = sortRows(items, sk, sd, {
              name: (a, b) => a.name.localeCompare(b.name),
              phone: (a, b) => a.phone.localeCompare(b.phone),
              role: (a, b) => a.role.localeCompare(b.role),
              entity: (a, b) => a.entity.localeCompare(b.entity),
              type: (a, b) => Number(b.is_main) - Number(a.is_main),
              actions: () => 0,
            });
            return sorted.map((r, i) => (
              <tr key={r.id ?? i} className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}>
                <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">
                  <span className="flex items-center gap-2">
                    <span className={`w-6 h-6 rounded-full flex items-center justify-center shrink-0 text-[9px] font-bold ${r.is_main ? "bg-cyan-500/15 text-cyan-400" : "bg-[var(--bg-elevated)] text-[var(--text-muted)]"}`}>
                      {getInitials(r.name)}
                    </span>
                    {r.is_main && (
                      <svg className="w-3.5 h-3.5 text-amber-400 shrink-0" fill="currentColor" viewBox="0 0 24 24">
                        <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
                      </svg>
                    )}
                    {r.name}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-[var(--text-muted)]" style={{ fontFamily: "var(--font-mono)" }}>{r.phone || "--"}</td>
                <td className="px-4 py-2.5 text-[var(--text-muted)]">{r.role || "--"}</td>
                <td className="px-4 py-2.5 text-[var(--text-muted)]">{r.entity || "--"}</td>
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-1">
                    {r.is_externo && <Badge color="amber">{t("responsavel.external")}</Badge>}
                  </div>
                </td>
                <td className="px-4 py-2.5">
                  {r.phone && <WhatsAppButton phone={r.phone} name={r.name} />}
                </td>
              </tr>
            ));
          }}
        </SortableTable>
      )}
    </>
  );
}
