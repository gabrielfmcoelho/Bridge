"use client";

import Badge from "@/components/ui/Badge";
import type { Service } from "@/lib/types";

interface ServicesTableViewProps {
  services: Service[];
  t: (key: string) => string;
}

export default function ServicesTableView({ services, t }: ServicesTableViewProps) {
  return (
    <div className="bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-x-auto animate-fade-in">
      <table className="w-full text-sm min-w-[600px]">
        <thead>
          <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
            <th className="text-left px-4 py-3 font-semibold">{t("service.nickname")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("common.description")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("service.technologyStack")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("service.developedBy")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("common.tags")}</th>
          </tr>
        </thead>
        <tbody>
          {services.map((svc, i) => (
            <tr
              key={svc.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => window.location.href = `/services/${svc.id}`}
            >
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">{svc.nickname}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[200px] truncate">{svc.description || "-"}</td>
              <td className="px-4 py-2.5">{svc.technology_stack ? <Badge>{svc.technology_stack}</Badge> : <span className="text-[var(--text-faint)]">-</span>}</td>
              <td className="px-4 py-2.5">
                {svc.is_external_dependency ? (
                  <Badge color="amber">{t("service.isExternalDependency")}</Badge>
                ) : (
                  <Badge color={svc.developed_by === "internal" ? "cyan" : "amber"}>
                    {svc.developed_by === "internal" ? t("service.internal") : t("service.external")}
                  </Badge>
                )}
              </td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {svc.tags && svc.tags.length > 0
                    ? svc.tags.slice(0, 3).map((tag) => <Badge key={tag}>{tag}</Badge>)
                    : <span className="text-[var(--text-faint)]">-</span>}
                  {svc.tags && svc.tags.length > 3 && <span className="text-[10px] text-[var(--text-faint)]">+{svc.tags.length - 3}</span>}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
