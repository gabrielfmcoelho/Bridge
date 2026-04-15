import FloatingActionButton, { type FABAction } from "@/components/ui/FloatingActionButton";

interface DnsFABProps {
  canEdit: boolean;
  hasRecords: boolean;
  activeFilterCount: number;
  onAdd: () => void;
  onFilter: () => void;
  onExport: () => void;
}

export default function DnsFAB({ canEdit, hasRecords, activeFilterCount, onAdd, onFilter, onExport }: DnsFABProps) {
  const actions: FABAction[] = [
    ...(canEdit
      ? [{ label: "Add DNS", icon: "M12 4v16m8-8H4", color: "#06b6d4", onClick: onAdd }]
      : []),
    {
      label: `Filters${activeFilterCount > 0 ? ` (${activeFilterCount})` : ""}`,
      icon: "M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z",
      color: "#8b5cf6",
      onClick: onFilter,
    },
    ...(hasRecords
      ? [{
          label: "Export",
          icon: "M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z",
          color: "#6b7280",
          onClick: onExport,
        }]
      : []),
  ];

  return <FloatingActionButton actions={actions} />;
}
