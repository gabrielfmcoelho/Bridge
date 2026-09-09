"use client";

import Button from "@/components/ui/Button";
import PageHeader from "@/components/ui/PageHeader";
import ViewToggle, { VIEW_ICONS } from "@/components/ui/ViewToggle";

interface InventoryPageHeaderProps {
  title: string;
  viewMode: "cards" | "table";
  onViewModeChange: (mode: "cards" | "table") => void;
  addLabel?: string;
  onAdd?: () => void;
}

// PageHeader + the cards/table toggle. The add button is desktop-only here
// because inventory pages carry a FAB on phones.
export default function InventoryPageHeader({ title, viewMode, onViewModeChange, addLabel, onAdd }: InventoryPageHeaderProps) {
  return (
    <PageHeader
      title={title}
      actions={
        <>
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
              <Button size="sm" onClick={onAdd}>
                <span className="mr-1">+</span> {addLabel}
              </Button>
            </div>
          )}
        </>
      }
    />
  );
}
