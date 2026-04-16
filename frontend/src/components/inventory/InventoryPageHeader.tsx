"use client";

import Button from "@/components/ui/Button";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";

interface InventoryPageHeaderProps {
  title: string;
  viewMode: "cards" | "table";
  onViewModeChange: (mode: "cards" | "table") => void;
  addLabel?: string;
  onAdd?: () => void;
}

export default function InventoryPageHeader({
  title,
  viewMode,
  onViewModeChange,
  addLabel,
  onAdd,
}: InventoryPageHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2 mb-6">
      <h1 className="text-xl sm:text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{title}</h1>
      <div className="flex items-center gap-1.5">
        <div className="hidden sm:flex">
          <ViewToggle
            value={viewMode}
            onChange={(v) => onViewModeChange(v as "cards" | "table")}
            options={[
              { key: "cards", label: "Cards", icon: VIEW_ICONS.cards },
              { key: "table", label: "Table", icon: VIEW_ICONS.table },
            ]}
          />
        </div>
        {addLabel && onAdd && (
          <div className="hidden sm:block">
            <Button size="sm" onClick={onAdd}><span className="mr-1">+</span> {addLabel}</Button>
          </div>
        )}
      </div>
    </div>
  );
}
