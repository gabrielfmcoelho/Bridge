"use client";

import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";
import { useLocale } from "@/contexts/LocaleContext";

export default function HostsFAB({
  canEdit, hasHosts, hasScannableHosts, activeFilterCount,
  onAdd, onFilter, onScan, onExport,
}: {
  canEdit: boolean;
  hasHosts: boolean;
  hasScannableHosts: boolean;
  activeFilterCount: number;
  onAdd: () => void;
  onFilter: () => void;
  onScan: () => void;
  onExport: () => void;
}) {
  const { t } = useLocale();
  const actions: FABAction[] = [];

  if (canEdit) {
    actions.push({
      label: t("host.addHost"),
      icon: "M12 4v16m8-8H4",
      onClick: onAdd,
    });
  }
  actions.push({
    label: t("filters.title"),
    icon: "M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z",
    onClick: onFilter,
    color: activeFilterCount > 0 ? undefined : "#6b7280",
  });
  if (canEdit && hasScannableHosts) {
    actions.push({
      label: t("host.scanAll"),
      icon: "M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z",
      onClick: onScan,
      color: "#8b5cf6",
    });
  }
  if (hasHosts) {
    actions.push({
      label: t("common.export"),
      icon: "M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z",
      onClick: onExport,
      color: "#6b7280",
    });
  }

  return <FloatingActionButton actions={actions} />;
}
