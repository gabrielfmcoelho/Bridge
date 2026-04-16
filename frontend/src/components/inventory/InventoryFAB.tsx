"use client";

import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";

interface InventoryFABProps {
  canEdit: boolean;
  hasItems: boolean;
  activeFilterCount: number;
  onAdd: () => void;
  onFilter: () => void;
  onExport: () => void;
  addLabel: string;
  addColor?: string;
  extraActions?: FABAction[];
}

export default function InventoryFAB({
  canEdit,
  hasItems,
  activeFilterCount,
  onAdd,
  onFilter,
  onExport,
  addLabel,
  addColor,
  extraActions,
}: InventoryFABProps) {
  const { t } = useLocale();
  const actions: FABAction[] = [];

  if (canEdit) {
    actions.push({
      label: addLabel,
      icon: "M12 4v16m8-8H4",
      color: addColor,
      onClick: onAdd,
    });
  }

  actions.push({
    label: `${t("filters.title")}${activeFilterCount > 0 ? ` (${activeFilterCount})` : ""}`,
    icon: ICON_PATHS.filter,
    color: activeFilterCount > 0 ? undefined : "#6b7280",
    onClick: onFilter,
  });

  if (extraActions) {
    actions.push(...extraActions);
  }

  if (hasItems) {
    actions.push({
      label: t("common.export"),
      icon: ICON_PATHS.exportDoc,
      color: "#6b7280",
      onClick: onExport,
    });
  }

  return <FloatingActionButton actions={actions} />;
}
